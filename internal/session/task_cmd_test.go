package session

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestTaskWaitCreatesWatchBlockerCase(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		waitKind protocol.WaitKind
		reason   string
	}{
		{
			name:     "dependency reply",
			waitKind: protocol.WaitKindDependencyReply,
			reason:   "waiting for dependency reply",
		},
		{
			name:     "external event",
			waitKind: protocol.WaitKindExternalEvent,
			reason:   "waiting for external event",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := seedBlockerPolicyFixture(t, 1)

			if err := callTaskWaitForPolicy(fixture.cfg.Session.StateDir, string(fixture.sourceTask.Owner), fixture.sourceTask.MessageID, tc.waitKind, tc.reason); err != nil {
				t.Fatalf("task wait: %v", err)
			}

			blockerCase := mustReadBlockerCase(t, fixture)
			assertFileExists(t, mailbox.RunBlockerCasePath(fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask.TaskID))

			if blockerCase.SourceTaskID != fixture.sourceTask.TaskID {
				t.Fatalf("source task id = %q, want %q", blockerCase.SourceTaskID, fixture.sourceTask.TaskID)
			}
			if blockerCase.SourceMessageID != fixture.sourceTask.MessageID {
				t.Fatalf("source message id = %q, want %q", blockerCase.SourceMessageID, fixture.sourceTask.MessageID)
			}
			if blockerCase.SourceOwner != fixture.sourceTask.Owner {
				t.Fatalf("source owner = %q, want %q", blockerCase.SourceOwner, fixture.sourceTask.Owner)
			}
			if blockerCase.CurrentTaskID != fixture.sourceTask.TaskID {
				t.Fatalf("current task id = %q, want %q", blockerCase.CurrentTaskID, fixture.sourceTask.TaskID)
			}
			if blockerCase.CurrentMessageID != fixture.sourceTask.MessageID {
				t.Fatalf("current message id = %q, want %q", blockerCase.CurrentMessageID, fixture.sourceTask.MessageID)
			}
			if blockerCase.CurrentOwner != fixture.sourceTask.Owner {
				t.Fatalf("current owner = %q, want %q", blockerCase.CurrentOwner, fixture.sourceTask.Owner)
			}
			if blockerCase.DeclaredState != "wait" {
				t.Fatalf("declared state = %q, want %q", blockerCase.DeclaredState, "wait")
			}
			if blockerCase.WaitKind != tc.waitKind {
				t.Fatalf("wait kind = %q, want %q", blockerCase.WaitKind, tc.waitKind)
			}
			if blockerCase.SelectedAction != protocol.BlockerActionWatch {
				t.Fatalf("selected action = %q, want %q", blockerCase.SelectedAction, protocol.BlockerActionWatch)
			}
			if blockerCase.Status != protocol.BlockerStatusActive {
				t.Fatalf("status = %q, want %q", blockerCase.Status, protocol.BlockerStatusActive)
			}
			if blockerCase.RerouteCount != 0 {
				t.Fatalf("reroute count = %d, want 0", blockerCase.RerouteCount)
			}
			if blockerCase.MaxReroutes != 1 {
				t.Fatalf("max reroutes = %d, want 1", blockerCase.MaxReroutes)
			}
			if blockerCase.RecommendedAction != nil {
				t.Fatalf("recommended action = %#v, want nil", blockerCase.RecommendedAction)
			}
		})
	}
}

