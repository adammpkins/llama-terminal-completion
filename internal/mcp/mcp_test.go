package mcp

import (
	"context"
	"os"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestLoadConfigInjected(t *testing.T) {
	orig := readFileFunc
	readFileFunc = func(string) ([]byte, error) {
		return []byte("servers:\n  fs:\n    command: echo\n    args: [hi]\n  remote:\n    transport: http\n    url: https://x/mcp\n"), nil
	}
	defer func() { readFileFunc = orig }()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(cfg.Servers))
	}
	if cfg.Servers["fs"].Command != "echo" || cfg.Servers["remote"].URL != "https://x/mcp" {
		t.Errorf("unexpected config: %+v", cfg.Servers)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	orig := readFileFunc
	readFileFunc = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	defer func() { readFileFunc = orig }()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("expected empty config, got %+v", cfg.Servers)
	}
}

func TestBuildTransportErrors(t *testing.T) {
	if _, err := buildTransport(ServerConfig{Transport: "stdio"}); err == nil {
		t.Error("stdio without command should error")
	}
	if _, err := buildTransport(ServerConfig{Transport: "http"}); err == nil {
		t.Error("http without url should error")
	}
	if _, err := buildTransport(ServerConfig{Transport: "bogus", Command: "x"}); err == nil {
		t.Error("unknown transport should error")
	}
}

type echoIn struct {
	Msg string `json:"msg"`
}

// TestInMemoryRoundTrip wires a real in-memory MCP server to our Session wrapper
// and verifies tool discovery and invocation through the tools.Tool adapter.
func TestInMemoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	serverT, clientT := sdk.NewInMemoryTransports()

	server := sdk.NewServer(&sdk.Implementation{Name: "test-server", Version: "1"}, nil)
	sdk.AddTool(server, &sdk.Tool{Name: "echo", Description: "echoes its input"},
		func(_ context.Context, _ *sdk.CallToolRequest, in echoIn) (*sdk.CallToolResult, any, error) {
			return &sdk.CallToolResult{
				Content: []sdk.Content{&sdk.TextContent{Text: "echo:" + in.Msg}},
			}, nil, nil
		})
	if _, err := server.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	sess := &Session{name: "test", session: cs}
	defer func() { _ = sess.Close() }()

	ts, err := sess.Tools(ctx)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(ts) != 1 || ts[0].Name() != "echo" {
		t.Fatalf("expected echo tool, got %+v", ts)
	}
	if ts[0].Risk() != 1 { // RiskWrite
		t.Errorf("expected MCP tools to be RiskWrite")
	}

	out, err := ts[0].Execute(ctx, []byte(`{"msg":"hi"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "echo:hi") {
		t.Errorf("unexpected tool output: %q", out)
	}
}
