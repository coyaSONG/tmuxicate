package adapter

import (
	"context"
	"strings"
	"testing"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"github.com/coyaSONG/tmuxicate/internal/tmux"
)

func TestNewAdapter_Generic(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	a, err := NewAdapter("generic", client, "%1")
	if err != nil {
		t.Fatalf("NewAdapter(generic) unexpected error: %v", err)
	}
	if _, ok := a.(*GenericAdapter); !ok {
		t.Fatalf("NewAdapter(generic) returned %T, want *GenericAdapter", a)
	}
}

func TestNewAdapter_ClaudeCode(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	a, err := NewAdapter("claude-code", client, "%1")
	if err != nil {
		t.Fatalf("NewAdapter(claude-code) unexpected error: %v", err)
	}
	cc, ok := a.(*ClaudeCodeAdapter)
	if !ok {
		t.Fatalf("NewAdapter(claude-code) returned %T, want *ClaudeCodeAdapter", a)
	}
	if cc.cfg.QuietPeriod != 1200_000_000 {
		t.Fatalf("QuietPeriod = %v, want 1.2s", cc.cfg.QuietPeriod)
	}
}

func TestNewAdapter_Codex(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	a, err := NewAdapter("codex", client, "%1")
	if err != nil {
		t.Fatalf("NewAdapter(codex) unexpected error: %v", err)
	}
	cx, ok := a.(*CodexAdapter)
	if !ok {
		t.Fatalf("NewAdapter(codex) returned %T, want *CodexAdapter", a)
	}
	if cx.cfg.QuietPeriod != 1500_000_000 {
		t.Fatalf("QuietPeriod = %v, want 1.5s", cx.cfg.QuietPeriod)
	}
}

func TestNewAdapter_Unknown(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	_, err := NewAdapter("unknown", client, "%1")
	if err == nil {
		t.Fatal("NewAdapter(unknown) expected error, got nil")
	}
}

func TestClaudeCodeAdapter_Notify(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	client.PaneCaptures["%1"] = "❯\n"

	a, err := NewClaudeCodeAdapter(client, "%1")
	if err != nil {
		t.Fatalf("NewClaudeCodeAdapter() unexpected error: %v", err)
	}
	// Pre-seed snapshot and backdate lastChanged so quiet period is satisfied.
	a.lastSnapshot = client.PaneCaptures["%1"]
	a.lastChanged = a.lastChanged.Add(-2 * a.cfg.QuietPeriod)

	ref := MessageRef{ID: protocol.NewMessageID(1), From: "coordinator"}
	if err := a.Notify(context.Background(), ref); err != nil {
		t.Fatalf("Notify() unexpected error: %v", err)
	}

	if len(client.SendKeysCalls) != 1 {
		t.Fatalf("SendKeysCalls len = %d, want 1", len(client.SendKeysCalls))
	}
	sent := client.SendKeysCalls[0].Text
	if !strings.Contains(sent, "using the shell tool") {
		t.Fatalf("Notify message = %q, want it to contain %q", sent, "using the shell tool")
	}
	if !strings.Contains(sent, "reply through tmuxicate") {
		t.Fatalf("Notify message = %q, want it to contain %q", sent, "reply through tmuxicate")
	}
}

func TestCodexAdapter_Notify(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	client.PaneCaptures["%1"] = "›\n"

	a, err := NewCodexAdapter(client, "%1")
	if err != nil {
		t.Fatalf("NewCodexAdapter() unexpected error: %v", err)
	}
	// Pre-seed snapshot and backdate lastChanged so quiet period is satisfied.
	a.lastSnapshot = client.PaneCaptures["%1"]
	a.lastChanged = a.lastChanged.Add(-2 * a.cfg.QuietPeriod)

	ref := MessageRef{ID: protocol.NewMessageID(1), From: "reviewer"}
	if err := a.Notify(context.Background(), ref); err != nil {
		t.Fatalf("Notify() unexpected error: %v", err)
	}

	if len(client.SendKeysCalls) != 1 {
		t.Fatalf("SendKeysCalls len = %d, want 1", len(client.SendKeysCalls))
	}
	sent := client.SendKeysCalls[0].Text
	if !strings.Contains(sent, "use the shell tool") {
		t.Fatalf("Notify message = %q, want it to contain %q", sent, "use the shell tool")
	}
	if !strings.Contains(sent, "respond via tmuxicate") {
		t.Fatalf("Notify message = %q, want it to contain %q", sent, "respond via tmuxicate")
	}
}

func TestClaudeCodeAdapter_Probe_Ready(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	client.PaneCaptures["%1"] = "some output\n❯\n"

	a, err := NewClaudeCodeAdapter(client, "%1")
	if err != nil {
		t.Fatalf("NewClaudeCodeAdapter() unexpected error: %v", err)
	}
	a.lastSnapshot = client.PaneCaptures["%1"]
	a.lastChanged = a.lastChanged.Add(-2 * a.cfg.QuietPeriod)

	state, err := a.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe() unexpected error: %v", err)
	}
	if state != ReadyStateReady {
		t.Fatalf("Probe() = %q, want %q", state, ReadyStateReady)
	}
}

func TestCodexAdapter_Probe_Ready(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	client.PaneCaptures["%1"] = "some output\n› "

	a, err := NewCodexAdapter(client, "%1")
	if err != nil {
		t.Fatalf("NewCodexAdapter() unexpected error: %v", err)
	}
	a.lastSnapshot = client.PaneCaptures["%1"]
	a.lastChanged = a.lastChanged.Add(-2 * a.cfg.QuietPeriod)

	state, err := a.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe() unexpected error: %v", err)
	}
	if state != ReadyStateReady {
		t.Fatalf("Probe() = %q, want %q", state, ReadyStateReady)
	}
}
