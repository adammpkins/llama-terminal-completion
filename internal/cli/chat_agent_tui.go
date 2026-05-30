package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/agent"
	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/adammpkins/llama-terminal-completion/internal/shellrun"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// This is the Bubble Tea frontend for the agentic chat. It is one of two
// frontends over the same evented agent core (the other is the plain REPL in
// chat_agent.go). The agent runs in a worker goroutine and communicates with
// the UI exclusively through channels:
//
//	worker → UI : agentEventMsg / agentConfirmMsg / agentTurnDoneMsg (events chan)
//	UI → worker : a bool on the decision chan (answers a confirmation)
//
// Crucially, a confirmation blocks the *worker*, not the UI: the agent's
// Confirm callback sends an agentConfirmMsg and then blocks on <-decision while
// the Bubble Tea event loop stays responsive.

// Messages flowing from the worker goroutine into the Bubble Tea program.
type agentEventMsg struct{ ev agent.Event }
type agentConfirmMsg struct{ action, detail string }
type agentTurnDoneMsg struct {
	messages []client.ChatMessage
	err      error
}

// toolLineStyle styles the "→ tool" activity lines.
var toolLineStyle = lipgloss.NewStyle().Foreground(secondaryColor)

type agentChatModel struct {
	ag        *agent.Agent
	model     string
	messages  []client.ChatMessage
	toolCount int

	events   chan tea.Msg
	decision chan bool

	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model
	width    int
	height   int

	blocks []string // finalized transcript blocks
	// stream is the in-progress assistant text. It is a plain string (not a
	// strings.Builder) because Bubble Tea copies the model by value each Update,
	// and a strings.Builder panics when copied after use.
	stream string
	// turnProduced tracks whether the current turn emitted any visible output,
	// so an empty turn (common with reasoning models that exhaust the token
	// budget) can be surfaced instead of leaving a blank screen.
	turnProduced  bool
	running       bool
	awaitConfirm  bool
	confirmAction string
	confirmDetail string
	err           error
	ready         bool
}

// waitForAgentMsg returns a command that blocks until the next worker message.
// Exactly one of these is kept outstanding at any time (each handler re-issues
// it), so there is a single reader of the channel.
func waitForAgentMsg(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (m agentChatModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick, waitForAgentMsg(m.events))
}

func (m agentChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Confirmation modal intercepts input without touching the worker's
		// liveness — we just answer the decision channel.
		if m.awaitConfirm {
			switch msg.String() {
			case "y", "Y":
				m.decision <- true
				m.awaitConfirm = false
			case "ctrl+c":
				m.decision <- false
				return m, tea.Quit
			default:
				m.decision <- false
				m.awaitConfirm = false
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.running {
				return m, nil
			}
			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}
			if input == "exit" || input == "quit" {
				return m, tea.Quit
			}
			if expanded, err := expandReferences(input); err == nil {
				input = expanded
			}
			m.appendBlock(userStyle.Render(" You ") + "\n" + userMsgStyle.Render(input))
			m.messages = append(m.messages, client.ChatMessage{Role: "user", Content: input})
			m.textarea.Reset()
			m.running = true
			m.startTurn()
			m.refresh()
			cmds = append(cmds, m.spinner.Tick)
			return m, tea.Batch(cmds...)
		}

	case agentEventMsg:
		m.handleEvent(msg.ev)
		m.refresh()
		return m, waitForAgentMsg(m.events)

	case agentConfirmMsg:
		m.flushStream()
		m.awaitConfirm = true
		m.confirmAction = msg.action
		m.confirmDetail = msg.detail
		m.refresh()
		return m, waitForAgentMsg(m.events)

	case agentTurnDoneMsg:
		m.flushStream()
		m.running = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.messages = msg.messages
		}
		m.refresh()
		return m, waitForAgentMsg(m.events)

	case spinner.TickMsg:
		if m.running {
			var c tea.Cmd
			m.spinner, c = m.spinner.Update(msg)
			cmds = append(cmds, c)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpHeight := msg.Height - 10
		if vpHeight < 3 {
			vpHeight = 3
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width-4, vpHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 4
			m.viewport.Height = vpHeight
		}
		m.textarea.SetWidth(msg.Width - 4)
		m.refresh()
	}

	var tiCmd, vpCmd tea.Cmd
	if !m.awaitConfirm {
		m.textarea, tiCmd = m.textarea.Update(msg)
	}
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, tiCmd, vpCmd)
	return m, tea.Batch(cmds...)
}

// startTurn launches the agent for the current message history in a goroutine.
// It does not issue a new waitForAgentMsg — exactly one is already outstanding.
func (m *agentChatModel) startTurn() {
	history := make([]client.ChatMessage, len(m.messages))
	copy(history, m.messages)
	go func() {
		updated, _, err := m.ag.RunTurn(context.Background(), history)
		m.events <- agentTurnDoneMsg{messages: updated, err: err}
	}()
}

