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
		fixture.run.RunID,
		fixture.sourceTask.TaskID,
		protocol.BlockerResolutionActionManualReroute,
		"backend-low",
		"manual reroute to the lower-priority backend owner",
		nil,
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
		fixture.run.RunID,
		fixture.sourceTask.TaskID,
		protocol.BlockerResolutionActionClarify,
		"",
		"ask the current owner whether to keep the session dependency split",
		[]byte("Please confirm whether the session/protocol split still holds.\n"),
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
		fixture.run.RunID,
		fixture.sourceTask.TaskID,
		protocol.BlockerResolutionActionDismiss,
		"",
		"operator dismissed this blocker after manual inspection",
		nil,
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
