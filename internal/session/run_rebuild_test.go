package session

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"gopkg.in/yaml.v3"
)

func TestRebuildRunGraphFromDisk(t *testing.T) {
	t.Parallel()

	fixture := seedRunGraphFixture(t)

	graph, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	if graph.Run.RunID != fixture.run.RunID {
		t.Fatalf("unexpected run id: got %q want %q", graph.Run.RunID, fixture.run.RunID)
	}
	if graph.Run.RootMessageID != fixture.run.RootMessageID {
		t.Fatalf("unexpected root message id: got %q want %q", graph.Run.RootMessageID, fixture.run.RootMessageID)
	}
	if len(graph.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(graph.Tasks))
	}

	tasksByID := map[protocol.TaskID]RunGraphTask{}
	for _, task := range graph.Tasks {
		tasksByID[task.Task.TaskID] = task
	}

	backend := tasksByID[fixture.backendTask.TaskID]
	if backend.Task.Owner != fixture.backendTask.Owner {
		t.Fatalf("unexpected backend owner: got %q want %q", backend.Task.Owner, fixture.backendTask.Owner)
	}
	if backend.Task.MessageID != fixture.backendTask.MessageID {
		t.Fatalf("unexpected backend message id: got %q want %q", backend.Task.MessageID, fixture.backendTask.MessageID)
	}
	if backend.ReceiptState != protocol.FolderStateDone {
		t.Fatalf("unexpected backend receipt state: got %q want %q", backend.ReceiptState, protocol.FolderStateDone)
	}
	if backend.DeclaredState != "idle" {
		t.Fatalf("unexpected backend declared state: got %q want %q", backend.DeclaredState, "idle")
	}

	reviewer := tasksByID[fixture.reviewerTask.TaskID]
	if reviewer.Task.Owner != fixture.reviewerTask.Owner {
		t.Fatalf("unexpected reviewer owner: got %q want %q", reviewer.Task.Owner, fixture.reviewerTask.Owner)
	}
	if !reflect.DeepEqual(reviewer.Task.DependsOn, []protocol.TaskID{fixture.backendTask.TaskID}) {
		t.Fatalf("unexpected reviewer dependencies: got %#v want %#v", reviewer.Task.DependsOn, []protocol.TaskID{fixture.backendTask.TaskID})
	}
	if reviewer.Task.MessageID != fixture.reviewerTask.MessageID {
		t.Fatalf("unexpected reviewer message id: got %q want %q", reviewer.Task.MessageID, fixture.reviewerTask.MessageID)
	}
	if reviewer.ReceiptState != protocol.FolderStateActive {
		t.Fatalf("unexpected reviewer receipt state: got %q want %q", reviewer.ReceiptState, protocol.FolderStateActive)
	}
	if reviewer.DeclaredState != "blocked" {
		t.Fatalf("unexpected reviewer declared state: got %q want %q", reviewer.DeclaredState, "blocked")
	}
}

