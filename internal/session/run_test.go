package session

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"gopkg.in/yaml.v3"
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

func TestRunRootMessageContractUsesRouteTaskCommand(t *testing.T) {
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
			{Name: "pm", Alias: "lead", Role: "research", Teammates: []string{"builder", "qa"}},
			{Name: "builder", Alias: "dev", Role: "implementation", Teammates: []string{"pm", "qa"}},
			{Name: "qa", Alias: "tester", Role: "review", Teammates: []string{"pm", "builder"}},
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
		"tmuxicate run route-task --run run_000000000101",
		"--task-class <class>",
		"--domain <domain>",
		"--goal \"<goal>\" --expected-output \"<deliverable>\"",
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

func TestRouteChildTaskSelectsDeterministicOwner(t *testing.T) {
	t.Parallel()

	cfg := testRouteTaskConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := Run(cfg, store, RunRequest{
		Goal:        "Route an implementation task to the most specific backend owner",
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
		Goal:           "Implement structured role-aware routing",
		ExpectedOutput: "A deterministic route-task workflow with inspectable evidence",
		ReviewRequired: true,
	})
	if err != nil {
		t.Fatalf("route child task: %v", err)
	}

	if task.Owner != "backend-high" {
		t.Fatalf("task owner = %q, want %q", task.Owner, "backend-high")
	}
	if decision == nil {
		t.Fatalf("expected routing decision to be returned")
	}
	if decision.SelectedOwner != "backend-high" {
		t.Fatalf("selected owner = %q, want %q", decision.SelectedOwner, "backend-high")
	}
	if decision.TieBreak != "route_priority desc, config_order asc" {
		t.Fatalf("tie break = %q, want %q", decision.TieBreak, "route_priority desc, config_order asc")
	}

	wantCandidates := []protocol.AgentName{"backend-high", "backend-later", "backend-low"}
	if !reflect.DeepEqual(decision.Candidates, wantCandidates) {
		t.Fatalf("candidates = %#v, want %#v", decision.Candidates, wantCandidates)
	}
}

func TestRouteChildTaskRejectsNoMatchWithStructuredReason(t *testing.T) {
	t.Parallel()

	cfg := testRouteTaskConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := Run(cfg, store, RunRequest{
		Goal:        "Route an implementation task to the most specific backend owner",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	_, _, err = RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
		RunID:          run.RunID,
		TaskClass:      protocol.TaskClassImplementation,
		Domains:        []string{"frontend"},
		Goal:           "Implement frontend routing even though no owner can cover it",
		ExpectedOutput: "A fail-loud structured rejection",
	})
	if err == nil {
		t.Fatalf("expected no-match route request to fail")
	}

	var rejection *protocol.RouteRejection
	if !errors.As(err, &rejection) {
		t.Fatalf("expected structured route rejection, got %T: %v", err, err)
	}

	if rejection.TaskClass != protocol.TaskClassImplementation {
		t.Fatalf("task_class = %q, want %q", rejection.TaskClass, protocol.TaskClassImplementation)
	}
	if !reflect.DeepEqual(rejection.Domains, []string{"frontend"}) {
		t.Fatalf("domains = %#v, want %#v", rejection.Domains, []string{"frontend"})
	}
	if len(rejection.EligibleCandidates) == 0 {
		t.Fatalf("eligible_candidates should not be empty: %#v", rejection)
	}
	if len(rejection.AllowedOwners) == 0 {
		t.Fatalf("allowed_owners should not be empty: %#v", rejection)
	}
	if len(rejection.Suggestions) == 0 {
		t.Fatalf("suggestions should not be empty: %#v", rejection)
	}
	if !slices.Contains(rejection.AllowedOwners, protocol.AgentName("backend-high")) {
		t.Fatalf("allowed_owners = %#v, want backend-high present", rejection.AllowedOwners)
	}
}

func TestRouteChildTaskBlocksExclusiveDuplicate(t *testing.T) {
	t.Parallel()

	cfg := testRouteTaskConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := Run(cfg, store, RunRequest{
		Goal:        "Route implementation work without duplicate execution",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	firstTask, _, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
		RunID:          run.RunID,
		TaskClass:      protocol.TaskClassImplementation,
		Domains:        []string{"session", "protocol"},
		Goal:           "Implement duplicate routing safeguards",
		ExpectedOutput: "One implementation task for protocol/session work",
	})
	if err != nil {
		t.Fatalf("first route child task: %v", err)
	}

	const duplicateKeyTemplate = "run_<id>|implementation|protocol,session"
	wantDuplicateKey := string(run.RunID) + "|implementation|protocol,session"

	_, _, err = RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
		RunID:          run.RunID,
		TaskClass:      protocol.TaskClassImplementation,
		Domains:        []string{"protocol", "session"},
		OwnerOverride:  protocol.AgentName("backend-low"),
		OverrideReason: "manual routing review",
		Goal:           "Re-run the same implementation task with reordered domains",
		ExpectedOutput: "Duplicate routes are blocked before owner selection",
	})
	if err == nil {
		t.Fatalf("expected duplicate route to fail with duplicate_key %q (template %q)", wantDuplicateKey, duplicateKeyTemplate)
	}
	if !strings.Contains(err.Error(), "duplicate_key") {
		t.Fatalf("expected duplicate error to mention duplicate_key, got %v", err)
	}
	if !strings.Contains(err.Error(), wantDuplicateKey) {
		t.Fatalf("expected duplicate error to contain duplicate_key %q, got %v", wantDuplicateKey, err)
	}
	if !strings.Contains(err.Error(), string(firstTask.TaskID)) {
		t.Fatalf("expected duplicate error to mention matched task id %q, got %v", firstTask.TaskID, err)
	}
}

