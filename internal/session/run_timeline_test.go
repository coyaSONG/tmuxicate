package session

import (
	"strings"
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestBuildRunTimelineIncludesRoutingReviewBlockerReplanAndStateTransitions(t *testing.T) {
	t.Parallel()

	fixture := seedRunTimelineFixture(t)

	timeline, err := BuildRunTimeline(fixture.cfg.Session.StateDir, fixture.graph)
	if err != nil {
		t.Fatalf("BuildRunTimeline() unexpected error: %v", err)
	}

	if timeline.RunID != fixture.run.RunID {
		t.Fatalf("timeline.RunID = %q, want %q", timeline.RunID, fixture.run.RunID)
	}

	requiredKinds := []string{
		"run.created",
		"task.created",
		"task.routed",
		"review.handoff",
		"review.respond",
		"blocker.escalated",
		"blocker.resolved",
		"partial_replan.applied",
		"task.accept",
		"task.wait",
		"task.block",
		"task.done",
	}
	for _, kind := range requiredKinds {
		if !timelineHasKind(timeline.Events, kind) {
			t.Fatalf("expected timeline to contain %q\n%#v", kind, timeline.Events)
		}
	}

	routeEvent := findTimelineEvent(t, timeline.Events, "task.routed", fixture.sourceTask.TaskID)
	if routeEvent.Owner != fixture.sourceTask.Owner {
		t.Fatalf("route event owner = %q, want %q", routeEvent.Owner, fixture.sourceTask.Owner)
	}
	if routeEvent.TaskClass != protocol.TaskClassImplementation {
		t.Fatalf("route event class = %q, want %q", routeEvent.TaskClass, protocol.TaskClassImplementation)
	}
	if routeEvent.ExecutionTarget != "sandbox" {
		t.Fatalf("route event target = %q, want %q", routeEvent.ExecutionTarget, "sandbox")
	}

	reviewEvent := findTimelineEvent(t, timeline.Events, "review.handoff", fixture.sourceTask.TaskID)
	if reviewEvent.TaskClass != protocol.TaskClassReview {
		t.Fatalf("review handoff class = %q, want %q", reviewEvent.TaskClass, protocol.TaskClassReview)
	}

	replanEvent := findTimelineEvent(t, timeline.Events, "partial_replan.applied", fixture.sourceTask.TaskID)
	if replanEvent.ExecutionTarget != "sandbox" {
		t.Fatalf("partial replan source target = %q, want %q", replanEvent.ExecutionTarget, "sandbox")
	}

	localLifecycle := findTimelineEvent(t, timeline.Events, "task.wait", fixture.waitTask.TaskID)
	if localLifecycle.ExecutionTarget != "local" {
		t.Fatalf("local lifecycle target = %q, want %q", localLifecycle.ExecutionTarget, "local")
	}
}

func TestBuildRunTimelineSortsDeterministicallyWhenTimestampsCollide(t *testing.T) {
	t.Parallel()

	fixture := seedRunTimelineFixture(t)
	writeStateEventAt(t, fixture.cfg.Session.StateDir, "backend-low", &TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     "2026-04-05T06:00:00Z",
		Agent:         "backend-low",
		Event:         "task.accept",
		DeclaredState: "busy",
		MessageID:     fixture.waitTask.MessageID,
		Thread:        fixture.waitTask.ThreadID,
		ReceiptState:  protocol.FolderStateActive,
	})
	writeStateEventAt(t, fixture.cfg.Session.StateDir, "backend-low", &TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     "2026-04-05T06:00:00Z",
		Agent:         "backend-low",
		Event:         "task.accept",
		DeclaredState: "busy",
		MessageID:     fixture.blockTask.MessageID,
		Thread:        fixture.blockTask.ThreadID,
		ReceiptState:  protocol.FolderStateActive,
	})

	firstTimeline, err := BuildRunTimeline(fixture.cfg.Session.StateDir, fixture.graph)
	if err != nil {
		t.Fatalf("BuildRunTimeline() first call unexpected error: %v", err)
	}
	secondTimeline, err := BuildRunTimeline(fixture.cfg.Session.StateDir, fixture.graph)
	if err != nil {
		t.Fatalf("BuildRunTimeline() second call unexpected error: %v", err)
	}

	first := summarizeTimelineOrder(firstTimeline.Events)
	second := summarizeTimelineOrder(secondTimeline.Events)
	if strings.Join(first, "\n") != strings.Join(second, "\n") {
		t.Fatalf("timeline ordering not stable\nfirst:\n%s\nsecond:\n%s", strings.Join(first, "\n"), strings.Join(second, "\n"))
	}
}

