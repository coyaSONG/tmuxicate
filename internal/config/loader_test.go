package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "tmuxicate.yaml")
	writeTestFile(t, cfgPath, `
version: 1
session:
  name: tmuxicate-dev
  workspace: .
  state_dir: .tmuxicate/sessions/dev
  window_name: agents
  layout: triad
  attach: true
delivery:
  mode: notify_then_read
  ack_timeout: 2m
  retry_interval: 30s
  max_retries: 3
  safe_notify_only_when_ready: true
  auto_notify: true
transcript:
  mode: pipe-pane
  dir: .tmuxicate/sessions/dev/transcripts
routing:
  coordinator: coordinator
  exclusive_task_kinds:
    - task
  fanout_task_kinds:
    - review_request
    - question
defaults:
  workdir: .
  env:
    TMUXICATE_SESSION: tmuxicate-dev
  bootstrap_template: default
  notify:
    enabled: true
agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role: coordinator
    pane:
      slot: main
    teammates:
      - backend
  - name: backend
    alias: api
    adapter: claude-code
    command: claude
    role: backend
    pane:
      slot: right-top
    teammates:
      - coordinator
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.Version != 1 {
		t.Fatalf("Version = %d, want 1", cfg.Version)
	}
	if !filepath.IsAbs(cfg.Session.Workspace) {
		t.Fatalf("Session.Workspace should be absolute, got %q", cfg.Session.Workspace)
	}
	if !filepath.IsAbs(cfg.Session.StateDir) {
		t.Fatalf("Session.StateDir should be absolute, got %q", cfg.Session.StateDir)
	}
	if !filepath.IsAbs(cfg.Transcript.Dir) {
		t.Fatalf("Transcript.Dir should be absolute, got %q", cfg.Transcript.Dir)
	}
	if !filepath.IsAbs(cfg.Agents[0].Workdir) {
		t.Fatalf("Agent workdir should be absolute, got %q", cfg.Agents[0].Workdir)
	}
}

func TestLoadMissingRequiredFields(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "tmuxicate.yaml")
	writeTestFile(t, cfgPath, `
version: 1
session:
  workspace: .
  state_dir: .tmuxicate/sessions/dev
agents: []
`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "session.name") {
		t.Fatalf("Load() error = %q, want session.name failure", err)
	}
}

func TestLoadDuplicateAgentNames(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "tmuxicate.yaml")
	writeTestFile(t, cfgPath, `
version: 1
session:
  name: dev
  workspace: .
  state_dir: .tmuxicate/sessions/dev
routing:
  coordinator: coordinator
agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role: coordinator
    pane:
      slot: main
  - name: coordinator
    alias: api
    adapter: generic
    command: fake-agent
    role: backend
    pane:
      slot: right-top
`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate agent name") {
		t.Fatalf("Load() error = %q, want duplicate agent name", err)
	}
}

func TestResolvePathResolution(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version: 1,
		Session: SessionConfig{
			Name:      "dev",
			Workspace: ".",
			StateDir:  ".tmuxicate/sessions/dev",
		},
		Delivery: DeliveryConfig{
			Mode:          "notify_then_read",
			AckTimeout:    Duration(120000000000),
			RetryInterval: Duration(30000000000),
			MaxRetries:    3,
		},
		Transcript: TranscriptConfig{
			Mode: "pipe-pane",
		},
		Routing: RoutingConfig{
			Coordinator: "coordinator",
		},
		Defaults: DefaultsConfig{
			Workdir: ".",
			Notify: NotifyConfig{
				Enabled: boolPtr(true),
			},
		},
		Agents: []AgentConfig{
			{
				Name:    "coordinator",
				Alias:   "pm",
				Adapter: "codex",
				Command: "codex",
				Role:    "coordinator",
				Pane:    PaneConfig{Slot: "main"},
			},
		},
	}

	base := t.TempDir()
	resolved, err := cfg.Resolve(base)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	if resolved.Session.Workspace != filepath.Join(base, ".") && !filepath.IsAbs(resolved.Session.Workspace) {
		t.Fatalf("resolved workspace should be absolute, got %q", resolved.Session.Workspace)
	}
	if !filepath.IsAbs(resolved.Session.StateDir) {
		t.Fatalf("resolved state dir should be absolute, got %q", resolved.Session.StateDir)
	}
	if !filepath.IsAbs(resolved.Transcript.Dir) {
		t.Fatalf("resolved transcript dir should be absolute, got %q", resolved.Transcript.Dir)
	}
	if !filepath.IsAbs(resolved.Agents[0].Workdir) {
		t.Fatalf("resolved agent workdir should be absolute, got %q", resolved.Agents[0].Workdir)
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", path, err)
	}
}
