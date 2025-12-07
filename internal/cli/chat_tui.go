package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/adammpkins/llamaterm/internal/client"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Styles for the chat interface
var (
	primaryColor   = lipgloss.Color("#7C3AED")
	secondaryColor = lipgloss.Color("#06B6D4")
	mutedColor     = lipgloss.Color("#6B7280")
	errorColor     = lipgloss.Color("#EF4444")
	successColor   = lipgloss.Color("#10B981")

	appStyle = lipgloss.NewStyle().Padding(1, 2)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 2).
			MarginBottom(1)

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 1).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(secondaryColor).
			Padding(0, 1).
			Bold(true)

	userMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			PaddingLeft(2).
			MarginBottom(1)

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true).
			MarginTop(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	modelSelectorStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)
)

// View states
type viewState int

const (
	viewChat viewState = iota
	viewModelSelector
)

type chatMessage struct {
	Role    string
	Content string
}

// Model item for the list component
type modelItem struct {
	id      string
	ownedBy string
}

func (i modelItem) Title() string       { return i.id }
func (i modelItem) Description() string { return fmt.Sprintf("Owned by: %s", i.ownedBy) }
func (i modelItem) FilterValue() string { return i.id }

type chatModel struct {
	textarea        textarea.Model
	viewport        viewport.Model
	modelList       list.Model
	spinner         spinner.Model
	glamourRenderer *glamour.TermRenderer
	messages        []chatMessage
	apiMessages     []client.ChatMessage
	apiClient       *client.Client
	loading         bool
	loadingModels   bool
	currentResponse strings.Builder
	err             error
	width           int
	height          int
	ready           bool
	model           string
	baseURL         string
	viewState       viewState
	availableModels []client.Model
}

type responseMsg struct {
	content string
	err     error
}

type modelsMsg struct {
	models []client.Model
	err    error
}

func newChatModel() chatModel {
	ta := textarea.New()
	ta.Placeholder = "Ask me anything..."
	ta.Focus()
	ta.CharLimit = 4000
	ta.SetWidth(60)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(primaryColor)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	vp := viewport.New(80, 20)
	vp.SetContent("")

	// Create list delegate with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(primaryColor).
		BorderForeground(primaryColor)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(secondaryColor)

	ml := list.New([]list.Item{}, delegate, 60, 20)
	ml.Title = "🔧 Select Model"
	ml.SetShowStatusBar(true)
	ml.SetFilteringEnabled(true)
	ml.Styles.Title = headerStyle

	return chatModel{
		textarea:        ta,
		viewport:        vp,
		modelList:       ml,
		spinner:         sp,
		glamourRenderer: renderer,
		messages:        []chatMessage{},
		apiMessages: []client.ChatMessage{
			{Role: "system", Content: "You are a helpful AI assistant. Be concise but thorough. Format code with markdown."},
		},
		apiClient: client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model),
		model:     cfg.Model,
		baseURL:   cfg.BaseURL,
		viewState: viewChat,
	}
}

func (m chatModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func (m chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var tiCmd, vpCmd, spCmd, listCmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle model selector view
		if m.viewState == viewModelSelector {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.viewState = viewChat
				m.textarea.Focus()
				return m, nil
			case "enter":
				if !m.loadingModels {
					selected := m.modelList.SelectedItem()
					if selected != nil {
						item := selected.(modelItem)
						m.model = item.id
						m.apiClient = client.NewClient(cfg.BaseURL, cfg.APIKey, m.model)
						m.messages = append(m.messages, chatMessage{
							Role:    "system",
							Content: fmt.Sprintf("✓ Switched to model: %s", m.model),
						})
						m.updateViewport()
					}
					m.viewState = viewChat
					m.textarea.Focus()
					return m, nil
				}
			}
			m.modelList, listCmd = m.modelList.Update(msg)
			return m, listCmd
		}

		// Handle chat view
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyCtrlO:
			// Ctrl+O opens model selector (if not loading)
			if !m.loading {
				m.viewState = viewModelSelector
				m.loadingModels = true
				return m, m.fetchModels()
			}

		case tea.KeyEnter:
			if m.loading {
				return m, nil
			}

			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}

			switch input {
			case "exit", "quit", "q":
				return m, tea.Quit
			case "/clear":
				m.messages = []chatMessage{}
				m.apiMessages = m.apiMessages[:1]
				m.viewport.SetContent("")
				m.textarea.Reset()
				return m, nil
			case "/help":
				helpText := "\nCommands:\n  /clear  - Clear history\n  /save   - Save conversation\n  /model  - Select model\n  /help   - Show help\n  exit    - End chat\n"
				m.messages = append(m.messages, chatMessage{Role: "system", Content: helpText})
				m.updateViewport()
				m.textarea.Reset()
				return m, nil
			case "/save":
				if err := saveHistory(m.apiMessages, m.model); err != nil {
					m.err = err
				} else {
					m.messages = append(m.messages, chatMessage{Role: "system", Content: "✓ Conversation saved"})
					m.updateViewport()
				}
				m.textarea.Reset()
				return m, nil
			case "/model":
				m.viewState = viewModelSelector
				m.loadingModels = true
				m.textarea.Reset()
				return m, m.fetchModels()
			}

			m.messages = append(m.messages, chatMessage{Role: "user", Content: input})
			m.apiMessages = append(m.apiMessages, client.ChatMessage{Role: "user", Content: input})
			m.currentResponse.Reset()
			m.messages = append(m.messages, chatMessage{Role: "assistant", Content: ""})
			m.updateViewport()
			m.textarea.Reset()
			m.loading = true
			m.err = nil

			return m, tea.Batch(m.spinner.Tick, m.sendMessage())
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		vpWidth := msg.Width - 4
		vpHeight := msg.Height - 14
		if vpHeight < 5 {
			vpHeight = 5
		}
		if vpWidth < 20 {
			vpWidth = 20
		}

		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
		m.textarea.SetWidth(vpWidth - 2)
		m.modelList.SetWidth(vpWidth)
		m.modelList.SetHeight(vpHeight)

		if !m.ready {
			m.ready = true
		}
		m.updateViewport()

	case modelsMsg:
		m.loadingModels = false
		if msg.err != nil {
			m.err = msg.err
			m.viewState = viewChat
			m.textarea.Focus()
			return m, nil
		}

		m.availableModels = msg.models
		items := make([]list.Item, len(msg.models))
		for i, model := range msg.models {
			items[i] = modelItem{id: model.ID, ownedBy: model.OwnedBy}
		}
		m.modelList.SetItems(items)

		// Pre-select current model if it exists
		for i, model := range msg.models {
			if model.ID == m.model {
				m.modelList.Select(i)
				break
			}
		}
		return m, nil

	case responseMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			if len(m.apiMessages) > 1 {
				m.apiMessages = m.apiMessages[:len(m.apiMessages)-1]
			}
			if len(m.messages) > 0 {
				m.messages = m.messages[:len(m.messages)-1]
			}
		} else {
			if len(m.messages) > 0 {
				m.messages[len(m.messages)-1].Content = msg.content
			}
			m.apiMessages = append(m.apiMessages, client.ChatMessage{Role: "assistant", Content: msg.content})
		}
		m.updateViewport()
		m.viewport.GotoBottom()
		return m, nil

	case spinner.TickMsg:
		if m.loading || m.loadingModels {
			m.spinner, spCmd = m.spinner.Update(msg)
			m.updateViewport()
			return m, spCmd
		}
	}

	if m.viewState == viewModelSelector {
		m.modelList, listCmd = m.modelList.Update(msg)
		return m, listCmd
	}

	if !m.loading {
		m.textarea, tiCmd = m.textarea.Update(msg)
	}
	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd, spCmd)
}

