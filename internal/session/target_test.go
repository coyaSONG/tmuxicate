package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestRouteChildTaskDispatchesNonPaneTarget(t *testing.T) {
	t.Parallel()

	cfg := testExecutionTargetRouteConfig(t)
	logPath := filepath.Join(t.TempDir(), "dispatch.log")
	cfg.ExecutionTargets[0].Dispatch.Command = `printf '%s:%s:%s' "$TMUXICATE_AGENT" "$TMUXICATE_MESSAGE_ID" "$TMUXICATE_TASK_ID" > "$DISPATCH_LOG"`
	cfg.ExecutionTargets[0].Dispatch.Env = map[string]string{"DISPATCH_LOG": logPath}

	if err := os.MkdirAll(cfg.Session.Workspace, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := createStateTree(cfg); err != nil {
		t.Fatalf("create state tree: %v", err)
	}

	store := mailbox.NewStore(cfg.Session.StateDir)
	run, err := Run(cfg, store, RunRequest{
		Goal:        "Dispatch remote implementation work",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	task, _, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
		RunID:          run.RunID,
		TaskClass:      protocol.TaskClassImplementation,
		Domains:        []string{"session", "protocol"},
		Goal:           "Execute implementation through remote dispatch",
		ExpectedOutput: "Dispatch command receives canonical task identifiers",
	})
	if err != nil {
		t.Fatalf("route child task: %v", err)
	}

	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read dispatch log: %v", err)
	}
	wantSnippet := strings.Join([]string{"backend-high", string(task.MessageID), string(task.TaskID)}, ":")
	if string(got) != wantSnippet {
		t.Fatalf("dispatch log = %q, want %q", string(got), wantSnippet)
	}

	record, err := mailbox.ReadTargetDispatch(cfg.Session.StateDir, "sandbox", task.MessageID)
	if err != nil {
		t.Fatalf("read target dispatch: %v", err)
	}
	if record.Status != mailbox.TargetDispatchDispatched {
		t.Fatalf("dispatch status = %q, want %q", record.Status, mailbox.TargetDispatchDispatched)
	}
}

func TestRouteChildTaskSkipsDisabledTarget(t *testing.T) {
	t.Parallel()

	cfg := testExecutionTargetRouteConfig(t)
	if err := os.MkdirAll(cfg.Session.Workspace, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := createStateTree(cfg); err != nil {
		t.Fatalf("create state tree: %v", err)
	}

	target := protocol.ExecutionTarget{
		Name:         "sandbox",
		Kind:         "sandbox",
		Description:  "Sandbox worker",
		Capabilities: []string{"sandbox", "ephemeral"},
		PaneBacked:   false,
	}
	if _, err := mailbox.RecordTargetHeartbeat(cfg.Session.StateDir, &target, mailbox.TargetAvailabilityDisabled, "operator disabled sandbox", "operator", nil); err != nil {
		t.Fatalf("disable sandbox target: %v", err)
	}

	store := mailbox.NewStore(cfg.Session.StateDir)
	run, err := Run(cfg, store, RunRequest{
		Goal:        "Route work away from disabled targets",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	task, decision, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
		RunID:          run.RunID,
		TaskClass:      protocol.TaskClassImplementation,
		Domains:        []string{"session", "protocol"},
		Goal:           "Prefer a healthy target",
		ExpectedOutput: "Routing excludes disabled target owners",
	})
	if err != nil {
		t.Fatalf("route child task: %v", err)
	}

	if task.Owner != "backend-later" {
		t.Fatalf("task owner = %q, want %q", task.Owner, "backend-later")
	}
	if decision == nil || len(decision.ExcludedTargets) == 0 {
		t.Fatalf("expected excluded targets in routing decision, got %#v", decision)
	}
	if decision.ExcludedTargets[0].TargetName != "sandbox" || decision.ExcludedTargets[0].Status != string(mailbox.TargetAvailabilityDisabled) {
		t.Fatalf("excluded targets = %#v, want sandbox disabled", decision.ExcludedTargets)
	}
}

func TestEnableTargetRedispatchesPendingUnreadTasks(t *testing.T) {
	t.Parallel()

	cfg := testExecutionTargetRouteConfig(t)
	logPath := filepath.Join(t.TempDir(), "redispatch.log")
	if err := os.MkdirAll(cfg.Session.Workspace, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := createStateTree(cfg); err != nil {
		t.Fatalf("create state tree: %v", err)
	}
	makeConfigLoadable(cfg)
	cfg.ExecutionTargets[0].Dispatch.Command = ""
	cfg.ExecutionTargets[0].Dispatch.Env = nil
	if err := writeResolvedConfig(cfg); err != nil {
		t.Fatalf("write resolved config: %v", err)
	}

	store := mailbox.NewStore(cfg.Session.StateDir)
	run, err := Run(cfg, store, RunRequest{
		Goal:        "Redispatch pending unread tasks after recovery",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	task, _, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
		RunID:          run.RunID,
		TaskClass:      protocol.TaskClassImplementation,
		Domains:        []string{"session", "protocol"},
		Goal:           "Queue work while dispatch is unavailable",
		ExpectedOutput: "enable target should redispatch unread work",
	})
	if err != nil {
		t.Fatalf("route child task: %v", err)
	}

	pending, err := mailbox.ReadTargetDispatch(cfg.Session.StateDir, "sandbox", task.MessageID)
	if err != nil {
		t.Fatalf("read pending dispatch: %v", err)
	}
	if pending.Status != mailbox.TargetDispatchPending {
		t.Fatalf("pending dispatch status = %q, want %q", pending.Status, mailbox.TargetDispatchPending)
	}

	cfg.ExecutionTargets[0].Dispatch.Command = `printf '%s' "$TMUXICATE_MESSAGE_ID" > "$REDISPATCH_LOG"`
	cfg.ExecutionTargets[0].Dispatch.Env = map[string]string{"REDISPATCH_LOG": logPath}
	if err := writeResolvedConfig(cfg); err != nil {
		t.Fatalf("rewrite resolved config with dispatch command: %v", err)
	}

	_, redispatched, err := EnableTarget(cfg.Session.StateDir, "sandbox", "launcher restored")
	if err != nil {
		t.Fatalf("enable target: %v", err)
	}
	if redispatched != 1 {
		t.Fatalf("redispatched = %d, want 1", redispatched)
	}

	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read redispatch log: %v", err)
	}
	if strings.TrimSpace(string(got)) != string(task.MessageID) {
		t.Fatalf("redispatch log = %q, want %q", strings.TrimSpace(string(got)), task.MessageID)
	}
}