func TestRunShowSummarizesReceiptAndDeclaredState(t *testing.T) {
	t.Parallel()

	fixture := seedRunGraphFixture(t)

	graph, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	output := FormatRunGraph(graph)

	requiredSnippets := []string{
		"Run: " + string(fixture.run.RunID),
		"Task: " + string(fixture.backendTask.TaskID),
		"Task: " + string(fixture.reviewerTask.TaskID),
		"Owner: " + string(fixture.backendTask.Owner),
		"Owner: " + string(fixture.reviewerTask.Owner),
		"Goal: " + fixture.backendTask.Goal,
		"Expected Output: " + fixture.backendTask.ExpectedOutput,
		"Depends On: " + string(fixture.backendTask.TaskID),
		"State: blocked [active]",
		"Message: " + string(fixture.reviewerTask.MessageID),
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected formatted run graph to contain %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestRunShowRejectsMissingOrMismatchedArtifacts(t *testing.T) {
	t.Parallel()

	t.Run("missing task yaml", func(t *testing.T) {
		t.Parallel()

		fixture := seedRunGraphFixture(t)
		if err := os.Remove(mailbox.RunTaskPath(fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.backendTask.TaskID)); err != nil {
			t.Fatalf("remove backend task yaml: %v", err)
		}

		_, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
		if err == nil {
			t.Fatalf("expected missing task yaml to fail")
		}
		if !strings.Contains(err.Error(), "coordinator artifact mismatch") {
			t.Fatalf("expected mismatch error, got %v", err)
		}
	})

	t.Run("unknown dependency id", func(t *testing.T) {
		t.Parallel()

		fixture := seedRunGraphFixture(t)
		mutateChildTask(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.reviewerTask.TaskID, func(task *protocol.ChildTask) {
			task.DependsOn = []protocol.TaskID{"task_999999999999"}
		})

		_, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
		if err == nil {
			t.Fatalf("expected unknown dependency id to fail")
		}
		if !strings.Contains(err.Error(), "coordinator artifact mismatch") {
			t.Fatalf("expected mismatch error, got %v", err)
		}
	})

	t.Run("message link mismatch", func(t *testing.T) {
		t.Parallel()

		fixture := seedRunGraphFixture(t)
		mutateChildTask(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.reviewerTask.TaskID, func(task *protocol.ChildTask) {
			task.MessageID = protocol.MessageID("msg_999999999999")
		})

		_, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
		if err == nil {
			t.Fatalf("expected message link mismatch to fail")
		}
		if !strings.Contains(err.Error(), "coordinator artifact mismatch") {
			t.Fatalf("expected mismatch error, got %v", err)
		}
	})
}

type runGraphFixture struct {
	cfg          *config.ResolvedConfig
	run          *protocol.CoordinatorRun
	backendTask  *protocol.ChildTask
	reviewerTask *protocol.ChildTask
}

func seedRunGraphFixture(t *testing.T) runGraphFixture {
	t.Helper()

	cfg := testRunWorkflowConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := Run(cfg, store, RunRequest{
		Goal:        "Rebuild coordinator state from durable artifacts",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	backendTask, err := AddChildTask(cfg, store, ChildTaskRequest{
		ParentRunID:    run.RunID,
		Owner:          "backend",
		Goal:           "Implement the rebuild reader",
		ExpectedOutput: "LoadRunGraph returns task lineage from disk",
	})
	if err != nil {
		t.Fatalf("add backend task: %v", err)
	}

	reviewerTask, err := AddChildTask(cfg, store, ChildTaskRequest{
		ParentRunID:    run.RunID,
		Owner:          "reviewer",
		Goal:           "Review rebuilt task lineage",
		ExpectedOutput: "run show exposes task ownership and mailbox references",
		DependsOn:      []protocol.TaskID{backendTask.TaskID},
		ReviewRequired: true,
	})
	if err != nil {
		t.Fatalf("add reviewer task: %v", err)
	}

	markReceiptState(t, store, "backend", backendTask.MessageID, protocol.FolderStateUnread, protocol.FolderStateDone)
	writeTaskState(t, cfg.Session.StateDir, "backend", backendTask.MessageID, backendTask.ThreadID, protocol.FolderStateDone, "idle")

	markReceiptState(t, store, "reviewer", reviewerTask.MessageID, protocol.FolderStateUnread, protocol.FolderStateActive)
	writeTaskState(t, cfg.Session.StateDir, "reviewer", reviewerTask.MessageID, reviewerTask.ThreadID, protocol.FolderStateActive, "blocked")

	return runGraphFixture{
		cfg:          cfg,
		run:          run,
		backendTask:  backendTask,
		reviewerTask: reviewerTask,
	}
}

func markReceiptState(t *testing.T, store *mailbox.Store, agent string, msgID protocol.MessageID, from, to protocol.FolderState) {
	t.Helper()

	if to == protocol.FolderStateDone {
		if err := store.MoveReceipt(agent, msgID, from, protocol.FolderStateActive); err != nil {
			t.Fatalf("move receipt to active before done: %v", err)
		}
		doneAt := time.Date(2026, time.April, 5, 6, 5, 0, 0, time.UTC)
		if err := store.UpdateReceipt(agent, msgID, func(receipt *protocol.Receipt) {
			receipt.DoneAt = &doneAt
			receipt.Revision++
		}); err != nil {
			t.Fatalf("update receipt before done move: %v", err)
		}
		from = protocol.FolderStateActive
	}
	if err := store.MoveReceipt(agent, msgID, from, to); err != nil {
		t.Fatalf("move receipt: %v", err)
	}
}

func writeTaskState(t *testing.T, stateDir, agent string, msgID protocol.MessageID, threadID protocol.ThreadID, receiptState protocol.FolderState, declaredState string) {
	t.Helper()

	if err := appendStateEvent(stateDir, agent, &TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     "2026-04-05T06:00:00Z",
		Agent:         agent,
		Event:         "task.update",
		DeclaredState: declaredState,
		MessageID:     msgID,
		Thread:        threadID,
		ReceiptState:  receiptState,
	}); err != nil {
		t.Fatalf("append state event: %v", err)
	}
}

func mutateChildTask(t *testing.T, stateDir string, runID protocol.RunID, taskID protocol.TaskID, mutate func(task *protocol.ChildTask)) {
	t.Helper()

	path := mailbox.RunTaskPath(stateDir, runID, taskID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read task yaml: %v", err)
	}

	var task protocol.ChildTask
	if err := yaml.Unmarshal(data, &task); err != nil {
		t.Fatalf("unmarshal task yaml: %v", err)
	}

	mutate(&task)

	updated, err := yaml.Marshal(&task)
	if err != nil {
		t.Fatalf("marshal task yaml: %v", err)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatalf("write task yaml: %v", err)
	}
}