func TestRouteChildTaskAllowsFanoutReviewClass(t *testing.T) {
	t.Parallel()

	cfg := testRouteTaskConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := Run(cfg, store, RunRequest{
		Goal:        "Allow explicit review fanout for the same normalized domains",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	firstTask, firstDecision, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
		RunID:          run.RunID,
		TaskClass:      protocol.TaskClassReview,
		Domains:        []string{"session", "protocol"},
		Goal:           "Review the implementation routing outcome",
		ExpectedOutput: "A review artifact for the same work item",
		ReviewRequired: true,
	})
	if err != nil {
		t.Fatalf("first route child task: %v", err)
	}
	secondTask, secondDecision, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
		RunID:          run.RunID,
		TaskClass:      protocol.TaskClassReview,
		Domains:        []string{"protocol", "session"},
		Goal:           "Fan out review to a second reviewer pass",
		ExpectedOutput: "A second review artifact for the same work item",
		ReviewRequired: true,
	})
	if err != nil {
		t.Fatalf("second route child task: %v", err)
	}

	if firstDecision == nil || secondDecision == nil {
		t.Fatalf("expected routing decisions for review fanout")
	}
	if firstTask.TaskID == secondTask.TaskID {
		t.Fatalf("expected fanout review tasks to produce distinct task ids")
	}
}

func TestRouteChildTaskRequiresOverrideReason(t *testing.T) {
	t.Parallel()

	t.Run("missing override_reason fails fast", func(t *testing.T) {
		t.Parallel()

		cfg := testRouteTaskConfig(t)
		store := mailbox.NewStore(cfg.Session.StateDir)

		run, err := Run(cfg, store, RunRequest{
			Goal:        "Validate owner override guardrails",
			Coordinator: "pm",
			CreatedBy:   "human",
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}

		_, _, err = RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
			RunID:          run.RunID,
			TaskClass:      protocol.TaskClassImplementation,
			Domains:        []string{"session", "protocol"},
			OwnerOverride:  protocol.AgentName("backend-low"),
			Goal:           "Try to override routing without an explanation",
			ExpectedOutput: "override_reason should be required",
		})
		if err == nil {
			t.Fatalf("expected override without override_reason to fail")
		}
		if !strings.Contains(err.Error(), "override_reason") {
			t.Fatalf("expected override validation error to mention override_reason, got %v", err)
		}
	})

	t.Run("override cannot bypass duplicate blocking", func(t *testing.T) {
		t.Parallel()

		cfg := testRouteTaskConfig(t)
		store := mailbox.NewStore(cfg.Session.StateDir)

		run, err := Run(cfg, store, RunRequest{
			Goal:        "Validate override behavior against duplicate policy",
			Coordinator: "pm",
			CreatedBy:   "human",
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}

		firstTask, _, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
			RunID:          run.RunID,
			TaskClass:      protocol.TaskClassImplementation,
			Domains:        []string{"session", "protocol"},
			Goal:           "Create the first implementation task",
			ExpectedOutput: "One implementation task exists before the override attempt",
		})
		if err != nil {
			t.Fatalf("first route child task: %v", err)
		}

		wantDuplicateKey := string(run.RunID) + "|implementation|protocol,session"
		_, _, err = RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
			RunID:          run.RunID,
			TaskClass:      protocol.TaskClassImplementation,
			Domains:        []string{"protocol", "session"},
			OwnerOverride:  protocol.AgentName("backend-low"),
			OverrideReason: "manual reviewer pass",
			Goal:           "Try to bypass duplicate safeguards with an explicit override",
			ExpectedOutput: "Duplicate routes stay blocked even with override_reason",
		})
		if err == nil {
			t.Fatalf("expected duplicate route with override_reason to fail")
		}
		if !strings.Contains(err.Error(), "duplicate_key") || !strings.Contains(err.Error(), wantDuplicateKey) {
			t.Fatalf("expected duplicate override failure to mention duplicate_key %q, got %v", wantDuplicateKey, err)
		}
		if !strings.Contains(err.Error(), string(firstTask.TaskID)) {
			t.Fatalf("expected duplicate override failure to mention matched task id %q, got %v", firstTask.TaskID, err)
		}
	})
}

