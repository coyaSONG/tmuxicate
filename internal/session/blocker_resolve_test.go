package session

import (
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestBlockerResolveManualRerouteRecordsResolution(t *testing.T) {
	t.Parallel()

	fixture := seedEscalatedBlockerFixture(t, protocol.BlockerResolutionActionManualReroute)
	store := mailbox.NewStore(fixture.cfg.Session.StateDir)

	if err := BlockerResolve(
		fixture.cfg.Session.StateDir,
		store,
		BlockerResolveOpts{
			RunID:        fixture.run.RunID,
			SourceTaskID: fixture.sourceTask.TaskID,
			Action:       protocol.BlockerResolutionActionManualReroute,
			Owner:        "backend-low",
			Reason:       "manual reroute to the lower-priority backend owner",
		},
	); err != nil {
		t.Fatalf("blocker resolve: %v", err)
	}

	blockerCase := mustReadBlockerCase(t, fixture.blockerPolicyFixture)
	if blockerCase.Status != protocol.BlockerStatusResolved {
		t.Fatalf("status = %q, want %q", blockerCase.Status, protocol.BlockerStatusResolved)
	}
	if blockerCase.Resolution == nil {
		t.Fatalf("resolution should not be nil")
	}
	if blockerCase.Resolution.Action != protocol.BlockerResolutionActionManualReroute {
		t.Fatalf("resolution action = %q, want %q", blockerCase.Resolution.Action, protocol.BlockerResolutionActionManualReroute)
	}
	if blockerCase.Resolution.Note != "manual reroute to the lower-priority backend owner" {
		t.Fatalf("resolution note = %q, want manual reroute reason", blockerCase.Resolution.Note)
	}
	if blockerCase.Resolution.CreatedTaskID == "" {
		t.Fatalf("created task id should not be empty")
	}
	if blockerCase.Resolution.CreatedMessageID == "" {
		t.Fatalf("created message id should not be empty")
	}
	if blockerCase.CurrentTaskID != blockerCase.Resolution.CreatedTaskID {
		t.Fatalf("current task id = %q, want %q", blockerCase.CurrentTaskID, blockerCase.Resolution.CreatedTaskID)
	}
	if blockerCase.CurrentMessageID != blockerCase.Resolution.CreatedMessageID {
		t.Fatalf("current message id = %q, want %q", blockerCase.CurrentMessageID, blockerCase.Resolution.CreatedMessageID)
	}
	if blockerCase.CurrentOwner != "backend-low" {
		t.Fatalf("current owner = %q, want %q", blockerCase.CurrentOwner, "backend-low")
	}
}

func TestBlockerResolveClarifySendsDecisionMessage(t *testing.T) {
	t.Parallel()

	fixture := seedEscalatedBlockerFixture(t, protocol.BlockerResolutionActionClarify)
	store := mailbox.NewStore(fixture.cfg.Session.StateDir)

	if err := BlockerResolve(
		fixture.cfg.Session.StateDir,
		store,
		BlockerResolveOpts{
			RunID:        fixture.run.RunID,
			SourceTaskID: fixture.sourceTask.TaskID,
			Action:       protocol.BlockerResolutionActionClarify,
			Reason:       "ask the current owner whether to keep the session dependency split",
			Body:         []byte("Please confirm whether the session/protocol split still holds.\n"),
		},
	); err != nil {
		t.Fatalf("blocker resolve: %v", err)
	}

	blockerCase := mustReadBlockerCase(t, fixture.blockerPolicyFixture)
	if blockerCase.Status != protocol.BlockerStatusResolved {
		t.Fatalf("status = %q, want %q", blockerCase.Status, protocol.BlockerStatusResolved)
	}
	if blockerCase.Resolution == nil {
		t.Fatalf("resolution should not be nil")
	}
	if blockerCase.Resolution.Action != protocol.BlockerResolutionActionClarify {
		t.Fatalf("resolution action = %q, want %q", blockerCase.Resolution.Action, protocol.BlockerResolutionActionClarify)
	}
	if blockerCase.Resolution.Note != "ask the current owner whether to keep the session dependency split" {
		t.Fatalf("resolution note = %q, want clarify reason", blockerCase.Resolution.Note)
	}
	if blockerCase.Resolution.CreatedMessageID == "" {
		t.Fatalf("created message id should not be empty")
	}
	if blockerCase.Resolution.CreatedTaskID != "" {
		t.Fatalf("created task id = %q, want empty", blockerCase.Resolution.CreatedTaskID)
	}

	env, _, err := store.ReadMessage(blockerCase.Resolution.CreatedMessageID)
	if err != nil {
		t.Fatalf("read decision message: %v", err)
	}
	if env.Kind != protocol.KindDecision {
		t.Fatalf("decision message kind = %q, want %q", env.Kind, protocol.KindDecision)
	}
	if env.Thread != fixture.run.RootThreadID {
		t.Fatalf("decision thread = %q, want %q", env.Thread, fixture.run.RootThreadID)
	}
	if env.ReplyTo == nil || *env.ReplyTo != fixture.sourceTask.MessageID {
		t.Fatalf("decision reply_to = %v, want %q", env.ReplyTo, fixture.sourceTask.MessageID)
	}
}

func TestBlockerResolveDismissMarksResolved(t *testing.T) {
	t.Parallel()

	fixture := seedEscalatedBlockerFixture(t, protocol.BlockerResolutionActionClarify)
	store := mailbox.NewStore(fixture.cfg.Session.StateDir)
	taskCountBefore := countRunTaskDocs(t, fixture.cfg.Session.StateDir, fixture.run.RunID)
	messageCountBefore := countMessageDirs(t, fixture.cfg.Session.StateDir)

	if err := BlockerResolve(
		fixture.cfg.Session.StateDir,
		store,
		BlockerResolveOpts{
			RunID:        fixture.run.RunID,
			SourceTaskID: fixture.sourceTask.TaskID,
			Action:       protocol.BlockerResolutionActionDismiss,
			Reason:       "operator dismissed this blocker after manual inspection",
		},
	); err != nil {
		t.Fatalf("blocker resolve: %v", err)
	}

	blockerCase := mustReadBlockerCase(t, fixture.blockerPolicyFixture)
	if blockerCase.Status != protocol.BlockerStatusResolved {
		t.Fatalf("status = %q, want %q", blockerCase.Status, protocol.BlockerStatusResolved)
	}
	if blockerCase.Resolution == nil {
		t.Fatalf("resolution should not be nil")
	}
	if blockerCase.Resolution.Action != protocol.BlockerResolutionActionDismiss {
		t.Fatalf("resolution action = %q, want %q", blockerCase.Resolution.Action, protocol.BlockerResolutionActionDismiss)
	}
	if blockerCase.Resolution.Note != "operator dismissed this blocker after manual inspection" {
		t.Fatalf("resolution note = %q, want dismiss reason", blockerCase.Resolution.Note)
	}
	if blockerCase.Resolution.CreatedTaskID != "" {
		t.Fatalf("created task id = %q, want empty", blockerCase.Resolution.CreatedTaskID)
	}
	if blockerCase.Resolution.CreatedMessageID != "" {
		t.Fatalf("created message id = %q, want empty", blockerCase.Resolution.CreatedMessageID)
	}
	if got := countRunTaskDocs(t, fixture.cfg.Session.StateDir, fixture.run.RunID); got != taskCountBefore {
		t.Fatalf("task doc count = %d, want %d", got, taskCountBefore)
	}
	if got := countMessageDirs(t, fixture.cfg.Session.StateDir); got != messageCountBefore {
		t.Fatalf("message dir count = %d, want %d", got, messageCountBefore)
	}
}

func TestBlockerResolvePartialReplanCreatesReplacementTaskAndArtifact(t *testing.T) {
	t.Parallel()

	fixture := seedEscalatedBlockerFixture(t, protocol.BlockerResolutionActionPartialReplan)
	store := mailbox.NewStore(fixture.cfg.Session.StateDir)
	taskCountBefore := countRunTaskDocs(t, fixture.cfg.Session.StateDir, fixture.run.RunID)

	err := BlockerResolve(
		fixture.cfg.Session.StateDir,
		store,
		BlockerResolveOpts{
			RunID:          fixture.run.RunID,
			SourceTaskID:   fixture.sourceTask.TaskID,
			Action:         protocol.BlockerResolutionActionPartialReplan,
			Reason:         "replace the blocked implementation with one bounded reroute",
			TaskClass:      protocol.TaskClassImplementation,
			Domains:        []string{"session", "protocol"},
			Goal:           "Implement the bounded replacement path",
			ExpectedOutput: "replacement task continues the same source-task lineage",
		},
	)
	if err != nil {
		t.Fatalf("blocker resolve: %v", err)
	}

	blockerCase := mustReadBlockerCase(t, fixture.blockerPolicyFixture)
	if blockerCase.Status != protocol.BlockerStatusResolved {
		t.Fatalf("status = %q, want %q", blockerCase.Status, protocol.BlockerStatusResolved)
	}
	if blockerCase.Resolution == nil {
		t.Fatalf("resolution should not be nil")
	}
	if blockerCase.Resolution.Action != protocol.BlockerResolutionActionPartialReplan {
		t.Fatalf("resolution action = %q, want %q", blockerCase.Resolution.Action, protocol.BlockerResolutionActionPartialReplan)
	}
	if blockerCase.Resolution.CreatedTaskID == "" || blockerCase.Resolution.CreatedMessageID == "" {
		t.Fatalf("resolution should record replacement task/message ids: %#v", blockerCase.Resolution)
	}
	if blockerCase.CurrentTaskID != blockerCase.Resolution.CreatedTaskID {
		t.Fatalf("current task id = %q, want %q", blockerCase.CurrentTaskID, blockerCase.Resolution.CreatedTaskID)
	}
	if blockerCase.CurrentMessageID != blockerCase.Resolution.CreatedMessageID {
		t.Fatalf("current message id = %q, want %q", blockerCase.CurrentMessageID, blockerCase.Resolution.CreatedMessageID)
	}

	if got := countRunTaskDocs(t, fixture.cfg.Session.StateDir, fixture.run.RunID); got != taskCountBefore+1 {
		t.Fatalf("task doc count = %d, want %d", got, taskCountBefore+1)
	}

	supersededReceipt, err := store.ReadReceipt(string(fixture.sourceTask.Owner), fixture.sourceTask.MessageID)
	if err != nil {
		t.Fatalf("read superseded receipt: %v", err)
	}
	if supersededReceipt.FolderState == protocol.FolderStateActive {
		t.Fatalf("superseded receipt should not remain active")
	}

	replacementTask, err := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir).ReadTask(fixture.run.RunID, blockerCase.Resolution.CreatedTaskID)
	if err != nil {
		t.Fatalf("read replacement task: %v", err)
	}
	if replacementTask.ParentRunID != fixture.run.RunID {
		t.Fatalf("replacement parent run id = %q, want %q", replacementTask.ParentRunID, fixture.run.RunID)
	}
	if replacementTask.ThreadID != fixture.run.RootThreadID {
		t.Fatalf("replacement thread id = %q, want %q", replacementTask.ThreadID, fixture.run.RootThreadID)
	}

	replan, err := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir).ReadPartialReplan(fixture.run.RunID, fixture.sourceTask.TaskID)
	if err != nil {
		t.Fatalf("read partial replan: %v", err)
	}
	if replan.SourceTaskID != fixture.sourceTask.TaskID {
		t.Fatalf("replan source task id = %q, want %q", replan.SourceTaskID, fixture.sourceTask.TaskID)
	}
	if replan.SupersededTaskID != fixture.sourceTask.TaskID {
		t.Fatalf("replan superseded task id = %q, want %q", replan.SupersededTaskID, fixture.sourceTask.TaskID)
	}
	if replan.ReplacementTaskID != replacementTask.TaskID {
		t.Fatalf("replan replacement task id = %q, want %q", replan.ReplacementTaskID, replacementTask.TaskID)
	}
}