func TestTaskBlockReroutesWithinCeiling(t *testing.T) {
	t.Parallel()

	fixture := seedBlockerPolicyFixture(t, 1)
	taskCountBefore := countRunTaskDocs(t, fixture.cfg.Session.StateDir, fixture.run.RunID)

	if err := callTaskBlockForPolicy(
		fixture.cfg.Session.StateDir,
		string(fixture.sourceTask.Owner),
		fixture.sourceTask.MessageID,
		protocol.BlockKindRerouteNeeded,
		"current owner is blocked; reroute this work",
	); err != nil {
		t.Fatalf("task block: %v", err)
	}

	blockerCase := mustReadBlockerCase(t, fixture)
	if blockerCase.SelectedAction != protocol.BlockerActionReroute {
		t.Fatalf("selected action = %q, want %q", blockerCase.SelectedAction, protocol.BlockerActionReroute)
	}
	if blockerCase.Status != protocol.BlockerStatusActive {
		t.Fatalf("status = %q, want %q", blockerCase.Status, protocol.BlockerStatusActive)
	}
	if blockerCase.DeclaredState != "block" {
		t.Fatalf("declared state = %q, want %q", blockerCase.DeclaredState, "block")
	}
	if blockerCase.BlockKind != protocol.BlockKindRerouteNeeded {
		t.Fatalf("block kind = %q, want %q", blockerCase.BlockKind, protocol.BlockKindRerouteNeeded)
	}
	if blockerCase.RerouteCount != 1 {
		t.Fatalf("reroute count = %d, want 1", blockerCase.RerouteCount)
	}
	if blockerCase.CurrentTaskID == fixture.sourceTask.TaskID {
		t.Fatalf("current task id = %q, want a rerouted task id", blockerCase.CurrentTaskID)
	}
	if blockerCase.CurrentMessageID == fixture.sourceTask.MessageID {
		t.Fatalf("current message id = %q, want a rerouted message id", blockerCase.CurrentMessageID)
	}
	if blockerCase.CurrentOwner == fixture.sourceTask.Owner {
		t.Fatalf("current owner = %q, want reroute to move ownership", blockerCase.CurrentOwner)
	}
	if len(blockerCase.Attempts) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(blockerCase.Attempts))
	}
	if blockerCase.Attempts[0].Action != protocol.BlockerActionReroute {
		t.Fatalf("attempt action = %q, want %q", blockerCase.Attempts[0].Action, protocol.BlockerActionReroute)
	}
	if blockerCase.Attempts[0].TaskID != blockerCase.CurrentTaskID {
		t.Fatalf("attempt task id = %q, want %q", blockerCase.Attempts[0].TaskID, blockerCase.CurrentTaskID)
	}
	if blockerCase.Attempts[0].MessageID != blockerCase.CurrentMessageID {
		t.Fatalf("attempt message id = %q, want %q", blockerCase.Attempts[0].MessageID, blockerCase.CurrentMessageID)
	}
	if blockerCase.Attempts[0].Owner != blockerCase.CurrentOwner {
		t.Fatalf("attempt owner = %q, want %q", blockerCase.Attempts[0].Owner, blockerCase.CurrentOwner)
	}
	if got := countRunTaskDocs(t, fixture.cfg.Session.StateDir, fixture.run.RunID); got != taskCountBefore+1 {
		t.Fatalf("task doc count = %d, want %d", got, taskCountBefore+1)
	}

	coordinatorStore := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir)
	reroutedTask, err := coordinatorStore.ReadTask(fixture.run.RunID, blockerCase.CurrentTaskID)
	if err != nil {
		t.Fatalf("read rerouted task: %v", err)
	}
	if reroutedTask.MessageID != blockerCase.CurrentMessageID {
		t.Fatalf("rerouted task message id = %q, want %q", reroutedTask.MessageID, blockerCase.CurrentMessageID)
	}
	if reroutedTask.Owner != blockerCase.CurrentOwner {
		t.Fatalf("rerouted task owner = %q, want %q", reroutedTask.Owner, blockerCase.CurrentOwner)
	}
}

