package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestLoadValidConfigWithStructuredRoles(t *testing.T) {
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
  exclusive_task_classes:
    - implementation
  fanout_task_classes:
    - review
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
    role:
      kind: research
      domains: [routing]
      description: Coordinates routing and research work
    route_priority: 100
    pane:
      slot: main
    teammates:
      - backend
  - name: backend
    alias: api
    adapter: claude-code
    command: claude
    role:
      kind: implementation
      domains: [session, protocol]
      description: Owns run/session changes
    route_priority: 20
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

func TestLoadValidConfigWithExecutionTargetsAndLocalFallback(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "tmuxicate.yaml")
	writeTestFile(t, cfgPath, `
version: 1
session:
  name: remote-targets
  workspace: .
  state_dir: .tmuxicate/sessions/dev
  window_name: agents
  layout: triad
delivery:
  mode: notify_then_read
  ack_timeout: 2m
  retry_interval: 30s
  max_retries: 3
transcript:
  mode: pipe-pane
  dir: .tmuxicate/sessions/dev/transcripts
routing:
  coordinator: coordinator
defaults:
  workdir: .
execution_targets:
  - name: sandbox
    kind: sandbox
    description: Sandboxed execution host
    capabilities: [sandbox, ephemeral, sandbox]
    pane_backed: false
  - name: remote-linux
    kind: remote
    description: Remote Linux worker
    capabilities: [ssh, linux]
    pane_backed: false
agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role:
      kind: research
      domains: [routing]
    pane:
      slot: main
    teammates: [sandboxed, remote, local]
  - name: sandboxed
    alias: sbx
    adapter: generic
    command: fake-agent
    role:
      kind: implementation
      domains: [session]
    execution_target: sandbox
    pane:
      slot: right-top
    teammates: [coordinator]
  - name: remote
    alias: ssh
    adapter: generic
    command: fake-agent
    role:
      kind: implementation
      domains: [protocol]
    execution_target: remote-linux
    pane:
      slot: right-bottom
    teammates: [coordinator]
  - name: local
    alias: local
    adapter: claude-code
    command: claude
    role:
      kind: review
      domains: [session]
    pane:
      slot: left-bottom
    teammates: [coordinator]
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if len(cfg.ExecutionTargets) != 2 {
		t.Fatalf("ExecutionTargets = %d, want 2", len(cfg.ExecutionTargets))
	}
	if cfg.ExecutionTargets[0].Name != "sandbox" || cfg.ExecutionTargets[0].Kind != "sandbox" {
		t.Fatalf("sandbox target = %#v, want sandbox target round-trip", cfg.ExecutionTargets[0])
	}
	if cfg.ExecutionTargets[1].Name != "remote-linux" || cfg.ExecutionTargets[1].Kind != "remote" {
		t.Fatalf("remote target = %#v, want remote target round-trip", cfg.ExecutionTargets[1])
	}
	if cfg.Agents[1].ExecutionTarget != "sandbox" {
		t.Fatalf("sandboxed agent execution_target = %q, want sandbox", cfg.Agents[1].ExecutionTarget)
	}
	if cfg.Agents[2].ExecutionTarget != "remote-linux" {
		t.Fatalf("remote agent execution_target = %q, want remote-linux", cfg.Agents[2].ExecutionTarget)
	}
	if cfg.Agents[3].ExecutionTarget != "" {
		t.Fatalf("local agent execution_target = %q, want implicit local fallback", cfg.Agents[3].ExecutionTarget)
	}

	localOnlyPath := filepath.Join(tmpDir, "local-only.yaml")
	writeTestFile(t, localOnlyPath, `
version: 1
session:
  name: local-only
  workspace: .
  state_dir: .tmuxicate/sessions/local
  window_name: agents
  layout: triad
delivery:
  mode: notify_then_read
  ack_timeout: 2m
  retry_interval: 30s
  max_retries: 3
transcript:
  mode: pipe-pane
  dir: .tmuxicate/sessions/local/transcripts
routing:
  coordinator: coordinator
defaults:
  workdir: .
agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role:
      kind: research
      domains: [routing]
    pane:
      slot: main
`)

	localOnly, err := Load(localOnlyPath)
	if err != nil {
		t.Fatalf("Load() local-only config unexpected error: %v", err)
	}
	if len(localOnly.ExecutionTargets) != 0 {
		t.Fatalf("local-only ExecutionTargets = %#v, want none", localOnly.ExecutionTargets)
	}
	if localOnly.Agents[0].ExecutionTarget != "" {
		t.Fatalf("local-only agent execution_target = %q, want implicit local fallback", localOnly.Agents[0].ExecutionTarget)
	}
}

func TestLoadExecutionTargetsRejectsUnknownBindingsAndDuplicateNames(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		contents string
		wantErr  string
	}{
		{
			name: "duplicate target names",
			contents: `
version: 1
session:
  name: invalid-targets
  workspace: .
  state_dir: .tmuxicate/sessions/dev
  window_name: agents
  layout: triad
delivery:
  mode: notify_then_read
  ack_timeout: 2m
  retry_interval: 30s
  max_retries: 3
transcript:
  mode: pipe-pane
  dir: .tmuxicate/sessions/dev/transcripts
routing:
  coordinator: coordinator
defaults:
  workdir: .
execution_targets:
  - name: sandbox
    kind: sandbox
    pane_backed: false
  - name: sandbox
    kind: remote
    pane_backed: false
agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role:
      kind: research
      domains: [routing]
    pane:
      slot: main
`,
			wantErr: "duplicate execution target name",
		},
		{
			name: "invalid target kind",
			contents: `
version: 1
session:
  name: invalid-target-kind
  workspace: .
  state_dir: .tmuxicate/sessions/dev
  window_name: agents
  layout: triad
delivery:
  mode: notify_then_read
  ack_timeout: 2m
  retry_interval: 30s
  max_retries: 3
transcript:
  mode: pipe-pane
  dir: .tmuxicate/sessions/dev/transcripts
routing:
  coordinator: coordinator
defaults:
  workdir: .
execution_targets:
  - name: sandbox
    kind: hovercraft
    pane_backed: false
agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role:
      kind: research
      domains: [routing]
    pane:
      slot: main
`,
			wantErr: "invalid execution_targets[0].kind",
		},
		{
			name: "unknown agent execution target binding",
			contents: `
version: 1
session:
  name: invalid-target-binding
  workspace: .
  state_dir: .tmuxicate/sessions/dev
  window_name: agents
  layout: triad
delivery:
  mode: notify_then_read
  ack_timeout: 2m
  retry_interval: 30s
  max_retries: 3
transcript:
  mode: pipe-pane
  dir: .tmuxicate/sessions/dev/transcripts
routing:
  coordinator: coordinator
defaults:
  workdir: .
execution_targets:
  - name: sandbox
    kind: sandbox
    pane_backed: false
agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role:
      kind: research
      domains: [routing]
    execution_target: missing-target
    pane:
      slot: main
`,
			wantErr: "unknown execution target",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			cfgPath := filepath.Join(tmpDir, "tmuxicate.yaml")
			writeTestFile(t, cfgPath, tc.contents)

			_, err := Load(cfgPath)
			if err == nil {
				t.Fatal("Load() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Load() error = %q, want %q", err, tc.wantErr)
			}
		})
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
    role:
      kind: research
      domains: [routing]
      description: Coordinates routing and research work
    pane:
      slot: main
  - name: coordinator
    alias: api
    adapter: generic
    command: fake-agent
    role:
      kind: implementation
      domains: [session, protocol]
      description: Owns run/session changes
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
			ExclusiveTaskClasses: []protocol.TaskClass{
				protocol.TaskClassImplementation,
			},
			FanoutTaskClasses: []protocol.TaskClass{
				protocol.TaskClassReview,
			},
		},
		Defaults: DefaultsConfig{
			Workdir: ".",
			Notify: NotifyConfig{
				Enabled: boolPtr(true),
			},
		},
		Agents: []AgentConfig{
			{
				Name:          "coordinator",
				Alias:         "pm",
				Adapter:       "codex",
				Command:       "codex",
				RoutePriority: 100,
				Role: RoleSpec{
					Kind:        string(protocol.TaskClassResearch),
					Domains:     []string{"routing"},
					Description: "Coordinates routing and research work",
				},
				Pane: PaneConfig{Slot: "main"},
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

func TestLoadValidConfigWithBlockerRerouteCeilings(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "tmuxicate.yaml")
	writeTestFile(t, cfgPath, `
version: 1
session:
  name: blocker-dev
  workspace: .
  state_dir: .tmuxicate/sessions/dev
routing:
  coordinator: coordinator
blockers:
  max_reroutes_default: 1
  max_reroutes_by_task_class:
    implementation: 1
    research: 1
    review: 0
agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role:
      kind: research
      domains: [routing]
      description: Coordinates routing work
    pane:
      slot: main
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.Blockers.MaxReroutesDefault != 1 {
		t.Fatalf("Blockers.MaxReroutesDefault = %d, want 1", cfg.Blockers.MaxReroutesDefault)
	}
	if got := cfg.Blockers.MaxReroutesByTaskClass[protocol.TaskClassImplementation]; got != 1 {
		t.Fatalf("implementation reroute ceiling = %d, want 1", got)
	}
	if got := cfg.Blockers.MaxReroutesByTaskClass[protocol.TaskClassResearch]; got != 1 {
		t.Fatalf("research reroute ceiling = %d, want 1", got)
	}
	if got := cfg.Blockers.MaxReroutesByTaskClass[protocol.TaskClassReview]; got != 0 {
		t.Fatalf("review reroute ceiling = %d, want 0", got)
	}
}

func TestLoadRejectsInvalidBlockerRerouteCeilings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		blockersYML string
		wantSubstr  string
	}{
		{
			name: "invalid task class key",
			blockersYML: `
blockers:
  max_reroutes_default: 1
  max_reroutes_by_task_class:
    invalid: 1
`,
			wantSubstr: "blockers.max_reroutes_by_task_class",
		},
		{
			name: "negative default",
			blockersYML: `
blockers:
  max_reroutes_default: -1
`,
			wantSubstr: "blockers.max_reroutes_default",
		},
		{
			name: "negative override",
			blockersYML: `
blockers:
  max_reroutes_default: 1
  max_reroutes_by_task_class:
    review: -1
`,
			wantSubstr: "blockers.max_reroutes_by_task_class",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			cfgPath := filepath.Join(tmpDir, "tmuxicate.yaml")
			writeTestFile(t, cfgPath, `
version: 1
session:
  name: blocker-dev
  workspace: .
  state_dir: .tmuxicate/sessions/dev
routing:
  coordinator: coordinator
`+tt.blockersYML+`
agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role:
      kind: research
      domains: [routing]
      description: Coordinates routing work
    pane:
      slot: main
`)

			_, err := Load(cfgPath)
			if err == nil {
				t.Fatal("Load() expected error, got nil")
			}
			if !strings.Contains(err.Error(), "blockers.") {
				t.Fatalf("Load() error = %q, want blockers.* failure", err)
			}
			if !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Fatalf("Load() error = %q, want substring %q", err, tt.wantSubstr)
			}
		})
	}
}

