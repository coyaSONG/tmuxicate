//go:build integration

package tmux

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRealClientSessionLifecycle(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	client := NewRealClient("")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sessionName := "tmuxicate-test-lifecycle"
	_ = client.KillSession(context.Background(), sessionName)

	exists, err := client.HasSession(ctx, sessionName)
	if err != nil {
		t.Fatalf("HasSession() unexpected error: %v", err)
	}
	if exists {
		t.Fatalf("HasSession() = true before create, want false")
	}

	paneID, err := client.NewSession(ctx, SessionSpec{
		Name:       sessionName,
		WindowName: "agents",
		Command:    "sh -lc 'cat'",
	})
	if err != nil {
		t.Fatalf("NewSession() unexpected error: %v", err)
	}
	t.Cleanup(func() {
		_ = client.KillSession(context.Background(), sessionName)
	})

	if paneID == "" {
		t.Fatal("NewSession() returned empty pane id")
	}

	if err := client.SetPaneOption(ctx, paneID, "@tmuxicate-agent", "coordinator"); err != nil {
		t.Fatalf("SetPaneOption() unexpected error: %v", err)
	}

	got, err := client.ShowPaneOption(ctx, paneID, "@tmuxicate-agent")
	if err != nil {
		t.Fatalf("ShowPaneOption() unexpected error: %v", err)
	}
	if got != "coordinator" {
		t.Fatalf("ShowPaneOption() = %q, want %q", got, "coordinator")
	}

	if err := client.SendKeys(ctx, paneID, "hello from tmux", true); err != nil {
		t.Fatalf("SendKeys() unexpected error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	captured, err := client.CapturePane(ctx, paneID, 20)
	if err != nil {
		t.Fatalf("CapturePane() unexpected error: %v", err)
	}
	if !strings.Contains(captured, "hello from tmux") {
		t.Fatalf("CapturePane() missing expected text, got %q", captured)
	}

	panes, err := client.ListPanes(ctx, sessionName)
	if err != nil {
		t.Fatalf("ListPanes() unexpected error: %v", err)
	}
	if len(panes) == 0 {
		t.Fatal("ListPanes() returned no panes")
	}
}