func (m *agentChatModel) handleEvent(e agent.Event) {
	switch e.Kind {
	case agent.EventStarted:
		m.turnProduced = false
	case agent.EventToken:
		if strings.TrimSpace(e.Token) != "" {
			m.turnProduced = true
		}
		m.stream += e.Token
	case agent.EventToolRequested:
		m.turnProduced = true
		m.flushStream()
		m.toolCount++
		m.appendBlock(toolLineStyle.Render(fmt.Sprintf("→ %s %s", e.Tool, e.Args)))
	case agent.EventToolResult:
		if e.Declined {
			m.appendBlock(helpStyle.Render("  (declined)"))
		}
	case agent.EventError:
		m.flushStream()
		m.err = e.Err
	case agent.EventDone:
		m.flushStream()
		if !m.turnProduced {
			m.appendBlock(helpStyle.Render("(no response — the model returned nothing. Try a higher --max-tokens or a non-reasoning model.)"))
		}
	}
}

// flushStream finalizes any in-progress assistant text into a rendered block.
func (m *agentChatModel) flushStream() {
	text := strings.TrimSpace(m.stream)
	m.stream = ""
	if text == "" {
		return
	}
	m.appendBlock(assistantStyle.Render(" Assistant ") + "\n" + strings.TrimRight(renderMarkdown(text), "\n"))
}

func (m *agentChatModel) appendBlock(b string) {
	m.blocks = append(m.blocks, b)
}

// refresh rebuilds the viewport content from finalized blocks plus any
// streaming text, and scrolls to the bottom.
func (m *agentChatModel) refresh() {
	if !m.ready {
		return
	}
	var b strings.Builder
	b.WriteString(strings.Join(m.blocks, "\n\n"))
	if live := m.stream; strings.TrimSpace(live) != "" {
		if len(m.blocks) > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(assistantStyle.Render(" Assistant ") + "\n" + live)
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
}

func (m agentChatModel) View() string {
	if !m.ready {
		return "Starting agentic chat..."
	}
	var b strings.Builder
	header := headerStyle.Render("🦙 LlamaTerm Agentic Chat")
	status := statusStyle.Render(fmt.Sprintf("Model: %s • Tools: %d", m.model, m.toolCount))
	b.WriteString(header + "  " + status + "\n\n")
	b.WriteString(m.viewport.View() + "\n")

	if m.err != nil {
		b.WriteString(errorMsgStyle.Render("Error: "+m.err.Error()) + "\n")
	}

	switch {
	case m.awaitConfirm:
		warn := ""
		if m.confirmAction == "run_command" && shellrun.IsDangerous(m.confirmDetail) {
			warn = "  " + errorMsgStyle.Render("⚠ dangerous")
		}
		b.WriteString(inputStyle.Render(fmt.Sprintf("Run %s?  %s%s", modelSelectorStyle.Render(m.confirmAction), m.confirmDetail, warn)) + "\n")
		b.WriteString(helpStyle.Render("y = approve • any other key = decline"))
	case m.running:
		b.WriteString(m.spinner.View() + " Working...\n")
		b.WriteString(helpStyle.Render("Ctrl+C to quit"))
	default:
		b.WriteString(inputStyle.Render(m.textarea.View()) + "\n")
		b.WriteString(helpStyle.Render("Enter to send • 'exit' or Ctrl+C to quit"))
	}
	return appStyle.Render(b.String())
}

// runAgenticChatTUI is the Bubble Tea frontend entry point for `lt chat --agent`
// in a TTY.
func runAgenticChatTUI() error {
	ws, registry, cleanup, err := buildAgentRuntime()
	if err != nil {
		return err
	}
	defer cleanup()
	_ = ws

	events := make(chan tea.Msg, 256)
	decision := make(chan bool, 1)

	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)
	sys := agent.DefaultSystemPrompt
	if s := activeRoleSystem(); s != "" {
		sys = s
	}

	confirm := agent.ConfirmFunc(nil)
	if !agentYes && cfg.ConfirmCommands {
		confirm = func(action, detail string) (bool, error) {
			events <- agentConfirmMsg{action: action, detail: detail}
			return <-decision, nil
		}
	}

	ag := &agent.Agent{
		Client:        apiClient,
		Registry:      registry,
		MaxIterations: agentMaxIter,
		MaxTokens:     cfg.MaxTokens,
		Temperature:   cfg.Temperature,
		Emit:          func(e agent.Event) { events <- agentEventMsg{ev: e} },
		Confirm:       confirm,
		Audit:         auditLogger(),
		Stream:        cfg.Stream,
	}

	ta := textarea.New()
	ta.Placeholder = "Ask the agent to do something..."
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.SetHeight(2)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(primaryColor)

	m := agentChatModel{
		ag:       ag,
		model:    cfg.Model,
		messages: []client.ChatMessage{{Role: "system", Content: sys}},
		events:   events,
		decision: decision,
		textarea: ta,
		spinner:  sp,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
