package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"github.com/coyaSONG/tmuxicate/internal/tmux"
)

func TestDaemonNotifiesUnreadReceipt(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	agentName := "reviewer"
	if err := os.MkdirAll(filepath.Join(stateDir, "runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(stateDir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(stateDir, "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mailbox.MessagesDir(stateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mailbox.StagingDir(stateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mailbox.OrphanedMessagesDir(stateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mailbox.LocksDir(stateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mailbox.ReceiptLocksDir(stateDir, agentName), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, folder := range []protocol.FolderState{protocol.FolderStateUnread, protocol.FolderStateActive, protocol.FolderStateDone, protocol.FolderStateDead} {
		if err := os.MkdirAll(mailbox.InboxDir(stateDir, agentName, folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(mailbox.AgentDir(stateDir, agentName), "events"), 0o755); err != nil {
		t.Fatal(err)
	}

	readyPayload := map[string]any{
		"session": "dev",
		"agents":  map[string]string{agentName: "%1"},
	}
	readyData, err := json.Marshal(readyPayload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "runtime", "ready.json"), readyData, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.ResolvedConfig{
		Config: config.Config{
			Version: 1,
			Session: config.SessionConfig{
				Name:      "dev",
				Workspace: stateDir,
				StateDir:  stateDir,
				Layout:    "triad",
			},
			Delivery: config.DeliveryConfig{
				Mode:          "notify_then_read",
				RetryInterval: config.Duration(100 * time.Millisecond),
			},
			Transcript: config.TranscriptConfig{
				Mode: "pipe-pane",
				Dir:  filepath.Join(stateDir, "transcripts"),
			},
			Routing: config.RoutingConfig{
				Coordinator: "reviewer",
			},
			Defaults: config.DefaultsConfig{
				Workdir: stateDir,
			},
			Agents: []config.AgentConfig{
				{
					Name:    agentName,
					Alias:   "review",
					Adapter: "generic",
					Command: "fake-agent",
					Role:    "reviewer",
					Pane:    config.PaneConfig{Slot: "main"},
					Workdir: stateDir,
				},
			},
		},
	}

	fakeTmux := tmux.NewFakeClient()
	fakeTmux.PaneCaptures["%1"] = "READY\n"

	d := NewDaemon(stateDir, fakeTmux, cfg)
	d.healthInterval = 50 * time.Millisecond
	d.heartbeatInterval = 50 * time.Millisecond
	d.sweepInterval = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- d.Run(ctx)
	}()
	defer func() {
		cancel()
		<-done
	}()

	time.Sleep(50 * time.Millisecond)

	store := mailbox.NewStore(stateDir)
	body := []byte("review this\n")
	sum := sha256.Sum256(body)
	env := protocol.Envelope{
		Schema:      protocol.MessageSchemaV1,
		ID:          protocol.NewMessageID(1),
		Seq:         1,
		Session:     "dev",
		Thread:      protocol.NewThreadID(1),
		Kind:        protocol.KindReviewRequest,
		From:        protocol.AgentName("coordinator"),
		To:          []protocol.AgentName{protocol.AgentName(agentName)},
		CreatedAt:   time.Now().UTC(),
		BodyFormat:  protocol.BodyFormatMD,
		BodySHA256:  fmt.Sprintf("%x", sum[:]),
		BodyBytes:   int64(len(body)),
		Priority:    protocol.PriorityHigh,
		RequiresAck: true,
		Subject:     "Review request",
	}
	if err := store.CreateMessage(env, body); err != nil {
		t.Fatal(err)
	}

	receipt := protocol.Receipt{
		Schema:         protocol.ReceiptSchemaV1,
		MessageID:      env.ID,
		Seq:            env.Seq,
		Recipient:      protocol.AgentName(agentName),
		FolderState:    protocol.FolderStateUnread,
		Revision:       0,
		NotifyAttempts: 0,
	}
	if err := store.CreateReceipt(receipt); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := store.ReadReceipt(agentName, env.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(fakeTmux.SendKeysCalls) > 0 && got.NotifyAttempts > 0 && got.LastNotifiedAt != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected notification, got none")
}