func (m *chatModel) fetchModels() tea.Cmd {
	return func() tea.Msg {
		resp, err := m.apiClient.ListModels()
		if err != nil {
			return modelsMsg{err: err}
		}

		// Sort models alphabetically by ID
		models := resp.Data
		sort.Slice(models, func(i, j int) bool {
			return models[i].ID < models[j].ID
		})

		return modelsMsg{models: models}
	}
}

func (m *chatModel) sendMessage() tea.Cmd {
	return func() tea.Msg {
		var fullContent strings.Builder

		if cfg.Stream {
			err := m.apiClient.ChatCompletionStream(m.apiMessages, cfg.MaxTokens, cfg.Temperature, func(content string) {
				fullContent.WriteString(content)
			})
			if err != nil {
				return responseMsg{err: err}
			}
			return responseMsg{content: fullContent.String()}
		}

		resp, err := m.apiClient.ChatCompletion(m.apiMessages, cfg.MaxTokens, cfg.Temperature)
		if err != nil {
			return responseMsg{err: err}
		}
		if len(resp.Choices) == 0 {
			return responseMsg{err: fmt.Errorf("no response from API")}
		}
		return responseMsg{content: resp.Choices[0].Message.Content}
	}
}

func (m *chatModel) updateViewport() {
	var content strings.Builder

	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			content.WriteString(userStyle.Render(" You ") + "\n")
			content.WriteString(userMsgStyle.Render(msg.Content) + "\n\n")
		case "assistant":
			content.WriteString(assistantStyle.Render(" Assistant ") + "\n")
			if m.glamourRenderer != nil && msg.Content != "" {
				rendered, err := m.glamourRenderer.Render(msg.Content)
				if err == nil {
					content.WriteString(rendered + "\n")
				} else {
					content.WriteString(msg.Content + "\n\n")
				}
			} else if msg.Content != "" {
				content.WriteString(msg.Content + "\n\n")
			}
		case "system":
			content.WriteString(helpStyle.Render(msg.Content) + "\n\n")
		}
	}

	if m.loading {
		content.WriteString("\n" + m.spinner.View() + " Thinking...\n")
	}

	m.viewport.SetContent(content.String())
}

func (m chatModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	// Model selector view
	if m.viewState == viewModelSelector {
		var b strings.Builder
		header := headerStyle.Render("🦙 LlamaTerm - Model Selector")
		b.WriteString(header + "\n\n")

		if m.loadingModels {
			b.WriteString(m.spinner.View() + " Loading models...\n")
		} else {
			b.WriteString(m.modelList.View())
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Enter to select • / to filter • Esc to cancel"))

		return appStyle.Render(b.String())
	}

	// Chat view
	var b strings.Builder

	header := headerStyle.Render("🦙 LlamaTerm Chat")
	modelBadge := modelSelectorStyle.Render(fmt.Sprintf("[%s]", m.model))
	modelInfo := statusStyle.Render(fmt.Sprintf("Model: %s • API: %s", modelBadge, m.baseURL))
	b.WriteString(header + "\n")
	b.WriteString(modelInfo + "\n\n")

	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errorMsgStyle.Render("Error: "+m.err.Error()) + "\n")
	}

	if m.loading {
		b.WriteString(m.spinner.View() + " Thinking...\n")
	} else {
		b.WriteString(inputStyle.Render(m.textarea.View()))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("Enter to send • Ctrl+O or /model to switch model • /help • Ctrl+C to quit"))

	return appStyle.Render(b.String())
}

func runChatTUI() error {
	p := tea.NewProgram(
		newChatModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