func TestTaskBlockEscalatesAtRerouteCeiling(t *testing.T) {
	t.Parallel()

	t.Run("reroute ceiling", func(t *testing.T) {
		t.Parallel()

		fixture := seedBlockerPolicyFixture(t, 1)
		seedExistingBlockerCase(t, fixture, protocol.BlockerCase{
			RunID:            fixture.run.RunID,
			SourceTaskID:     fixture.sourceTask.TaskID,
			SourceMessageID:  fixture.sourceTask.MessageID,
			SourceOwner:      fixture.sourceTask.Owner,
			CurrentTaskID:    fixture.sourceTask.TaskID,
			CurrentMessageID: fixture.sourceTask.MessageID,
			CurrentOwner:     fixture.sourceTask.Owner,
			DeclaredState:    "block",
			BlockKind:        protocol.BlockKindRerouteNeeded,
			Reason:           "reroute budget already spent",
			SelectedAction:   protocol.BlockerActionReroute,
			Status:           protocol.BlockerStatusActive,
			RerouteCount:     1,
			MaxReroutes:      1,
		})

		if err := callTaskBlockForPolicy(
			fixture.cfg.Session.StateDir,
			string(fixture.sourceTask.Owner),
			fixture.sourceTask.MessageID,
			protocol.BlockKindRerouteNeeded,
			"needs another reroute",
		); err != nil {
			t.Fatalf("task block: %v", err)
		}

		blockerCase := mustReadBlockerCase(t, fixture)
		if blockerCase.SelectedAction != protocol.BlockerActionEscalate {
			t.Fatalf("selected action = %q, want %q", blockerCase.SelectedAction, protocol.BlockerActionEscalate)
		}
		if blockerCase.Status != protocol.BlockerStatusEscalated {
			t.Fatalf("status = %q, want %q", blockerCase.Status, protocol.BlockerStatusEscalated)
		}
		if blockerCase.RerouteCount != 1 {
			t.Fatalf("reroute count = %d, want 1", blockerCase.RerouteCount)
		}
		if blockerCase.RecommendedAction == nil {
			t.Fatalf("recommended action should not be nil")
		}
		if blockerCase.RecommendedAction.Kind != protocol.BlockerResolutionActionManualReroute {
			t.Fatalf("recommended action kind = %q, want %q", blockerCase.RecommendedAction.Kind, protocol.BlockerResolutionActionManualReroute)
		}
		if blockerCase.EscalatedAt == nil {
			t.Fatalf("escalated_at should not be nil")
		}
		if got := countRunTaskDocs(t, fixture.cfg.Session.StateDir, fixture.run.RunID); got != 1 {
			t.Fatalf("task doc count = %d, want 1", got)
		}
	})

	testCases := []struct {
		name      string
		blockKind protocol.BlockKind
	}{
		{name: "human decision", blockKind: protocol.BlockKindHumanDecision},
		{name: "unsupported", blockKind: protocol.BlockKindUnsupported},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := seedBlockerPolicyFixture(t, 2)

			if err := callTaskBlockForPolicy(
				fixture.cfg.Session.StateDir,
				string(fixture.sourceTask.Owner),
				fixture.sourceTask.MessageID,
				tc.blockKind,
				"operator input required",
			); err != nil {
				t.Fatalf("task block: %v", err)
			}

			blockerCase := mustReadBlockerCase(t, fixture)
			if blockerCase.SelectedAction != protocol.BlockerActionEscalate {
				t.Fatalf("selected action = %q, want %q", blockerCase.SelectedAction, protocol.BlockerActionEscalate)
			}
			if blockerCase.Status != protocol.BlockerStatusEscalated {
				t.Fatalf("status = %q, want %q", blockerCase.Status, protocol.BlockerStatusEscalated)
			}
			if blockerCase.RecommendedAction == nil {
				t.Fatalf("recommended action should not be nil")
			}
			if blockerCase.RecommendedAction.Kind != protocol.BlockerResolutionActionClarify {
				t.Fatalf("recommended action kind = %q, want %q", blockerCase.RecommendedAction.Kind, protocol.BlockerResolutionActionClarify)
			}
		})
	}
}

