package adapter

import (
	"context"
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"github.com/coyaSONG/tmuxicate/internal/tmux"
)

func TestProbe_Ready_WhenQuiet(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	ctx := context.Background()
	paneID, err := client.NewSession(ctx, tmux.SessionSpec{Name: "dev"})
	if err != nil {
		t.Fatalf("NewSession() unexpected error: %v", err)
	}
	client.PaneCaptures[paneID] = "READY>"

	adapter, err := NewGenericAdapter(client, paneID, GenericConfig{
		ReadyRegex:  "READY>",
		QuietPeriod: 0,
	})
	if err != nil {
		t.Fatalf("NewGenericAdapter() unexpected error: %v", err)
	}

	state, err := adapter.Probe(ctx)
	if err != nil {
		t.Fatalf("Probe() unexpected error: %v", err)
	}
	if state != ReadyStateReady {
		t.Fatalf("Probe() = %q, want %q", state, ReadyStateReady)
	}
}

func TestProbe_Busy_WhenBusyRegex(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	ctx := context.Background()
	paneID, err := client.NewSession(ctx, tmux.SessionSpec{Name: "dev"})
	if err != nil {
		t.Fatalf("NewSession() unexpected error: %v", err)
	}
	client.PaneCaptures[paneID] = "BUSY"

	adapter, err := NewGenericAdapter(client, paneID, GenericConfig{
		BusyRegex: "BUSY",
	})
	if err != nil {
		t.Fatalf("NewGenericAdapter() unexpected error: %v", err)
	}

	state, err := adapter.Probe(ctx)
	if err != nil {
		t.Fatalf("Probe() unexpected error: %v", err)
	}
	if state != ReadyStateBusy {
		t.Fatalf("Probe() = %q, want %q", state, ReadyStateBusy)
	}
}

func TestNotify_OnlyWhenReady(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	ctx := context.Background()
	paneID, err := client.NewSession(ctx, tmux.SessionSpec{Name: "dev"})
	if err != nil {
		t.Fatalf("NewSession() unexpected error: %v", err)
	}

	adapter, err := NewGenericAdapter(client, paneID, GenericConfig{
		ReadyRegex:  "READY>",
		QuietPeriod: 0,
	})
	if err != nil {
		t.Fatalf("NewGenericAdapter() unexpected error: %v", err)
	}

	client.PaneCaptures[paneID] = "BUSY"
	if err := adapter.Notify(ctx, MessageRef{ID: protocol.NewMessageID(1), From: "coordinator"}); err == nil {
		t.Fatal("Notify() expected error when pane is not ready, got nil")
	}
	if len(client.SendKeysCalls) != 0 {
		t.Fatalf("SendKeysCalls len = %d, want 0", len(client.SendKeysCalls))
	}

	client.PaneCaptures[paneID] = "READY>"
	if err := adapter.Notify(ctx, MessageRef{ID: protocol.NewMessageID(2), From: "coordinator"}); err != nil {
		t.Fatalf("Notify() unexpected error: %v", err)
	}
	if len(client.SendKeysCalls) != 1 {
		t.Fatalf("SendKeysCalls len = %d, want 1", len(client.SendKeysCalls))
	}
}

func TestBootstrap_PasteMode(t *testing.T) {
	t.Parallel()

	client := tmux.NewFakeClient()
	ctx := context.Background()
	paneID, err := client.NewSession(ctx, tmux.SessionSpec{Name: "dev"})
	if err != nil {
		t.Fatalf("NewSession() unexpected error: %v", err)
	}

	adapter, err := NewGenericAdapter(client, paneID, GenericConfig{
		BootstrapMode: BootstrapModePaste,
		BootstrapText: "tmuxicate bootstrap",
		QuietPeriod:   10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewGenericAdapter() unexpected error: %v", err)
	}

	if err := adapter.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap() unexpected error: %v", err)
	}

	if len(client.SetBufferCalls) != 1 {
		t.Fatalf("SetBufferCalls len = %d, want 1", len(client.SetBufferCalls))
	}
	if len(client.PasteBufferCalls) != 1 {
		t.Fatalf("PasteBufferCalls len = %d, want 1", len(client.PasteBufferCalls))
	}
	if got := client.PaneCaptures[paneID]; got != "tmuxicate bootstrap\n" {
		t.Fatalf("Pane capture = %q, want %q", got, "tmuxicate bootstrap\n")
	}
}