func TestLoadAdaptiveRoutingConfigWithManualPreferences(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "tmuxicate.yaml")
	writeTestFile(t, cfgPath, `
version: 1
session:
  name: adaptive-routing
  workspace: .
  state_dir: .tmuxicate/sessions/adaptive
routing:
  coordinator: coordinator
  adaptive:
    enabled: true
    lookback_runs: 3
    success_weight: 4
    approval_weight: 3
    changes_requested_penalty: 2
    blocked_penalty: 5
    wait_penalty: 1
    manual_preferences:
      - task_class: implementation
        domains: [session, protocol]
        preferred_owner: backend-senior
        weight: 2
        reason: "Keeps protocol-heavy work on the same owner when the signal is explicit"
agents:
  - name: coordinator
    alias: pm
    adapter: codex
    command: codex
    role:
      kind: research
      domains: [routing]
      description: Coordinates adaptive routing work
    pane:
      slot: main
    teammates:
      - backend-senior
  - name: backend-senior
    alias: api
    adapter: claude-code
    command: claude
    role:
      kind: implementation
      domains: [protocol, session]
      description: Owns protocol-heavy implementation work
    pane:
      slot: right-top
    teammates:
      - coordinator
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if !cfg.Routing.Adaptive.Enabled {
		t.Fatalf("Routing.Adaptive.Enabled = false, want true")
	}
	if cfg.Routing.Adaptive.LookbackRuns != 3 {
		t.Fatalf("Routing.Adaptive.LookbackRuns = %d, want 3", cfg.Routing.Adaptive.LookbackRuns)
	}
	if cfg.Routing.Adaptive.SuccessWeight != 4 {
		t.Fatalf("Routing.Adaptive.SuccessWeight = %d, want 4", cfg.Routing.Adaptive.SuccessWeight)
	}
	if cfg.Routing.Adaptive.ApprovalWeight != 3 {
		t.Fatalf("Routing.Adaptive.ApprovalWeight = %d, want 3", cfg.Routing.Adaptive.ApprovalWeight)
	}
	if cfg.Routing.Adaptive.ChangesRequestedPenalty != 2 {
		t.Fatalf("Routing.Adaptive.ChangesRequestedPenalty = %d, want 2", cfg.Routing.Adaptive.ChangesRequestedPenalty)
	}
	if cfg.Routing.Adaptive.BlockedPenalty != 5 {
		t.Fatalf("Routing.Adaptive.BlockedPenalty = %d, want 5", cfg.Routing.Adaptive.BlockedPenalty)
	}
	if cfg.Routing.Adaptive.WaitPenalty != 1 {
		t.Fatalf("Routing.Adaptive.WaitPenalty = %d, want 1", cfg.Routing.Adaptive.WaitPenalty)
	}
	if len(cfg.Routing.Adaptive.ManualPreferences) != 1 {
		t.Fatalf("Routing.Adaptive.ManualPreferences = %d, want 1", len(cfg.Routing.Adaptive.ManualPreferences))
	}

	preference := cfg.Routing.Adaptive.ManualPreferences[0]
	if preference.TaskClass != protocol.TaskClassImplementation {
		t.Fatalf("preference.TaskClass = %q, want %q", preference.TaskClass, protocol.TaskClassImplementation)
	}
	if got, want := preference.Domains, []string{"protocol", "session"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("preference.Domains = %#v, want %#v", got, want)
	}
	if preference.PreferredOwner != "backend-senior" {
		t.Fatalf("preference.PreferredOwner = %q, want %q", preference.PreferredOwner, "backend-senior")
	}
	if preference.Weight != 2 {
		t.Fatalf("preference.Weight = %d, want 2", preference.Weight)
	}
	if preference.Reason != "Keeps protocol-heavy work on the same owner when the signal is explicit" {
		t.Fatalf("preference.Reason = %q, want exact fixture reason", preference.Reason)
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", path, err)
	}
}
