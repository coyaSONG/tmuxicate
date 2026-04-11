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
					Role: config.RoleSpec{
						Kind:        "review",
						Description: "Handles review notifications",
					},
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
	if err := store.CreateMessage(&env, body); err != nil {
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
	if err := store.CreateReceipt(&receipt); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := store.ReadReceipt(agentName, env.ID)
		if err != nil {
			t.Fatal(err)
		}
		fakeTmux.Mu.Lock()
		notified := len(fakeTmux.SendKeysCalls) > 0
		fakeTmux.Mu.Unlock()
		if notified && got.NotifyAttempts > 0 && got.LastNotifiedAt != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected notification, got none")
}

func TestDaemonSkipsAdaptersAndUnreadWatchersForNonPaneBackedTargets(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	for _, dir := range []string{
		filepath.Join(stateDir, "runtime"),
		filepath.Join(stateDir, "logs"),
		filepath.Join(stateDir, "state"),
		mailbox.MessagesDir(stateDir),
		mailbox.StagingDir(stateDir),
		mailbox.OrphanedMessagesDir(stateDir),
		mailbox.LocksDir(stateDir),
		mailbox.ReceiptLocksDir(stateDir, "local-reviewer"),
		mailbox.ReceiptLocksDir(stateDir, "sandboxed"),
		filepath.Join(mailbox.AgentDir(stateDir, "local-reviewer"), "events"),
		filepath.Join(mailbox.AgentDir(stateDir, "sandboxed"), "events"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, agentName := range []string{"local-reviewer", "sandboxed"} {
		for _, folder := range []protocol.FolderState{protocol.FolderStateUnread, protocol.FolderStateActive, protocol.FolderStateDone, protocol.FolderStateDead} {
			if err := os.MkdirAll(mailbox.InboxDir(stateDir, agentName, folder), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}

	readyPayload := map[string]any{
		"session": "mixed",
		"agents":  map[string]string{"local-reviewer": "%1"},
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
				Name:      "mixed",
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
				Coordinator: "local-reviewer",
			},
			Defaults: config.DefaultsConfig{
				Workdir: stateDir,
			},
			ExecutionTargets: []config.ExecutionTargetConfig{
				{
					Name:         "sandbox",
					Kind:         "sandbox",
					Capabilities: []string{"sandbox"},
					PaneBacked:   false,
				},
			},
			Agents: []config.AgentConfig{
				{
					Name:    "local-reviewer",
					Alias:   "review",
					Adapter: "generic",
					Command: "fake-agent",
					Role: config.RoleSpec{
						Kind:        "review",
						Domains:     []string{"session"},
						Description: "Handles local notifications",
					},
					Pane:    config.PaneConfig{Slot: "main"},
					Workdir: stateDir,
				},
				{
					Name:            "sandboxed",
					Alias:           "sbx",
					Adapter:         "generic",
					Command:         "fake-agent",
					ExecutionTarget: "sandbox",
					Role: config.RoleSpec{
						Kind:        "implementation",
						Domains:     []string{"session"},
						Description: "Runs in sandbox without pane",
					},
					Pane:    config.PaneConfig{Slot: "right-top"},
					Workdir: stateDir,
				},
			},
		},
	}

	fakeTmux := tmux.NewFakeClient()
	fakeTmux.PaneCaptures["%1"] = "READY\n"

	d := NewDaemon(stateDir, fakeTmux, cfg)
	adapters := d.buildAdapters()
	if len(adapters) != 1 {
		t.Fatalf("buildAdapters len = %d, want 1 pane-backed local adapter", len(adapters))
	}
	if _, ok := adapters["sandboxed"]; ok {
		t.Fatalf("buildAdapters should exclude non-pane-backed sandboxed target")
	}
	d.adapters = adapters

	store := mailbox.NewStore(stateDir)
	writeUnreadMessageAndReceipt := func(agentName string, seq int64) protocol.MessageID {
		body := []byte("review this\n")
		sum := sha256.Sum256(body)
		env := protocol.Envelope{
			Schema:      protocol.MessageSchemaV1,
			ID:          protocol.NewMessageID(seq),
			Seq:         seq,
			Session:     "mixed",
			Thread:      protocol.NewThreadID(seq),
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
		if err := store.CreateMessage(&env, body); err != nil {
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
		if err := store.CreateReceipt(&receipt); err != nil {
			t.Fatal(err)
		}

		return env.ID
	}

	sandboxMsgID := writeUnreadMessageAndReceipt("sandboxed", 1)
	if err := d.tryNotify(context.Background(), "sandboxed", sandboxMsgID, true); err != nil {
		t.Fatalf("tryNotify() for sandboxed target unexpected error: %v", err)
	}

	localMsgID := writeUnreadMessageAndReceipt("local-reviewer", 2)
	if err := d.tryNotify(context.Background(), "local-reviewer", localMsgID, true); err != nil {
		t.Fatalf("tryNotify() for local target unexpected error: %v", err)
	}
	if len(fakeTmux.SendKeysCalls) == 0 {
		t.Fatalf("expected local pane-backed agent notification to send keys")
	}
}
