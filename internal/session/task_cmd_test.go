package session

import (
	"os"
	"strings"
	"testing"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestTaskDoneCreatesReviewHandoffAndRoutesReview(t *testing.T) {
	t.Parallel()

	fixture := seedReviewHandoffFixture(t)

	if _, err := ReadMsg(fixture.cfg.Session.StateDir, "backend-high", fixture.sourceTask.MessageID); err != nil {
		t.Fatalf("activate source task: %v", err)
	}

	if err := TaskDone(fixture.cfg.Session.StateDir, "backend-high", fixture.sourceTask.MessageID, "implementation complete"); err != nil {
		t.Fatalf("task done: %v", err)
	}

	store := mailbox.NewStore(fixture.cfg.Session.StateDir)
	receipt, err := store.ReadReceipt("backend-high", fixture.sourceTask.MessageID)
	if err != nil {
		t.Fatalf("read source receipt: %v", err)
	}
	if receipt.FolderState != protocol.FolderStateDone {
		t.Fatalf("source receipt state = %q, want %q", receipt.FolderState, protocol.FolderStateDone)
	}

	coordinatorStore := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir)
	handoff, err := coordinatorStore.ReadReviewHandoff(fixture.run.RunID, fixture.sourceTask.TaskID)
	if err != nil {
		t.Fatalf("read review handoff: %v", err)
	}
	if handoff.SourceTaskID != fixture.sourceTask.TaskID {
		t.Fatalf("source task id = %q, want %q", handoff.SourceTaskID, fixture.sourceTask.TaskID)
	}
	if handoff.SourceMessageID != fixture.sourceTask.MessageID {
		t.Fatalf("source message id = %q, want %q", handoff.SourceMessageID, fixture.sourceTask.MessageID)
	}
	if handoff.Status != protocol.ReviewHandoffStatusPending {
		t.Fatalf("handoff status = %q, want %q", handoff.Status, protocol.ReviewHandoffStatusPending)
	}
	if handoff.Reviewer != "reviewer" {
		t.Fatalf("reviewer = %q, want %q", handoff.Reviewer, "reviewer")
	}

	reviewTask, err := coordinatorStore.ReadTask(fixture.run.RunID, handoff.ReviewTaskID)
	if err != nil {
		t.Fatalf("read review task: %v", err)
	}
	if reviewTask.TaskClass != protocol.TaskClassReview {
		t.Fatalf("review task class = %q, want %q", reviewTask.TaskClass, protocol.TaskClassReview)
	}
	if reviewTask.ReviewRequired {
		t.Fatalf("review task should not require follow-up review")
	}
	if reviewTask.MessageID != handoff.ReviewMessageID {
		t.Fatalf("review message id = %q, want %q", reviewTask.MessageID, handoff.ReviewMessageID)
	}

	env, _, err := store.ReadMessage(handoff.ReviewMessageID)
	if err != nil {
		t.Fatalf("read review message: %v", err)
	}
	if env.Kind != protocol.KindReviewRequest {
		t.Fatalf("review message kind = %q, want %q", env.Kind, protocol.KindReviewRequest)
	}
	if env.Meta["parent_run_id"] != string(fixture.run.RunID) {
		t.Fatalf("review message parent_run_id = %q, want %q", env.Meta["parent_run_id"], fixture.run.RunID)
	}
	if env.Meta["task_id"] != string(handoff.ReviewTaskID) {
		t.Fatalf("review message task_id = %q, want %q", env.Meta["task_id"], handoff.ReviewTaskID)
	}

	reviewReceipt, err := store.ReadReceipt("reviewer", handoff.ReviewMessageID)
	if err != nil {
		t.Fatalf("read review receipt: %v", err)
	}
	if reviewReceipt.FolderState != protocol.FolderStateUnread {
		t.Fatalf("review receipt state = %q, want %q", reviewReceipt.FolderState, protocol.FolderStateUnread)
	}

	entries, err := os.ReadDir(mailbox.RunReviewsDir(fixture.cfg.Session.StateDir, fixture.run.RunID))
	if err != nil {
		t.Fatalf("read reviews dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("review handoff count = %d, want 1", len(entries))
	}
}

func TestTaskDoneReviewHandoffIsIdempotent(t *testing.T) {
	t.Parallel()

	fixture := seedReviewHandoffFixture(t)

	if _, err := ReadMsg(fixture.cfg.Session.StateDir, "backend-high", fixture.sourceTask.MessageID); err != nil {
		t.Fatalf("activate source task: %v", err)
	}

	if err := TaskDone(fixture.cfg.Session.StateDir, "backend-high", fixture.sourceTask.MessageID, "implementation complete"); err != nil {
		t.Fatalf("first task done: %v", err)
	}

	coordinatorStore := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir)
	firstHandoff, err := coordinatorStore.ReadReviewHandoff(fixture.run.RunID, fixture.sourceTask.TaskID)
	if err != nil {
		t.Fatalf("read first handoff: %v", err)
	}
	taskCountBefore := countRunTaskDocs(t, fixture.cfg.Session.StateDir, fixture.run.RunID)
	messageCountBefore := countMessageDirs(t, fixture.cfg.Session.StateDir)

	store := mailbox.NewStore(fixture.cfg.Session.StateDir)
	if err := store.MoveReceipt("backend-high", fixture.sourceTask.MessageID, protocol.FolderStateDone, protocol.FolderStateActive); err != nil {
		t.Fatalf("restore source receipt to active: %v", err)
	}

	if err := TaskDone(fixture.cfg.Session.StateDir, "backend-high", fixture.sourceTask.MessageID, "implementation complete again"); err != nil {
		t.Fatalf("second task done: %v", err)
	}

	secondHandoff, err := coordinatorStore.ReadReviewHandoff(fixture.run.RunID, fixture.sourceTask.TaskID)
	if err != nil {
		t.Fatalf("read second handoff: %v", err)
	}
	if secondHandoff.ReviewTaskID != firstHandoff.ReviewTaskID {
		t.Fatalf("review task id changed: got %q want %q", secondHandoff.ReviewTaskID, firstHandoff.ReviewTaskID)
	}
	if secondHandoff.ReviewMessageID != firstHandoff.ReviewMessageID {
		t.Fatalf("review message id changed: got %q want %q", secondHandoff.ReviewMessageID, firstHandoff.ReviewMessageID)
	}

	if got := countRunTaskDocs(t, fixture.cfg.Session.StateDir, fixture.run.RunID); got != taskCountBefore {
		t.Fatalf("task doc count = %d, want %d", got, taskCountBefore)
	}
	if got := countMessageDirs(t, fixture.cfg.Session.StateDir); got != messageCountBefore {
		t.Fatalf("message dir count = %d, want %d", got, messageCountBefore)
	}
	entries, err := os.ReadDir(mailbox.RunReviewsDir(fixture.cfg.Session.StateDir, fixture.run.RunID))
	if err != nil {
		t.Fatalf("read reviews dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("review handoff count = %d, want 1", len(entries))
	}
}

func TestTaskDoneRecordsReviewHandoffFailureWithoutRollback(t *testing.T) {
	t.Parallel()

	fixture := seedReviewHandoffFixture(t)

	mutateChildTaskDocument(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask.TaskID, func(taskDoc map[string]any) {
		delete(taskDoc, "normalized_domains")
	})

	if _, err := ReadMsg(fixture.cfg.Session.StateDir, "backend-high", fixture.sourceTask.MessageID); err != nil {
		t.Fatalf("activate source task: %v", err)
	}

	if err := TaskDone(fixture.cfg.Session.StateDir, "backend-high", fixture.sourceTask.MessageID, "implementation complete"); err != nil {
		t.Fatalf("task done: %v", err)
	}

	store := mailbox.NewStore(fixture.cfg.Session.StateDir)
	receipt, err := store.ReadReceipt("backend-high", fixture.sourceTask.MessageID)
	if err != nil {
		t.Fatalf("read source receipt: %v", err)
	}
	if receipt.FolderState != protocol.FolderStateDone {
		t.Fatalf("source receipt state = %q, want %q", receipt.FolderState, protocol.FolderStateDone)
	}

	coordinatorStore := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir)
	handoff, err := coordinatorStore.ReadReviewHandoff(fixture.run.RunID, fixture.sourceTask.TaskID)
	if err != nil {
		t.Fatalf("read review handoff: %v", err)
	}
	if handoff.Status != protocol.ReviewHandoffStatusHandoffFailed {
		t.Fatalf("handoff status = %q, want %q", handoff.Status, protocol.ReviewHandoffStatusHandoffFailed)
	}
	if !strings.Contains(handoff.FailureSummary, "missing normalized_domains") {
		t.Fatalf("failure summary = %q, want normalized_domains explanation", handoff.FailureSummary)
	}
	if handoff.ReviewTaskID != "" {
		t.Fatalf("review task id = %q, want empty", handoff.ReviewTaskID)
	}
	if handoff.ReviewMessageID != "" {
		t.Fatalf("review message id = %q, want empty", handoff.ReviewMessageID)
	}
	if handoff.Reviewer != "" {
		t.Fatalf("reviewer = %q, want empty", handoff.Reviewer)
	}
	if got := countRunTaskDocs(t, fixture.cfg.Session.StateDir, fixture.run.RunID); got != 1 {
		t.Fatalf("task doc count = %d, want 1", got)
	}
	if got := countMessageDirs(t, fixture.cfg.Session.StateDir); got != 2 {
		t.Fatalf("message dir count = %d, want 2", got)
	}
}