func TestAddChildTaskRejectsDuplicateWithoutRouteDecision(t *testing.T) {
	t.Parallel()

	cfg := testRouteTaskConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := Run(cfg, store, RunRequest{
		Goal:        "Verify explicit-owner persistence re-checks duplicate routing metadata",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	wantDuplicateKey := string(run.RunID) + "|implementation|protocol,session"
	firstTask, err := AddChildTask(cfg, store, ChildTaskRequest{
		ParentRunID:       run.RunID,
		Owner:             "backend-high",
		Goal:              "Persist the first routed implementation task",
		ExpectedOutput:    "One implementation task exists with duplicate routing metadata",
		TaskClass:         protocol.TaskClassImplementation,
		Domains:           []string{"session", "protocol"},
		NormalizedDomains: []string{"protocol", "session"},
		DuplicateKey:      wantDuplicateKey,
		RoutingDecision: protocol.RoutingDecision{
			Status:          "selected",
			SelectedOwner:   protocol.AgentName("backend-high"),
			Candidates:      []protocol.AgentName{"backend-high", "backend-low"},
			TieBreak:        "route_priority desc, config_order asc",
			DuplicateStatus: "unique",
		},
	})
	if err != nil {
		t.Fatalf("first add child task: %v", err)
	}

	_, err = AddChildTask(cfg, store, ChildTaskRequest{
		ParentRunID:       run.RunID,
		Owner:             "backend-low",
		Goal:              "Persist a duplicate implementation task through add-task",
		ExpectedOutput:    "AddChildTask must reject duplicates without route decision bypass",
		TaskClass:         protocol.TaskClassImplementation,
		Domains:           []string{"protocol", "session"},
		NormalizedDomains: []string{"protocol", "session"},
		DuplicateKey:      wantDuplicateKey,
		RoutingDecision: protocol.RoutingDecision{
			Status:          "selected",
			SelectedOwner:   protocol.AgentName("backend-low"),
			Candidates:      []protocol.AgentName{"backend-high", "backend-low"},
			TieBreak:        "route_priority desc, config_order asc",
			DuplicateStatus: "duplicate-blocked",
			MatchedTaskID:   firstTask.TaskID,
		},
	})
	if err == nil {
		t.Fatalf("expected add child task duplicate to fail with duplicate_key %q", wantDuplicateKey)
	}
	if !strings.Contains(err.Error(), "duplicate_key") || !strings.Contains(err.Error(), wantDuplicateKey) {
		t.Fatalf("expected add child task duplicate error to mention duplicate_key %q, got %v", wantDuplicateKey, err)
	}
	if !strings.Contains(err.Error(), string(firstTask.TaskID)) {
		t.Fatalf("expected add child task duplicate error to mention matched task id %q, got %v", firstTask.TaskID, err)
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

func TestRunCreatesCoordinatorArtifactsAndRootMessage(t *testing.T) {
	t.Parallel()

	cfg := testRunWorkflowConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := Run(cfg, store, RunRequest{
		Goal:        "Break the coordinator workflow into bounded child tasks",
		Coordinator: "lead",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if run.RunID == "" {
		t.Fatalf("expected run id to be populated")
	}
	if run.RootMessageID == "" {
		t.Fatalf("expected root message id to be populated")
	}
	if run.RootThreadID == "" {
		t.Fatalf("expected root thread id to be populated")
	}

	wantAllowedOwners := []protocol.AgentName{"backend", "reviewer"}
	if !reflect.DeepEqual(run.AllowedOwners, wantAllowedOwners) {
		t.Fatalf("unexpected allowed owners: got %#v want %#v", run.AllowedOwners, wantAllowedOwners)
	}

	wantSnapshot := []protocol.AgentSnapshot{
		{
			Name:      "pm",
			Alias:     "lead",
			Role:      "research",
			Teammates: []string{"backend", "reviewer", "norole"},
		},
		{
			Name:      "backend",
			Alias:     "dev",
			Role:      "implementation",
			Teammates: []string{"pm", "reviewer"},
		},
		{
			Name:      "reviewer",
			Alias:     "qa",
			Role:      "review",
			Teammates: []string{"pm", "backend"},
		},
	}
	if !reflect.DeepEqual(run.TeamSnapshot, wantSnapshot) {
		t.Fatalf("unexpected team snapshot: got %#v want %#v", run.TeamSnapshot, wantSnapshot)
	}

	runBytes, err := os.ReadFile(mailbox.RunFilePath(cfg.Session.StateDir, run.RunID))
	if err != nil {
		t.Fatalf("read run file: %v", err)
	}

	var persisted protocol.CoordinatorRun
	if err := yaml.Unmarshal(runBytes, &persisted); err != nil {
		t.Fatalf("unmarshal run file: %v", err)
	}
	if !reflect.DeepEqual(persisted.AllowedOwners, wantAllowedOwners) {
		t.Fatalf("unexpected persisted allowed owners: got %#v want %#v", persisted.AllowedOwners, wantAllowedOwners)
	}
	if !reflect.DeepEqual(persisted.TeamSnapshot, wantSnapshot) {
		t.Fatalf("unexpected persisted team snapshot: got %#v want %#v", persisted.TeamSnapshot, wantSnapshot)
	}

	env, body, err := store.ReadMessage(run.RootMessageID)
	if err != nil {
		t.Fatalf("read root message: %v", err)
	}
	if !strings.HasPrefix(env.Subject, "Coordinator run ") {
		t.Fatalf("expected root subject to start with %q, got %q", "Coordinator run ", env.Subject)
	}
	if env.Thread != run.RootThreadID {
		t.Fatalf("unexpected root thread id: got %q want %q", env.Thread, run.RootThreadID)
	}
	if !reflect.DeepEqual(env.To, []protocol.AgentName{"pm"}) {
		t.Fatalf("unexpected recipients: got %#v want %#v", env.To, []protocol.AgentName{"pm"})
	}

	receipt, err := store.ReadReceipt("pm", run.RootMessageID)
	if err != nil {
		t.Fatalf("read root receipt: %v", err)
	}
	if receipt.FolderState != protocol.FolderStateUnread {
		t.Fatalf("expected root receipt to be unread, got %q", receipt.FolderState)
	}

	bodyText := string(body)
	requiredSnippets := []string{
		"## Decomposition Instructions",
		"## Run References",
		"tmuxicate run route-task --run " + string(run.RunID),
		"--task-class <class>",
		"--domain <domain>",
		"run_id: " + string(run.RunID),
		"root_message_id: " + string(run.RootMessageID),
		"root_thread_id: " + string(run.RootThreadID),
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(bodyText, snippet) {
			t.Fatalf("expected root body to contain %q\nbody:\n%s", snippet, bodyText)
		}
	}
}

func TestAddChildTaskPersistsSchemaAndEmitsMailboxTask(t *testing.T) {
	t.Parallel()

	cfg := testRunWorkflowConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := Run(cfg, store, RunRequest{
		Goal:        "Break the coordinator workflow into bounded child tasks",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	task, err := AddChildTask(cfg, store, ChildTaskRequest{
		ParentRunID:    run.RunID,
		Owner:          "backend",
		Goal:           "Implement canonical run and task artifact writers",
		ExpectedOutput: "run.yaml and <task-id>.yaml files are persisted under coordinator/runs",
		DependsOn:      []protocol.TaskID{"task_000000000111"},
		ReviewRequired: true,
	})
	if err != nil {
		t.Fatalf("add child task: %v", err)
	}

	if task.TaskID == "" {
		t.Fatalf("expected task id to be populated")
	}
	if task.MessageID == "" {
		t.Fatalf("expected task message id to be populated")
	}
	if task.ThreadID == "" {
		t.Fatalf("expected task thread id to be populated")
	}

	taskBytes, err := os.ReadFile(mailbox.RunTaskPath(cfg.Session.StateDir, run.RunID, task.TaskID))
	if err != nil {
		t.Fatalf("read task file: %v", err)
	}

	var persisted protocol.ChildTask
	if err := yaml.Unmarshal(taskBytes, &persisted); err != nil {
		t.Fatalf("unmarshal task file: %v", err)
	}

	if persisted.ParentRunID != run.RunID {
		t.Fatalf("unexpected parent_run_id: got %q want %q", persisted.ParentRunID, run.RunID)
	}
	if persisted.Owner != "backend" {
		t.Fatalf("unexpected owner: got %q want %q", persisted.Owner, "backend")
	}
	if persisted.Goal != "Implement canonical run and task artifact writers" {
		t.Fatalf("unexpected goal: got %q", persisted.Goal)
	}
	if persisted.ExpectedOutput != "run.yaml and <task-id>.yaml files are persisted under coordinator/runs" {
		t.Fatalf("unexpected expected_output: got %q", persisted.ExpectedOutput)
	}
	if !reflect.DeepEqual(persisted.DependsOn, []protocol.TaskID{"task_000000000111"}) {
		t.Fatalf("unexpected depends_on: got %#v", persisted.DependsOn)
	}
	if !persisted.ReviewRequired {
		t.Fatalf("expected review_required to be true")
	}

	env, body, err := store.ReadMessage(task.MessageID)
	if err != nil {
		t.Fatalf("read task message: %v", err)
	}
	if env.Meta["run_id"] != string(run.RunID) {
		t.Fatalf("expected run_id metadata %q, got %#v", run.RunID, env.Meta)
	}
	if env.Meta["task_id"] != string(task.TaskID) {
		t.Fatalf("expected task_id metadata %q, got %#v", task.TaskID, env.Meta)
	}
	if env.Meta["parent_run_id"] != string(run.RunID) {
		t.Fatalf("expected parent_run_id metadata %q, got %#v", run.RunID, env.Meta)
	}
	if env.Meta["expected_output"] != task.ExpectedOutput {
		t.Fatalf("expected expected_output metadata %q, got %#v", task.ExpectedOutput, env.Meta)
	}

	bodyText := string(body)
	requiredSnippets := []string{
		"# Task",
		"## Goal",
		"## Expected Output",
		"## Dependencies",
		"## Run References",
		"tmuxicate reply",
		"raw pane text",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(bodyText, snippet) {
			t.Fatalf("expected child task body to contain %q\nbody:\n%s", snippet, bodyText)
		}
	}

	receipt, err := store.ReadReceipt("backend", task.MessageID)
	if err != nil {
		t.Fatalf("read owner receipt: %v", err)
	}
	if receipt.FolderState != protocol.FolderStateUnread {
		t.Fatalf("expected owner receipt to be unread, got %q", receipt.FolderState)
	}

	receiptPaths, err := filepath.Glob(mailbox.ReceiptGlob(cfg.Session.StateDir, "backend", task.MessageID))
	if err != nil {
		t.Fatalf("glob owner receipts: %v", err)
	}
	if len(receiptPaths) != 1 {
		t.Fatalf("expected exactly one owner receipt, got %d (%v)", len(receiptPaths), receiptPaths)
	}
}

func TestAddChildTaskRejectsOwnerOutsideRoutingBaseline(t *testing.T) {
	t.Parallel()

	cfg := testRunWorkflowConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := Run(cfg, store, RunRequest{
		Goal:        "Break the coordinator workflow into bounded child tasks",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	testCases := []struct {
		name    string
		owner   string
		wantErr string
	}{
		{
			name:    "owner outside teammate graph",
			owner:   "outsider",
			wantErr: "allowed owner",
		},
		{
			name:    "owner missing role metadata",
			owner:   "norole",
			wantErr: "role",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := AddChildTask(cfg, store, ChildTaskRequest{
				ParentRunID:    run.RunID,
				Owner:          tc.owner,
				Goal:           "Implement bounded work item",
				ExpectedOutput: "One durable child task artifact and one mailbox task",
			})
			if err == nil {
				t.Fatalf("expected add child task to reject owner %q", tc.owner)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error to contain %q, got %v", tc.wantErr, err)
			}
		})
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
				{Name: "pm", Alias: "lead", Role: config.RoleSpec{Kind: string(protocol.TaskClassResearch), Domains: []string{"routing"}, Description: "Coordinates routing and research work"}, Teammates: []string{"builder", "qa"}},
				{Name: "builder", Alias: "dev", RoutePriority: 20, Role: config.RoleSpec{Kind: string(protocol.TaskClassImplementation), Domains: []string{"session", "protocol"}, Description: "Owns session and protocol implementation"}, Teammates: []string{"pm", "qa"}},
				{Name: "qa", Alias: "tester", RoutePriority: 10, Role: config.RoleSpec{Kind: string(protocol.TaskClassReview), Domains: []string{"session"}, Description: "Owns review work"}, Teammates: []string{"pm", "builder"}},
			},
		},
	}
}

func testRunWorkflowConfig(t *testing.T) *config.ResolvedConfig {
	t.Helper()

	stateDir := t.TempDir()

	return &config.ResolvedConfig{
		Config: config.Config{
			Session: config.SessionConfig{
				Name:      "coord-foundation",
				Workspace: filepath.Join(stateDir, "workspace"),
				StateDir:  stateDir,
			},
			Agents: []config.AgentConfig{
				{Name: "pm", Alias: "lead", Role: config.RoleSpec{Kind: string(protocol.TaskClassResearch), Domains: []string{"routing"}, Description: "Coordinates routing and research work"}, Teammates: []string{"backend", "reviewer", "norole"}},
				{Name: "backend", Alias: "dev", RoutePriority: 20, Role: config.RoleSpec{Kind: string(protocol.TaskClassImplementation), Domains: []string{"session", "protocol"}, Description: "Owns session and protocol implementation"}, Teammates: []string{"pm", "reviewer"}},
				{Name: "reviewer", Alias: "qa", RoutePriority: 10, Role: config.RoleSpec{Kind: string(protocol.TaskClassReview), Domains: []string{"session"}, Description: "Owns review work"}, Teammates: []string{"pm", "backend"}},
				{Name: "norole", Alias: "ghost", Role: config.RoleSpec{}, Teammates: []string{"pm"}},
				{Name: "outsider", Alias: "ops", Role: config.RoleSpec{Kind: string(protocol.TaskClassResearch), Domains: []string{"operations"}, Description: "Owns operations research"}, Teammates: []string{"reviewer"}},
			},
		},
	}
}

func testRouteTaskConfig(t *testing.T) *config.ResolvedConfig {
	t.Helper()

	stateDir := t.TempDir()

	return &config.ResolvedConfig{
		Config: config.Config{
			Session: config.SessionConfig{
				Name:      "role-routing",
				Workspace: filepath.Join(stateDir, "workspace"),
				StateDir:  stateDir,
			},
			Routing: config.RoutingConfig{
				Coordinator:          "pm",
				ExclusiveTaskClasses: []protocol.TaskClass{protocol.TaskClassImplementation},
				// Mirrors tmuxicate.yaml routing.fanout_task_classes for explicit review fanout.
				FanoutTaskClasses: []protocol.TaskClass{protocol.TaskClassReview},
			},
			Agents: []config.AgentConfig{
				{Name: "pm", Alias: "lead", Role: config.RoleSpec{Kind: string(protocol.TaskClassResearch), Domains: []string{"routing"}, Description: "Coordinates routing and research work"}, Teammates: []string{"backend-high", "backend-low", "backend-later", "reviewer"}},
				{Name: "backend-high", Alias: "api-high", RoutePriority: 20, Role: config.RoleSpec{Kind: string(protocol.TaskClassImplementation), Domains: []string{"session", "protocol"}, Description: "Owns session and protocol implementation"}, Teammates: []string{"pm", "reviewer"}},
				{Name: "backend-low", Alias: "api-low", RoutePriority: 10, Role: config.RoleSpec{Kind: string(protocol.TaskClassImplementation), Domains: []string{"session", "protocol"}, Description: "Owns secondary implementation work"}, Teammates: []string{"pm", "reviewer"}},
				{Name: "backend-later", Alias: "api-later", RoutePriority: 20, Role: config.RoleSpec{Kind: string(protocol.TaskClassImplementation), Domains: []string{"session", "protocol"}, Description: "Owns later-declared implementation work"}, Teammates: []string{"pm", "reviewer"}},
				{Name: "reviewer", Alias: "qa", RoutePriority: 5, Role: config.RoleSpec{Kind: string(protocol.TaskClassReview), Domains: []string{"session", "protocol"}, Description: "Owns review work"}, Teammates: []string{"pm", "backend-high", "backend-low", "backend-later"}},
			},
		},
	}
}
