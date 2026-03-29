package tmux

import (
	"context"
	"testing"
)

func TestFakeClientSessionAndOptions(t *testing.T) {
	t.Parallel()

	client := NewFakeClient()
	ctx := context.Background()

	paneID, err := client.NewSession(ctx, SessionSpec{
		Name:       "dev",
		WindowName: "agents",
		Command:    "fake-agent",
	})
	if err != nil {
		t.Fatalf("NewSession() unexpected error: %v", err)
	}
	if paneID == "" {
		t.Fatal("NewSession() returned empty pane id")
	}

	exists, err := client.HasSession(ctx, "dev")
	if err != nil {
		t.Fatalf("HasSession() unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("HasSession() = false, want true")
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
}

func TestFakeClientSendKeysAndCapturePane(t *testing.T) {
	t.Parallel()

	client := NewFakeClient()
	ctx := context.Background()

	paneID, err := client.NewSession(ctx, SessionSpec{Name: "dev"})
	if err != nil {
		t.Fatalf("NewSession() unexpected error: %v", err)
	}

	if err := client.SendKeys(ctx, paneID, "hello", true); err != nil {
		t.Fatalf("SendKeys() unexpected error: %v", err)
	}

	out, err := client.CapturePane(ctx, paneID, 10)
	if err != nil {
		t.Fatalf("CapturePane() unexpected error: %v", err)
	}
	if want := "hello\n"; out != want {
		t.Fatalf("CapturePane() = %q, want %q", out, want)
	}
}

func TestFakeClientListPanesAndKillSession(t *testing.T) {
	t.Parallel()

	client := NewFakeClient()
	ctx := context.Background()

	rootPane, err := client.NewSession(ctx, SessionSpec{Name: "dev", WindowName: "agents"})
	if err != nil {
		t.Fatalf("NewSession() unexpected error: %v", err)
	}
	if _, err := client.SplitPane(ctx, SplitSpec{TargetPane: rootPane, Direction: "h"}); err != nil {
		t.Fatalf("SplitPane() unexpected error: %v", err)
	}

	panes, err := client.ListPanes(ctx, "dev")
	if err != nil {
		t.Fatalf("ListPanes() unexpected error: %v", err)
	}
	if len(panes) != 2 {
		t.Fatalf("ListPanes() len = %d, want 2", len(panes))
	}

	if err := client.KillSession(ctx, "dev"); err != nil {
		t.Fatalf("KillSession() unexpected error: %v", err)
	}

	exists, err := client.HasSession(ctx, "dev")
	if err != nil {
		t.Fatalf("HasSession() unexpected error: %v", err)
	}
	if exists {
		t.Fatal("HasSession() = true after KillSession(), want false")
	}
}
