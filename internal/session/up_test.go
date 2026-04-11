package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"github.com/coyaSONG/tmuxicate/internal/tmux"
)

func TestUpSkipsPaneLifecycleForNonPaneBackedTargets(t *testing.T) {
	t.Parallel()

	cfg := testMixedTargetUpConfig(t)
	fakeTmux := tmux.NewFakeClient()

	originalStartBackgroundDaemon := startBackgroundDaemonFn
	startBackgroundDaemonFn = func(*config.ResolvedConfig) error { return nil }
	defer func() {
		startBackgroundDaemonFn = originalStartBackgroundDaemon
	}()

	if err := Up(cfg, fakeTmux); err != nil {
		t.Fatalf("Up() unexpected error: %v", err)
	}

	if len(fakeTmux.NewSessionCalls) != 1 {
		t.Fatalf("NewSessionCalls len = %d, want 1", len(fakeTmux.NewSessionCalls))
	}
	if len(fakeTmux.SplitPaneCalls) != 1 {
		t.Fatalf("SplitPaneCalls len = %d, want 1 for one additional local pane-backed agent", len(fakeTmux.SplitPaneCalls))
	}
	if len(fakeTmux.PipePaneCalls) != 2 {
		t.Fatalf("PipePaneCalls len = %d, want 2 for pane-backed local agents only", len(fakeTmux.PipePaneCalls))
	}

	bootstrapPath := filepath.Join(mailbox.AgentDir(cfg.Session.StateDir, "sandboxed"), "adapter", "bootstrap.txt")
	if _, err := os.Stat(bootstrapPath); err != nil {
		t.Fatalf("expected sandbox bootstrap artifact, stat err = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfg.Session.StateDir, "runtime", "ready.json"))
	if err != nil {
		t.Fatalf("read ready.json: %v", err)
	}

	var ready struct {
		Agents map[string]string `json:"agents"`
	}
	if err := json.Unmarshal(data, &ready); err != nil {
		t.Fatalf("unmarshal ready.json: %v", err)
	}

	if len(ready.Agents) != 2 {
		t.Fatalf("ready agents len = %d, want 2 pane-backed local agents", len(ready.Agents))
	}
	if _, ok := ready.Agents["sandboxed"]; ok {
		t.Fatalf("ready agents should exclude non-pane-backed sandboxed target: %#v", ready.Agents)
	}
}

func testMixedTargetUpConfig(t *testing.T) *config.ResolvedConfig {
	t.Helper()

	baseDir := t.TempDir()
	workspaceDir := filepath.Join(baseDir, "workspace")
	stateDir := filepath.Join(baseDir, "state")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace dir: %v", err)
	}

	return &config.ResolvedConfig{
		Config: config.Config{
			Version: 1,
			Session: config.SessionConfig{
				Name:       "mixed-target-up",
				Workspace:  workspaceDir,
				StateDir:   stateDir,
				WindowName: "agents",
				Layout:     "triad",
			},
			Delivery: config.DeliveryConfig{
				Mode:          "notify_then_read",
				AckTimeout:    config.Duration(2 * 60 * 1e9),
				RetryInterval: config.Duration(30 * 1e9),
				MaxRetries:    3,
			},
			Transcript: config.TranscriptConfig{
				Mode: "pipe-pane",
				Dir:  filepath.Join(stateDir, "transcripts"),
			},
			Routing: config.RoutingConfig{
				Coordinator: "pm",
			},
			Defaults: config.DefaultsConfig{
				Workdir: workspaceDir,
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
					Name:    "pm",
					Alias:   "lead",
					Adapter: "generic",
					Command: "fake-agent",
					Role: config.RoleSpec{
						Kind:        string(protocol.TaskClassResearch),
						Domains:     []string{"routing"},
						Description: "Coordinates mixed target startup",
					},
					Pane:      config.PaneConfig{Slot: "main"},
					Teammates: []string{"sandboxed", "reviewer"},
					Workdir:   workspaceDir,
				},
				{
					Name:            "sandboxed",
					Alias:           "sbx",
					Adapter:         "generic",
					Command:         "fake-agent",
					ExecutionTarget: "sandbox",
					Role: config.RoleSpec{
						Kind:        string(protocol.TaskClassImplementation),
						Domains:     []string{"session"},
						Description: "Runs in sandbox",
					},
					Pane:      config.PaneConfig{Slot: "right-top"},
					Teammates: []string{"pm"},
					Workdir:   workspaceDir,
				},
				{
					Name:    "reviewer",
					Alias:   "qa",
					Adapter: "generic",
					Command: "fake-agent",
					Role: config.RoleSpec{
						Kind:        string(protocol.TaskClassReview),
						Domains:     []string{"session"},
						Description: "Local pane-backed reviewer",
					},
					Pane:      config.PaneConfig{Slot: "right-bottom"},
					Teammates: []string{"pm"},
					Workdir:   workspaceDir,
				},
			},
		},
		ConfigDir: baseDir,
	}
}