func TestTaskBlockClarificationDoesNotConsumeRerouteBudget(t *testing.T) {
	t.Parallel()

	fixture := seedBlockerPolicyFixture(t, 2)
	seedExistingBlockerCase(t, fixture, protocol.BlockerCase{
		RunID:            fixture.run.RunID,
		SourceTaskID:     fixture.sourceTask.TaskID,
		SourceMessageID:  fixture.sourceTask.MessageID,
		SourceOwner:      fixture.sourceTask.Owner,
		CurrentTaskID:    fixture.sourceTask.TaskID,
		CurrentMessageID: fixture.sourceTask.MessageID,
		CurrentOwner:     fixture.sourceTask.Owner,
		DeclaredState:    "block",
		BlockKind:        protocol.BlockKindRerouteNeeded,
		Reason:           "already rerouted once",
		SelectedAction:   protocol.BlockerActionReroute,
		Status:           protocol.BlockerStatusActive,
		RerouteCount:     1,
		MaxReroutes:      2,
	})

	if err := callTaskBlockForPolicy(
		fixture.cfg.Session.StateDir,
		string(fixture.sourceTask.Owner),
		fixture.sourceTask.MessageID,
		protocol.BlockKindAgentClarification,
		"need agent clarification from coordinator",
	); err != nil {
		t.Fatalf("task block: %v", err)
	}

	blockerCase := mustReadBlockerCase(t, fixture)
	if blockerCase.SelectedAction != protocol.BlockerActionClarificationRequest {
		t.Fatalf("selected action = %q, want %q", blockerCase.SelectedAction, protocol.BlockerActionClarificationRequest)
	}
	if blockerCase.Status != protocol.BlockerStatusActive {
		t.Fatalf("status = %q, want %q", blockerCase.Status, protocol.BlockerStatusActive)
	}
	if blockerCase.RerouteCount != 1 {
		t.Fatalf("reroute count = %d, want 1", blockerCase.RerouteCount)
	}
	if blockerCase.CurrentTaskID != fixture.sourceTask.TaskID {
		t.Fatalf("current task id = %q, want %q", blockerCase.CurrentTaskID, fixture.sourceTask.TaskID)
	}
	if blockerCase.CurrentMessageID != fixture.sourceTask.MessageID {
		t.Fatalf("current message id = %q, want %q", blockerCase.CurrentMessageID, fixture.sourceTask.MessageID)
	}
	if got := countRunTaskDocs(t, fixture.cfg.Session.StateDir, fixture.run.RunID); got != 1 {
		t.Fatalf("task doc count = %d, want 1", got)
	}
}

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

type blockerPolicyFixture struct {
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

func seedBlockerPolicyFixture(t *testing.T, maxReroutes int) blockerPolicyFixture {
	t.Helper()

	cfg := testRouteTaskConfig(t)
	cfg.Blockers.MaxReroutesDefault = maxReroutes
	cfg.Blockers.MaxReroutesByTaskClass = map[protocol.TaskClass]int{
		protocol.TaskClassImplementation: maxReroutes,
	}
	makeConfigLoadable(cfg)
	if err := createStateTree(cfg); err != nil {
		t.Fatalf("create state tree: %v", err)
	}
	if err := writeResolvedConfig(cfg); err != nil {
		t.Fatalf("write resolved config: %v", err)
	}

	store := mailbox.NewStore(cfg.Session.StateDir)
	run, err := Run(cfg, store, RunRequest{
		Goal:        "Exercise blocker policy for coordinator-run child tasks",
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
		Goal:           "Implement blocker policy handling",
		ExpectedOutput: "A routed task used by blocker-policy tests",
		ReviewRequired: true,
	})
	if err != nil {
		t.Fatalf("route child task: %v", err)
	}
	if _, err := ReadMsg(cfg.Session.StateDir, string(sourceTask.Owner), sourceTask.MessageID); err != nil {
		t.Fatalf("activate source task: %v", err)
	}

	return blockerPolicyFixture{
		cfg:        cfg,
		run:        run,
		sourceTask: sourceTask,
	}
}

func seedExistingBlockerCase(t *testing.T, fixture blockerPolicyFixture, caseDoc protocol.BlockerCase) {
	t.Helper()

	now := time.Now().UTC()
	if caseDoc.CreatedAt.IsZero() {
		caseDoc.CreatedAt = now
	}
	if caseDoc.UpdatedAt.IsZero() {
		caseDoc.UpdatedAt = now
	}

	if err := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir).CreateBlockerCase(&caseDoc); err != nil {
		t.Fatalf("create blocker case: %v", err)
	}
}

func mustReadBlockerCase(t *testing.T, fixture blockerPolicyFixture) *protocol.BlockerCase {
	t.Helper()

	blockerCase, err := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir).ReadBlockerCase(fixture.run.RunID, fixture.sourceTask.TaskID)
	if err != nil {
		t.Fatalf("read blocker case: %v", err)
	}

	return blockerCase
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func callTaskWaitForPolicy(stateDir, agent string, msgID protocol.MessageID, waitKind protocol.WaitKind, reason string) error {
	return TaskWait(stateDir, agent, msgID, waitKind, "", reason)
}

func callTaskBlockForPolicy(stateDir, agent string, msgID protocol.MessageID, blockKind protocol.BlockKind, reason string) error {
	return TaskBlock(stateDir, agent, msgID, blockKind, "", reason)
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
