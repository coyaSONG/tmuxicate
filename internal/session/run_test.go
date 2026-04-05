package session

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestRunRequestValidation(t *testing.T) {
	t.Parallel()

	cfg := testResolvedConfig()
	valid := RunRequest{
		Goal:        "Ship phase 1 coordinator foundations",
		Coordinator: "pm",
		CreatedBy:   "human",
	}

	testCases := []struct {
		name    string
		mutate  func(*RunRequest)
		wantErr string
	}{
		{
			name: "blank goal",
			mutate: func(req *RunRequest) {
				req.Goal = "   "
			},
			wantErr: "goal",
		},
		{
			name: "blank coordinator",
			mutate: func(req *RunRequest) {
				req.Coordinator = ""
			},
			wantErr: "coordinator",
		},
		{
			name: "unknown coordinator",
			mutate: func(req *RunRequest) {
				req.Coordinator = "outsider"
			},
			wantErr: "coordinator",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := valid
			tc.mutate(&req)

			err := req.Validate(cfg)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error to contain %q, got %v", tc.wantErr, err)
			}
		})
	}

	if err := valid.Validate(cfg); err != nil {
		t.Fatalf("expected valid request to pass validation: %v", err)
	}

	alias := valid
	alias.Coordinator = "lead"
	if err := alias.Validate(cfg); err != nil {
		t.Fatalf("expected coordinator alias to pass validation: %v", err)
	}
}

func TestRunRootMessageContract(t *testing.T) {
	t.Parallel()

	run := protocol.CoordinatorRun{
		RunID:         protocol.RunID("run_000000000101"),
		Goal:          "Break the coordinator feature into bounded child tasks",
		Coordinator:   protocol.AgentName("pm"),
		CreatedBy:     protocol.AgentName("human"),
		CreatedAt:     time.Date(2026, time.April, 5, 6, 30, 0, 0, time.UTC),
		RootMessageID: protocol.MessageID("msg_000000000201"),
		RootThreadID:  protocol.ThreadID("thr_000000000201"),
		AllowedOwners: []protocol.AgentName{"builder", "qa"},
		TeamSnapshot: []protocol.AgentSnapshot{
			{Name: "pm", Alias: "lead", Role: "planner", Teammates: []string{"builder", "qa"}},
			{Name: "builder", Alias: "dev", Role: "implementer", Teammates: []string{"pm", "qa"}},
			{Name: "qa", Alias: "tester", Role: "reviewer", Teammates: []string{"pm", "builder"}},
		},
	}

	body, err := BuildRunRootMessageBody(RunRootMessageInput{
		Run: run,
	})
	if err != nil {
		t.Fatalf("build root message body: %v", err)
	}

	requiredSnippets := []string{
		"## Decomposition Instructions",
		"## Run References",
		"tmuxicate run add-task --run run_000000000101",
		"run_id: run_000000000101",
		"root_message_id: msg_000000000201",
		"root_thread_id: thr_000000000201",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(body, snippet) {
			t.Fatalf("expected root message to contain %q\nbody:\n%s", snippet, body)
		}
	}
}

func TestChildTaskValidation(t *testing.T) {
	t.Parallel()

	valid := protocol.ChildTask{
		TaskID:         protocol.TaskID("task_000000000301"),
		ParentRunID:    protocol.RunID("run_000000000101"),
		Owner:          protocol.AgentName("builder"),
		Goal:           "Implement the coordinator artifact writer",
		ExpectedOutput: "run.yaml and task artifacts persisted under coordinator/runs",
		DependsOn:      []protocol.TaskID{"task_000000000111"},
		ReviewRequired: true,
		MessageID:      protocol.MessageID("msg_000000000301"),
		ThreadID:       protocol.ThreadID("thr_000000000301"),
		CreatedAt:      time.Date(2026, time.April, 5, 7, 0, 0, 0, time.UTC),
	}

	testCases := []struct {
		name    string
		mutate  func(*protocol.ChildTask)
		wantErr string
	}{
		{
			name: "missing owner",
			mutate: func(task *protocol.ChildTask) {
				task.Owner = ""
			},
			wantErr: "owner",
		},
		{
			name: "missing goal",
			mutate: func(task *protocol.ChildTask) {
				task.Goal = ""
			},
			wantErr: "goal",
		},
		{
			name: "missing expected_output",
			mutate: func(task *protocol.ChildTask) {
				task.ExpectedOutput = ""
			},
			wantErr: "expected_output",
		},
		{
			name: "missing parent_run_id",
			mutate: func(task *protocol.ChildTask) {
				task.ParentRunID = ""
			},
			wantErr: "parent_run_id",
		},
		{
			name: "blank depends_on entry",
			mutate: func(task *protocol.ChildTask) {
				task.DependsOn = []protocol.TaskID{"task_000000000111", ""}
			},
			wantErr: "depends_on",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			task := valid
			tc.mutate(&task)

			err := task.Validate()
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error to contain %q, got %v", tc.wantErr, err)
			}
		})
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid child task to pass validation: %v", err)
	}
}

func TestCoordinatorPathsStayInsideStateDir(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	runID := protocol.RunID("run_000000000101")
	taskID := protocol.TaskID("task_000000000301")

	runDir := mailbox.RunDir(stateDir, runID)
	runFile := mailbox.RunFilePath(stateDir, runID)
	taskFile := mailbox.RunTaskPath(stateDir, runID, taskID)

	expectedRunDir := filepath.Join(stateDir, "coordinator", "runs", string(runID))
	expectedRunFile := filepath.Join(stateDir, "coordinator", "runs", string(runID), "run.yaml")
	expectedTaskFile := filepath.Join(stateDir, "coordinator", "runs", string(runID), "tasks", string(taskID)+".yaml")

	if runDir != expectedRunDir {
		t.Fatalf("unexpected run dir: got %q want %q", runDir, expectedRunDir)
	}
	if runFile != expectedRunFile {
		t.Fatalf("unexpected run file path: got %q want %q", runFile, expectedRunFile)
	}
	if taskFile != expectedTaskFile {
		t.Fatalf("unexpected task file path: got %q want %q", taskFile, expectedTaskFile)
	}

	for _, path := range []string{runDir, runFile, taskFile} {
		rel, err := filepath.Rel(stateDir, path)
		if err != nil {
			t.Fatalf("relative path check failed for %q: %v", path, err)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("expected path %q to stay inside state dir %q", path, stateDir)
		}
	}
}

func testResolvedConfig() *config.ResolvedConfig {
	return &config.ResolvedConfig{
		Config: config.Config{
			Session: config.SessionConfig{
				Name:      "coord-foundation",
				Workspace: "/tmp/workspace",
				StateDir:  "/tmp/state",
			},
			Agents: []config.AgentConfig{
				{Name: "pm", Alias: "lead", Role: "planner", Teammates: []string{"builder", "qa"}},
				{Name: "builder", Alias: "dev", Role: "implementer", Teammates: []string{"pm", "qa"}},
				{Name: "qa", Alias: "tester", Role: "reviewer", Teammates: []string{"pm", "builder"}},
			},
		},
	}
}