func TestBlockerResolvePartialReplanRejectsExistingArtifactOrNonEscalatedBlocker(t *testing.T) {
	t.Parallel()

	t.Run("rejects existing partial replan artifact", func(t *testing.T) {
		t.Parallel()

		fixture := seedEscalatedBlockerFixture(t, protocol.BlockerResolutionActionPartialReplan)
		store := mailbox.NewStore(fixture.cfg.Session.StateDir)
		coordinatorStore := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir)

		replacementTask, err := AddChildTask(fixture.cfg, store, ChildTaskRequest{
			ParentRunID:    fixture.run.RunID,
			Owner:          "backend-low",
			Goal:           "Existing bounded replacement",
			ExpectedOutput: "duplicate partial replan should be rejected",
			ReviewRequired: fixture.sourceTask.ReviewRequired,
		})
		if err != nil {
			t.Fatalf("add child task: %v", err)
		}

		now := time.Now().UTC()
		if err := coordinatorStore.CreatePartialReplan(&protocol.PartialReplan{
			RunID:                fixture.run.RunID,
			SourceTaskID:         fixture.sourceTask.TaskID,
			SourceMessageID:      fixture.sourceTask.MessageID,
			BlockerSourceTaskID:  fixture.sourceTask.TaskID,
			SupersededTaskID:     fixture.sourceTask.TaskID,
			SupersededMessageID:  fixture.sourceTask.MessageID,
			SupersededOwner:      fixture.sourceTask.Owner,
			ReplacementTaskID:    replacementTask.TaskID,
			ReplacementMessageID: replacementTask.MessageID,
			ReplacementOwner:     replacementTask.Owner,
			Reason:               "existing bounded replacement",
			Status:               protocol.PartialReplanStatusApplied,
			CreatedAt:            now,
			UpdatedAt:            now,
		}); err != nil {
			t.Fatalf("CreatePartialReplan() unexpected error: %v", err)
		}

		err = BlockerResolve(
			fixture.cfg.Session.StateDir,
			store,
			BlockerResolveOpts{
				RunID:          fixture.run.RunID,
				SourceTaskID:   fixture.sourceTask.TaskID,
				Action:         protocol.BlockerResolutionActionPartialReplan,
				Reason:         "attempt a second bounded replacement",
				TaskClass:      protocol.TaskClassImplementation,
				Domains:        []string{"session", "protocol"},
				Goal:           "Try again",
				ExpectedOutput: "should fail",
			},
		)
		if err == nil {
			t.Fatalf("expected duplicate partial replan to fail")
		}
	})

	t.Run("rejects non-escalated blocker case", func(t *testing.T) {
		t.Parallel()

		fixture := seedEscalatedBlockerFixture(t, protocol.BlockerResolutionActionPartialReplan)
		store := mailbox.NewStore(fixture.cfg.Session.StateDir)
		if err := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir).UpdateBlockerCase(fixture.run.RunID, fixture.sourceTask.TaskID, func(existing *protocol.BlockerCase) error {
			existing.Status = protocol.BlockerStatusActive
			existing.RecommendedAction = nil
			existing.EscalatedAt = nil
			existing.UpdatedAt = time.Now().UTC()
			return nil
		}); err != nil {
			t.Fatalf("UpdateBlockerCase() unexpected error: %v", err)
		}

		err := BlockerResolve(
			fixture.cfg.Session.StateDir,
			store,
			BlockerResolveOpts{
				RunID:          fixture.run.RunID,
				SourceTaskID:   fixture.sourceTask.TaskID,
				Action:         protocol.BlockerResolutionActionPartialReplan,
				Reason:         "attempt replan before escalation",
				TaskClass:      protocol.TaskClassImplementation,
				Domains:        []string{"session", "protocol"},
				Goal:           "Try again",
				ExpectedOutput: "should fail",
			},
		)
		if err == nil {
			t.Fatalf("expected non-escalated blocker to fail")
		}
	})
}