func TestBuildRunTimelineRejectsStateEventAgentMismatch(t *testing.T) {
	t.Parallel()

	fixture := seedRunTimelineFixture(t)
	writeStateEventAt(t, fixture.cfg.Session.StateDir, "reviewer", &TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     "2026-04-05T06:00:00Z",
		Agent:         "reviewer",
		Event:         "task.done",
		DeclaredState: "idle",
		MessageID:     fixture.waitTask.MessageID,
		Thread:        fixture.waitTask.ThreadID,
		ReceiptState:  protocol.FolderStateDone,
		Summary:       "wrong owner wrote a known task event",
	})

	_, err := BuildRunTimeline(fixture.cfg.Session.StateDir, fixture.graph)
	if err == nil {
		t.Fatalf("expected state-event agent mismatch to fail")
	}
	if !strings.Contains(err.Error(), "coordinator artifact mismatch") {
		t.Fatalf("expected coordinator artifact mismatch, got %v", err)
	}
}

func TestFilterRunTimelineByOwnerStateClassAndExecutionTarget(t *testing.T) {
	t.Parallel()

	fixture := seedRunTimelineFixture(t)
	timeline, err := BuildRunTimeline(fixture.cfg.Session.StateDir, fixture.graph)
	if err != nil {
		t.Fatalf("BuildRunTimeline() unexpected error: %v", err)
	}

	allEvents := FilterRunTimeline(timeline, RunTimelineFilter{})
	if len(allEvents) != len(timeline.Events) {
		t.Fatalf("empty filter len = %d, want %d", len(allEvents), len(timeline.Events))
	}

	sandboxDone := FilterRunTimeline(timeline, RunTimelineFilter{
		Owner:           "backend-high",
		State:           "idle",
		TaskClass:       protocol.TaskClassImplementation,
		ExecutionTarget: "sandbox",
	})
	if len(sandboxDone) == 0 {
		t.Fatalf("expected combined sandbox filter to match at least one event")
	}
	for _, event := range sandboxDone {
		if event.Owner != "backend-high" || event.State != "idle" || event.TaskClass != protocol.TaskClassImplementation || event.ExecutionTarget != "sandbox" {
			t.Fatalf("unexpected combined filter event: %#v", event)
		}
	}

	reviewEvents := FilterRunTimeline(timeline, RunTimelineFilter{TaskClass: protocol.TaskClassReview})
	if len(reviewEvents) == 0 {
		t.Fatalf("expected review class filter to match review events")
	}
	for _, event := range reviewEvents {
		if event.TaskClass != protocol.TaskClassReview {
			t.Fatalf("review filter returned non-review event: %#v", event)
		}
	}

	localEvents := FilterRunTimeline(timeline, RunTimelineFilter{ExecutionTarget: "local"})
	if len(localEvents) == 0 {
		t.Fatalf("expected local target filter to match implicit local placement")
	}
	for _, event := range localEvents {
		if event.ExecutionTarget != "local" {
			t.Fatalf("local target filter returned non-local event: %#v", event)
		}
	}
}

type runTimelineFixture struct {
	cfg             *config.ResolvedConfig
	run             *protocol.CoordinatorRun
	graph           *RunGraph
	sourceTask      *protocol.ChildTask
	waitTask        *protocol.ChildTask
	blockTask       *protocol.ChildTask
	replacementTask *protocol.ChildTask
}

