package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/adammpkins/llama-terminal-completion/internal/rag"
	"github.com/adammpkins/llama-terminal-completion/internal/tools"
	"github.com/spf13/cobra"
)

var (
	ragIndexName string // --index for rag subcommands
	ragSearchK   int
	ragFlag      string // --rag <index> on ask/agent
)

// ragDir returns the directory holding RAG indexes.
func ragDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "lt", "rag")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".lt", "rag")
	}
	return filepath.Join(".", ".lt", "rag")
}

// ragStorePath returns the store file path for a named index.
func ragStorePath(index string) string {
	if index == "" {
		index = "default"
	}
	return filepath.Join(ragDir(), index, "store.json")
}

var ragCmd = &cobra.Command{
	Use:   "rag",
	Short: "Manage retrieval-augmented generation indexes",
	Long: `Build and query embeddings-backed indexes of local files, directories, and
URLs, then use them as context with "lt ask --rag <index>".`,
}

var ragAddCmd = &cobra.Command{
	Use:   "add <path|url>",
	Short: "Index a file, directory, or URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		sources, err := rag.CollectSources(ctx, args[0])
		if err != nil {
			return err
		}
		if len(sources) == 0 {
			return fmt.Errorf("no indexable content found at %q", args[0])
		}
		store, err := rag.Open(ragStorePath(ragIndexName))
		if err != nil {
			return err
		}
		apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)
		n, err := rag.Index(ctx, store, apiClient, cfg.EmbeddingModel, sources)
		if err != nil {
			return err
		}
		if err := store.Save(); err != nil {
			return err
		}
		printSuccess("✓ Indexed %d chunks from %d source(s) into %q", n, len(sources), indexLabel())
		return nil
	},
}

var ragListCmd = &cobra.Command{
	Use:   "list",
	Short: "List RAG indexes",
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := os.ReadDir(ragDir())
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No indexes yet.")
				return nil
			}
			return err
		}
		found := false
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			store, err := rag.Open(ragStorePath(e.Name()))
			if err != nil {
				continue
			}
			fmt.Printf("%-20s %d chunks\n", e.Name(), len(store.Records))
			found = true
		}
		if !found {
			fmt.Println("No indexes yet.")
		}
		return nil
	},
}

var ragSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search an index and print the top matches",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := ragRetrieve(ragIndexName, args[0], ragSearchK)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Println("No matches.")
			return nil
		}
		for _, r := range results {
			printInfo(fmt.Sprintf("[%.3f] %s\n", r.Score, r.Record.Source))
			fmt.Println(truncateForDisplay(r.Record.Text, 300))
			fmt.Println()
		}
		return nil
	},
}

var ragRemoveCmd = &cobra.Command{
	Use:   "rm <index>",
	Short: "Delete a RAG index",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := filepath.Join(ragDir(), args[0])
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		printSuccess("✓ Removed index %q", args[0])
		return nil
	},
}

func init() {
	ragAddCmd.Flags().StringVar(&ragIndexName, "index", "default", "Index name")
	ragSearchCmd.Flags().StringVar(&ragIndexName, "index", "default", "Index name")
	ragSearchCmd.Flags().IntVarP(&ragSearchK, "top", "k", 5, "Number of results")
	ragCmd.AddCommand(ragAddCmd, ragListCmd, ragSearchCmd, ragRemoveCmd)
	rootCmd.AddCommand(ragCmd)

	// --rag context flag on ask and agent.
	askCmd.Flags().StringVar(&ragFlag, "rag", "", "Augment the prompt with context from a RAG index")
	agentCmd.Flags().StringVar(&ragFlag, "rag", "", "Expose a rag_search tool backed by this index")
}

func indexLabel() string {
	if ragIndexName == "" {
		return "default"
	}
	return ragIndexName
}

// ragRetrieve embeds the query and returns the top-k results from the index.
func ragRetrieve(index, query string, k int) ([]rag.Result, error) {
	store, err := rag.Open(ragStorePath(index))
	if err != nil {
		return nil, err
	}
	if len(store.Records) == 0 {
		return nil, fmt.Errorf("index %q is empty (use 'lt rag add')", indexOrDefault(index))
	}
	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)
	vecs, err := apiClient.Embeddings([]string{query}, cfg.EmbeddingModel)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("no embedding returned for query")
	}
	return store.Search(vecs[0], k), nil
}

// ragContext retrieves and formats top-k context for inlining into a prompt,
// returning the formatted context and the de-duplicated list of source names
// (for citations).
func ragContext(index, query string, k int) (string, []string, error) {
	results, err := ragRetrieve(index, query, k)
	if err != nil {
		return "", nil, err
	}
	if len(results) == 0 {
		return "", nil, nil
	}
	var b strings.Builder
	b.WriteString("--- Retrieved context ---\n")
	seen := map[string]bool{}
	var sources []string
	for _, r := range results {
		fmt.Fprintf(&b, "[%s]\n%s\n\n", r.Record.Source, r.Record.Text)
		if !seen[r.Record.Source] {
			seen[r.Record.Source] = true
			sources = append(sources, r.Record.Source)
		}
	}
	return strings.TrimSpace(b.String()), sources, nil
}

func indexOrDefault(index string) string {
	if index == "" {
		return "default"
	}
	return index
}

func truncateForDisplay(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// ragSearchTool exposes RAG retrieval to the agent as a tool. It implements
// tools.Tool and is registered when "lt agent --rag <index>" is used.
type ragSearchTool struct{ index string }

func (t *ragSearchTool) Name() string { return "rag_search" }
func (t *ragSearchTool) Description() string {
	return "Search the knowledge base for passages relevant to a query."
}
func (t *ragSearchTool) Risk() tools.RiskLevel { return tools.RiskRead }
func (t *ragSearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "What to search for."},
			"k": {"type": "integer", "description": "Number of passages to return (default 5)."}
		},
		"required": ["query"]
	}`)
}

func (t *ragSearchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Query string `json:"query"`
		K     int    `json:"k"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if a.K <= 0 {
		a.K = 5
	}
	out, _, err := ragContext(t.index, a.Query, a.K)
	if err != nil {
		return "", err
	}
	if out == "" {
		return "(no relevant passages found)", nil
	}
	return out, nil
}