type reviewHandoffFixture struct {
	cfg        *config.ResolvedConfig
	run        *protocol.CoordinatorRun
	sourceTask *protocol.ChildTask
}

func seedReviewHandoffFixture(t *testing.T) reviewHandoffFixture {
	t.Helper()

	cfg := testRouteTaskConfig(t)
	makeConfigLoadable(cfg)
	if err := createStateTree(cfg); err != nil {
		t.Fatalf("create state tree: %v", err)
	}
	if err := writeResolvedConfig(cfg); err != nil {
		t.Fatalf("write resolved config: %v", err)
	}

	store := mailbox.NewStore(cfg.Session.StateDir)
	run, err := Run(cfg, store, RunRequest{
		Goal:        "Route implementation work into review handoff flow",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	sourceTask, _, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
		RunID:          run.RunID,
		TaskClass:      protocol.TaskClassImplementation,
		Domains:        []string{"session", "protocol"},
		Goal:           "Implement review handoff flow",
		ExpectedOutput: "A routed implementation task that requires review",
		ReviewRequired: true,
	})
	if err != nil {
		t.Fatalf("route child task: %v", err)
	}

	return reviewHandoffFixture{
		cfg:        cfg,
		run:        run,
		sourceTask: sourceTask,
	}
}

func makeConfigLoadable(cfg *config.ResolvedConfig) {
	cfg.Version = 1
	for i := range cfg.Agents {
		cfg.Agents[i].Adapter = "generic"
		cfg.Agents[i].Command = "fake-agent"
		cfg.Agents[i].Pane.Slot = cfg.Agents[i].Name
	}
}

func countRunTaskDocs(t *testing.T, stateDir string, runID protocol.RunID) int {
	t.Helper()

	entries, err := os.ReadDir(mailbox.RunTasksDir(stateDir, runID))
	if err != nil {
		t.Fatalf("read task dir: %v", err)
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			count++
		}
	}

	return count
}

func countMessageDirs(t *testing.T, stateDir string) int {
	t.Helper()

	entries, err := os.ReadDir(mailbox.MessagesDir(stateDir))
	if err != nil {
		t.Fatalf("read messages dir: %v", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "msg_") {
			count++
		}
	}

	return count
}