func seedRunTimelineFixture(t *testing.T) runTimelineFixture {
	t.Helper()

	cfg := testExecutionTargetRouteConfig(t)
	makeConfigLoadable(cfg)
	if err := createStateTree(cfg); err != nil {
		t.Fatalf("create state tree: %v", err)
	}
	if err := writeResolvedConfig(cfg); err != nil {
		t.Fatalf("write resolved config: %v", err)
	}

	store := mailbox.NewStore(cfg.Session.StateDir)
	coordinatorStore := mailbox.NewCoordinatorStore(cfg.Session.StateDir)

	run, err := Run(cfg, store, RunRequest{
		Goal:        "Build one strict timeline from durable coordinator artifacts and task events",
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
		Goal:           "Implement timeline projection output",
		ExpectedOutput: "routed implementation task with review handoff and blocker lineage",
		ReviewRequired: true,
	})
	if err != nil {
		t.Fatalf("route child task: %v", err)
	}
	if _, err := ReadMsg(cfg.Session.StateDir, string(sourceTask.Owner), sourceTask.MessageID); err != nil {
		t.Fatalf("activate source task: %v", err)
	}
	if err := TaskDone(cfg.Session.StateDir, string(sourceTask.Owner), sourceTask.MessageID, "source implementation complete"); err != nil {
		t.Fatalf("complete source task: %v", err)
	}

	handoff, err := coordinatorStore.ReadReviewHandoff(run.RunID, sourceTask.TaskID)
	if err != nil {
		t.Fatalf("read review handoff: %v", err)
	}
	if _, err := ReadMsg(cfg.Session.StateDir, "reviewer", handoff.ReviewMessageID); err != nil {
		t.Fatalf("activate review task: %v", err)
	}
	if _, err := ReviewRespond(cfg.Session.StateDir, store, "reviewer", handoff.ReviewMessageID, protocol.ReviewOutcomeApproved, []byte("approved\n")); err != nil {
		t.Fatalf("review respond: %v", err)
	}

	waitTask, err := AddChildTask(cfg, store, ChildTaskRequest{
		ParentRunID:    run.RunID,
		Owner:          "backend-low",
		Goal:           "Wait on one durable external dependency",
		ExpectedOutput: "state timeline includes a wait transition for implicit local placement",
	})
	if err != nil {
		t.Fatalf("add wait task: %v", err)
	}
	if err := TaskAccept(cfg.Session.StateDir, "backend-low", waitTask.MessageID); err != nil {
		t.Fatalf("accept wait task: %v", err)
	}
	if err := TaskWait(cfg.Session.StateDir, "backend-low", waitTask.MessageID, protocol.WaitKindExternalEvent, "ops", "waiting for dependency completion"); err != nil {
		t.Fatalf("wait task: %v", err)
	}

	blockTask, err := AddChildTask(cfg, store, ChildTaskRequest{
		ParentRunID:    run.RunID,
		Owner:          "backend-low",
		Goal:           "Block on a missing decision",
		ExpectedOutput: "state timeline includes a block transition for implicit local placement",
	})
	if err != nil {
		t.Fatalf("add block task: %v", err)
	}
	if err := TaskAccept(cfg.Session.StateDir, "backend-low", blockTask.MessageID); err != nil {
		t.Fatalf("accept block task: %v", err)
	}
	if err := TaskBlock(cfg.Session.StateDir, "backend-low", blockTask.MessageID, protocol.BlockKindHumanDecision, "human", "need operator decision"); err != nil {
		t.Fatalf("block task: %v", err)
	}

	blocker := createEscalatedBlockerCase(t, cfg.Session.StateDir, run.RunID, sourceTask, escalatedBlockerOptions{})
	replacementTask, err := AddChildTask(cfg, store, ChildTaskRequest{
		ParentRunID:    run.RunID,
		Owner:          "backend-low",
		Goal:           "Continue source work through one bounded replacement task",
		ExpectedOutput: "partial replan persists replacement lineage with implicit local target",
		ReviewRequired: sourceTask.ReviewRequired,
	})
	if err != nil {
		t.Fatalf("add replacement task: %v", err)
	}
	writeTaskState(t, cfg.Session.StateDir, string(replacementTask.Owner), replacementTask.MessageID, replacementTask.ThreadID, protocol.FolderStateUnread, "idle")

	if err := coordinatorStore.UpdateBlockerCase(run.RunID, sourceTask.TaskID, func(existing *protocol.BlockerCase) error {
		now := time.Date(2026, time.April, 6, 10, 15, 0, 0, time.UTC)
		existing.Status = protocol.BlockerStatusResolved
		existing.CurrentTaskID = replacementTask.TaskID
		existing.CurrentMessageID = replacementTask.MessageID
		existing.CurrentOwner = replacementTask.Owner
		existing.Resolution = &protocol.BlockerResolution{
			Action:           protocol.BlockerResolutionActionPartialReplan,
			CreatedTaskID:    replacementTask.TaskID,
			CreatedMessageID: replacementTask.MessageID,
			ResolvedBy:       "human",
			Note:             "replace the blocked work with a bounded follow-up",
			CreatedAt:        now,
		}
		existing.ResolvedAt = &now
		existing.UpdatedAt = now
		existing.RecommendedAction = &protocol.RecommendedAction{
			Kind: protocol.BlockerResolutionActionPartialReplan,
			Note: blocker.RecommendedAction.Note,
		}
		return nil
	}); err != nil {
		t.Fatalf("update blocker case: %v", err)
	}

	replanTime := time.Date(2026, time.April, 6, 10, 16, 0, 0, time.UTC)
	if err := coordinatorStore.CreatePartialReplan(&protocol.PartialReplan{
		RunID:                run.RunID,
		SourceTaskID:         sourceTask.TaskID,
		SourceMessageID:      sourceTask.MessageID,
		BlockerSourceTaskID:  sourceTask.TaskID,
		SupersededTaskID:     sourceTask.TaskID,
		SupersededMessageID:  sourceTask.MessageID,
		SupersededOwner:      sourceTask.Owner,
		ReplacementTaskID:    replacementTask.TaskID,
		ReplacementMessageID: replacementTask.MessageID,
		ReplacementOwner:     replacementTask.Owner,
		Reason:               "replace the blocked work with one bounded follow-up task",
		Status:               protocol.PartialReplanStatusApplied,
		CreatedAt:            replanTime,
		UpdatedAt:            replanTime,
	}); err != nil {
		t.Fatalf("create partial replan: %v", err)
	}

	graph, err := LoadRunGraph(cfg.Session.StateDir, run.RunID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	return runTimelineFixture{
		cfg:             cfg,
		run:             run,
		graph:           graph,
		sourceTask:      sourceTask,
		waitTask:        waitTask,
		blockTask:       blockTask,
		replacementTask: replacementTask,
	}
}

func writeStateEventAt(t *testing.T, stateDir, agent string, event *TaskEvent) {
	t.Helper()

	if err := appendStateEvent(stateDir, agent, event); err != nil {
		t.Fatalf("append state event: %v", err)
	}
}

func timelineHasKind(events []RunTimelineEvent, kind string) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}

	return false
}

func findTimelineEvent(t *testing.T, events []RunTimelineEvent, kind string, taskID protocol.TaskID) RunTimelineEvent {
	t.Helper()

	for _, event := range events {
		if event.Kind == kind && event.TaskID == taskID {
			return event
		}
	}

	t.Fatalf("missing timeline event %q for task %s", kind, taskID)
	return RunTimelineEvent{}
}

func summarizeTimelineOrder(events []RunTimelineEvent) []string {
	summary := make([]string, 0, len(events))
	for _, event := range events {
		summary = append(summary, event.Timestamp.Format(time.RFC3339Nano)+"|"+event.Kind+"|"+string(event.TaskID)+"|"+string(event.MessageID)+"|"+string(event.Owner))
	}

	return summary
}