type escalatedBlockerFixture struct {
	blockerPolicyFixture
	blockerCase *protocol.BlockerCase
}

func seedEscalatedBlockerFixture(t *testing.T, recommended protocol.BlockerResolutionAction) escalatedBlockerFixture {
	t.Helper()

	fixture := seedBlockerPolicyFixture(t, 1)
	blockKind := protocol.BlockKindHumanDecision
	rerouteCount := 0
	if recommended == protocol.BlockerResolutionActionManualReroute {
		blockKind = protocol.BlockKindRerouteNeeded
		rerouteCount = 1
	}

	now := time.Now().UTC()
	blockerCase := protocol.BlockerCase{
		RunID:            fixture.run.RunID,
		SourceTaskID:     fixture.sourceTask.TaskID,
		SourceMessageID:  fixture.sourceTask.MessageID,
		SourceOwner:      fixture.sourceTask.Owner,
		CurrentTaskID:    fixture.sourceTask.TaskID,
		CurrentMessageID: fixture.sourceTask.MessageID,
		CurrentOwner:     fixture.sourceTask.Owner,
		DeclaredState:    "block",
		BlockKind:        blockKind,
		Reason:           "operator intervention required",
		SelectedAction:   protocol.BlockerActionEscalate,
		Status:           protocol.BlockerStatusEscalated,
		RerouteCount:     rerouteCount,
		MaxReroutes:      1,
		RecommendedAction: &protocol.RecommendedAction{
			Kind: recommended,
			Note: "follow the recommended operator action",
		},
		CreatedAt:   now,
		UpdatedAt:   now,
		EscalatedAt: &now,
	}
	seedExistingBlockerCase(t, fixture, blockerCase)

	return escalatedBlockerFixture{
		blockerPolicyFixture: fixture,
		blockerCase:          &blockerCase,
	}
}
