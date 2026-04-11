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

func TestRunShowIncludesRoutingDecisionEvidence(t *testing.T) {
	t.Parallel()

	cfg := testRouteTaskConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := Run(cfg, store, RunRequest{
		Goal:        "Rebuild routed task evidence from disk",
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
		Goal:           "Implement durable routing evidence",
		ExpectedOutput: "run show renders duplicate and tie-break context from task YAML",
	})
	if err != nil {
		t.Fatalf("route child task: %v", err)
	}

	writeTaskState(t, cfg.Session.StateDir, "backend-high", task.MessageID, task.ThreadID, protocol.FolderStateUnread, "idle")
	mutateChildTaskDocument(t, cfg.Session.StateDir, run.RunID, task.TaskID, func(taskDoc map[string]any) {
		taskDoc["task_class"] = "implementation"
		taskDoc["domains"] = []string{"session", "protocol"}
		taskDoc["normalized_domains"] = []string{"protocol", "session"}
		taskDoc["duplicate_key"] = string(run.RunID) + "|implementation|protocol,session"
		taskDoc["override_reason"] = "manual reviewer pass"
		taskDoc["routing_decision"] = map[string]any{
			"status":           "selected",
			"selected_owner":   "backend-high",
			"candidates":       []string{"backend-high", "backend-low"},
			"tie_break":        "route_priority desc, config_order asc",
			"duplicate_status": "unique",
		}
	})

	graph, err := LoadRunGraph(cfg.Session.StateDir, run.RunID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	output := FormatRunGraph(graph)
	requiredSnippets := []string{
		"Task Class: implementation",
		"Domains: protocol, session",
		"Duplicate Key: " + string(run.RunID) + "|implementation|protocol,session",
		"Routing Decision: selected backend-high",
		"Candidates: backend-high, backend-low",
		"Override Reason: manual reviewer pass",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected formatted run graph to contain %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestFormatRunGraphIncludesAdaptiveRoutingExplanation(t *testing.T) {
	t.Parallel()

	cfg := testAdaptiveRoutingConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)
	run, err := Run(cfg, store, RunRequest{
		Goal:        "Render adaptive routing explanation from durable task artifacts",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	preferences := &protocol.AdaptiveRoutingPreferenceSet{
		Coordinator:  "pm",
		UpdatedAt:    time.Now().UTC(),
		LookbackRuns: 3,
		Preferences: []protocol.AdaptiveRoutingPreference{
			{
				PreferenceKey:     "implementation|protocol,session|backend-steady",
				TaskClass:         protocol.TaskClassImplementation,
				NormalizedDomains: []string{"protocol", "session"},
				PreferredOwner:    "backend-steady",
				HistoricalScore:   4,
				ManualWeight:      2,
				TotalScore:        6,
				Evidence: []protocol.AdaptiveRoutingEvidenceRef{
					{RunID: "run_000000000001", SourceTaskID: "task_000000000001", MessageID: "msg_000000000001", Status: "completed", Note: "completed source task without blocker or review downgrade"},
				},
			},
		},
	}
	if err := mailbox.NewCoordinatorStore(cfg.Session.StateDir).WriteAdaptiveRoutingPreferences(preferences); err != nil {
		t.Fatalf("WriteAdaptiveRoutingPreferences() unexpected error: %v", err)
	}

	task, _, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
		RunID:          run.RunID,
		TaskClass:      protocol.TaskClassImplementation,
		Domains:        []string{"session", "protocol"},
		Goal:           "Render adaptive routing from run show",
		ExpectedOutput: "task-local routing evidence includes adaptive detail",
	})
	if err != nil {
		t.Fatalf("route child task: %v", err)
	}

	writeTaskState(t, cfg.Session.StateDir, string(task.Owner), task.MessageID, task.ThreadID, protocol.FolderStateUnread, "idle")

	graph, err := LoadRunGraph(cfg.Session.StateDir, run.RunID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	output := FormatRunGraph(graph)
	requiredSnippets := []string{
		"Adaptive Routing:",
		"Adaptive Baseline:",
		"Adaptive Score:",
		"Adaptive Evidence:",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected formatted run graph to contain %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestRunShowIncludesReviewHandoffBlock(t *testing.T) {
	t.Parallel()

	fixture := seedRespondedReviewFixture(t)

	graph, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	output := FormatRunGraph(graph)
	requiredSnippets := []string{
		"Task: " + string(fixture.sourceTask.TaskID),
		"Review Handoff: responded",
		"Review Task: " + string(fixture.handoff.ReviewTaskID),
		"Reviewer: " + string(fixture.handoff.Reviewer),
		"Response: " + string(fixture.handoff.ResponseMessageID),
		"Outcome: approved",
		"Failure: -",
		"Task: " + string(fixture.handoff.ReviewTaskID),
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected formatted run graph to contain %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestFormatRunGraphIncludesSummaryBeforeTaskDetails(t *testing.T) {
	t.Parallel()

	fixture := seedPendingReviewFixture(t)
	createEscalatedBlockerCase(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask, escalatedBlockerOptions{})

	graph, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	output := FormatRunGraph(graph)
	if !strings.HasPrefix(output, "Run: "+string(fixture.run.RunID)+"\n") {
		t.Fatalf("expected run show output to start with run header\noutput:\n%s", output)
	}

	summaryIndex := strings.Index(output, "Summary:\n")
	if summaryIndex == -1 {
		t.Fatalf("expected run show output to include summary block\noutput:\n%s", output)
	}

	firstTaskIndex := strings.Index(output, "\nTask: ")
	if firstTaskIndex == -1 {
		t.Fatalf("expected run show output to include task detail blocks\noutput:\n%s", output)
	}
	if summaryIndex > firstTaskIndex {
		t.Fatalf("expected summary block before first task block\noutput:\n%s", output)
	}

	expectedSummary := FormatRunSummary(BuildRunSummary(graph))
	actualSummary := output[summaryIndex:firstTaskIndex]
	if strings.TrimSpace(actualSummary) != strings.TrimSpace(expectedSummary) {
		t.Fatalf("summary block mismatch\nactual:\n%s\nexpected:\n%s", actualSummary, expectedSummary)
	}

	reviewIndex := strings.Index(output, "Review Handoff: pending")
	if reviewIndex == -1 || reviewIndex < firstTaskIndex {
		t.Fatalf("expected task-local review detail below summary\noutput:\n%s", output)
	}

	blockerIndex := strings.Index(output, "Blocker: escalated")
	if blockerIndex == -1 || blockerIndex < firstTaskIndex {
		t.Fatalf("expected task-local blocker detail below summary\noutput:\n%s", output)
	}
}

func TestRunShowIncludesTaskLocalBlockerBlock(t *testing.T) {
	t.Parallel()

	fixture := seedReviewHandoffFixture(t)
	blocker := createEscalatedBlockerCase(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask, escalatedBlockerOptions{})

	graph, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	output := FormatRunGraph(graph)
	requiredSnippets := []string{
		"Task: " + string(fixture.sourceTask.TaskID),
		"Blocker: escalated",
		"Current Owner: " + string(blocker.CurrentOwner),
		"Next Action: " + string(blocker.SelectedAction),
		"Reroutes: 1/2",
		"Recommended Action: " + string(blocker.RecommendedAction.Kind),
		"Reason: " + blocker.Reason,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected formatted run graph to contain %q\noutput:\n%s", snippet, output)
		}
	}
	if strings.Contains(output, "\nBlockers:\n") {
		t.Fatalf("expected blocker visibility to stay task-local\noutput:\n%s", output)
	}
}

func TestRunShowIncludesBlockerAndReviewBlocksTogether(t *testing.T) {
	t.Parallel()

	fixture := seedPendingReviewFixture(t)
	blocker := createEscalatedBlockerCase(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask, escalatedBlockerOptions{})

	graph, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	output := FormatRunGraph(graph)
	requiredSnippets := []string{
		"Task: " + string(fixture.sourceTask.TaskID),
		"Review Handoff: pending",
		"Review Task: " + string(fixture.handoff.ReviewTaskID),
		"Blocker: escalated",
		"Current Owner: " + string(blocker.CurrentOwner),
		"Next Action: " + string(blocker.SelectedAction),
		"Reroutes: 1/2",
		"Recommended Action: " + string(blocker.RecommendedAction.Kind),
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected formatted run graph to contain %q\noutput:\n%s", snippet, output)
		}
	}
	if strings.Contains(output, "\nBlockers:\n") {
		t.Fatalf("expected blocker visibility to stay task-local\noutput:\n%s", output)
	}
}

func TestRunShowIncludesPartialReplanSourceAndReplacementLineage(t *testing.T) {
	t.Parallel()

	fixture := seedPartialReplanLineageFixture(t)

	graph, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	output := FormatRunGraph(graph)
	requiredSnippets := []string{
		"Task: " + string(fixture.sourceTask.TaskID),
		"Partial Replan:",
		"Superseded Task: " + string(fixture.sourceTask.TaskID),
		"Replacement Task: " + string(fixture.replacementTask.TaskID),
		"Replacement Owner: " + string(fixture.replacementTask.Owner),
		"Replan Reason: " + fixture.replan.Reason,
		"Task: " + string(fixture.replacementTask.TaskID),
		"Replan Source: " + string(fixture.sourceTask.TaskID),
		"Supersedes: " + string(fixture.sourceTask.TaskID),
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected formatted run graph to contain %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestLoadRunGraphRejectsBrokenPartialReplanLinks(t *testing.T) {
	t.Parallel()

	fixture := seedPartialReplanLineageFixture(t)
	mutatePartialReplanDocument(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask.TaskID, func(replanDoc map[string]any) {
		replanDoc["replacement_message_id"] = string(protocol.NewMessageID(999))
	})

	_, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
	if err == nil {
		t.Fatalf("expected broken partial replan links to fail")
	}
	if !strings.Contains(err.Error(), "coordinator artifact mismatch") {
		t.Fatalf("expected coordinator artifact mismatch, got %v", err)
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

func TestLoadRunGraphRejectsBrokenBlockerLinks(t *testing.T) {
	t.Parallel()

	t.Run("source message mismatch", func(t *testing.T) {
		t.Parallel()

		fixture := seedReviewHandoffFixture(t)
		createEscalatedBlockerCase(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask, escalatedBlockerOptions{})
		mutateBlockerCaseDocument(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask.TaskID, func(caseDoc map[string]any) {
			caseDoc["source_message_id"] = "msg_999999999999"
		})

		_, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
		if err == nil {
			t.Fatalf("expected blocker source message mismatch to fail")
		}
		if !strings.Contains(err.Error(), "coordinator artifact mismatch") {
			t.Fatalf("expected mismatch error, got %v", err)
		}
	})

	t.Run("current owner mismatch", func(t *testing.T) {
		t.Parallel()

		fixture := seedReviewHandoffFixture(t)
		createEscalatedBlockerCase(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask, escalatedBlockerOptions{})
		mutateBlockerCaseDocument(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask.TaskID, func(caseDoc map[string]any) {
			caseDoc["current_owner"] = "reviewer"
		})

		_, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
		if err == nil {
			t.Fatalf("expected blocker current owner mismatch to fail")
		}
		if !strings.Contains(err.Error(), "coordinator artifact mismatch") {
			t.Fatalf("expected mismatch error, got %v", err)
		}
	})
}

func TestLoadRunGraphRejectsBrokenReviewHandoffLinks(t *testing.T) {
	t.Parallel()

	t.Run("missing response message", func(t *testing.T) {
		t.Parallel()

		fixture := seedRespondedReviewFixture(t)
		mutateReviewHandoffDocument(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask.TaskID, func(handoffDoc map[string]any) {
			handoffDoc["response_message_id"] = "msg_999999999999"
		})

		_, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
		if err == nil {
			t.Fatalf("expected missing response message to fail")
		}
		if !strings.Contains(err.Error(), "coordinator artifact mismatch") {
			t.Fatalf("expected mismatch error, got %v", err)
		}
	})

	t.Run("reviewer mismatch", func(t *testing.T) {
		t.Parallel()

		fixture := seedPendingReviewFixture(t)
		mutateReviewHandoffDocument(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask.TaskID, func(handoffDoc map[string]any) {
			handoffDoc["reviewer"] = "backend-high"
		})

		_, err := LoadRunGraph(fixture.cfg.Session.StateDir, fixture.run.RunID)
		if err == nil {
			t.Fatalf("expected reviewer mismatch to fail")
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

type partialReplanLineageFixture struct {
	blockerPolicyFixture
	replacementTask *protocol.ChildTask
	replan          *protocol.PartialReplan
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

func seedPartialReplanLineageFixture(t *testing.T) partialReplanLineageFixture {
	t.Helper()

	fixture := seedBlockerPolicyFixture(t, 1)
	store := mailbox.NewStore(fixture.cfg.Session.StateDir)
	coordinatorStore := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir)

	replacementTask, err := AddChildTask(fixture.cfg, store, ChildTaskRequest{
		ParentRunID:    fixture.run.RunID,
		Owner:          "backend-low",
		Goal:           "Continue the blocked source task through a bounded replacement",
		ExpectedOutput: "replacement task preserves source-task-local lineage",
		ReviewRequired: fixture.sourceTask.ReviewRequired,
	})
	if err != nil {
		t.Fatalf("add replacement task: %v", err)
	}
	writeTaskState(t, fixture.cfg.Session.StateDir, string(replacementTask.Owner), replacementTask.MessageID, replacementTask.ThreadID, protocol.FolderStateUnread, "idle")

	blocker := createEscalatedBlockerCase(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask, escalatedBlockerOptions{})
	if err := coordinatorStore.UpdateBlockerCase(fixture.run.RunID, fixture.sourceTask.TaskID, func(existing *protocol.BlockerCase) error {
		now := time.Now().UTC()
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
		t.Fatalf("UpdateBlockerCase() unexpected error: %v", err)
	}

	now := time.Now().UTC()
	replan := &protocol.PartialReplan{
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
		Reason:               "replace the blocked work with one bounded follow-up task",
		Status:               protocol.PartialReplanStatusApplied,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := coordinatorStore.CreatePartialReplan(replan); err != nil {
		t.Fatalf("CreatePartialReplan() unexpected error: %v", err)
	}

	return partialReplanLineageFixture{
		blockerPolicyFixture: fixture,
		replacementTask:      replacementTask,
		replan:               replan,
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

func mutateChildTaskDocument(t *testing.T, stateDir string, runID protocol.RunID, taskID protocol.TaskID, mutate func(taskDoc map[string]any)) {
	t.Helper()

	path := mailbox.RunTaskPath(stateDir, runID, taskID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read task yaml: %v", err)
	}

	taskDoc := make(map[string]any)
	if err := yaml.Unmarshal(data, &taskDoc); err != nil {
		t.Fatalf("unmarshal task yaml document: %v", err)
	}

	mutate(taskDoc)

	updated, err := yaml.Marshal(taskDoc)
	if err != nil {
		t.Fatalf("marshal task yaml document: %v", err)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatalf("write task yaml document: %v", err)
	}
}

func mutateReviewHandoffDocument(t *testing.T, stateDir string, runID protocol.RunID, sourceTaskID protocol.TaskID, mutate func(handoffDoc map[string]any)) {
	t.Helper()

	path := mailbox.RunReviewHandoffPath(stateDir, runID, sourceTaskID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read review handoff yaml: %v", err)
	}

	handoffDoc := make(map[string]any)
	if err := yaml.Unmarshal(data, &handoffDoc); err != nil {
		t.Fatalf("unmarshal review handoff yaml document: %v", err)
	}

	mutate(handoffDoc)

	updated, err := yaml.Marshal(handoffDoc)
	if err != nil {
		t.Fatalf("marshal review handoff yaml document: %v", err)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatalf("write review handoff yaml: %v", err)
	}
}

func seedRespondedReviewFixture(t *testing.T) pendingReviewFixture {
	t.Helper()

	fixture := seedPendingReviewFixture(t)
	store := mailbox.NewStore(fixture.cfg.Session.StateDir)
	responseID, err := ReviewRespond(
		fixture.cfg.Session.StateDir,
		store,
		"reviewer",
		fixture.handoff.ReviewMessageID,
		protocol.ReviewOutcomeApproved,
		[]byte("approved with no changes\n"),
	)
	if err != nil {
		t.Fatalf("review respond: %v", err)
	}

	updated, err := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir).ReadReviewHandoff(fixture.run.RunID, fixture.sourceTask.TaskID)
	if err != nil {
		t.Fatalf("read updated review handoff: %v", err)
	}
	if updated.ResponseMessageID != responseID {
		t.Fatalf("response message id = %q, want %q", updated.ResponseMessageID, responseID)
	}
	fixture.handoff = updated

	return fixture
}

type escalatedBlockerOptions struct {
	currentTask    *protocol.ChildTask
	currentOwner   protocol.AgentName
	currentMessage protocol.MessageID
}

func createEscalatedBlockerCase(t *testing.T, stateDir string, runID protocol.RunID, sourceTask *protocol.ChildTask, opts escalatedBlockerOptions) *protocol.BlockerCase {
	t.Helper()

	currentTaskID := sourceTask.TaskID
	currentMessageID := sourceTask.MessageID
	currentOwner := sourceTask.Owner
	if opts.currentTask != nil {
		currentTaskID = opts.currentTask.TaskID
		currentMessageID = opts.currentTask.MessageID
		currentOwner = opts.currentTask.Owner
	}
	if opts.currentMessage != "" {
		currentMessageID = opts.currentMessage
	}
	if opts.currentOwner != "" {
		currentOwner = opts.currentOwner
	}

	now := time.Date(2026, time.April, 6, 9, 30, 0, 0, time.UTC)
	blocker := &protocol.BlockerCase{
		RunID:            runID,
		SourceTaskID:     sourceTask.TaskID,
		SourceMessageID:  sourceTask.MessageID,
		SourceOwner:      sourceTask.Owner,
		CurrentTaskID:    currentTaskID,
		CurrentMessageID: currentMessageID,
		CurrentOwner:     currentOwner,
		DeclaredState:    "block",
		BlockKind:        protocol.BlockKindHumanDecision,
		Reason:           "Need operator decision before proceeding",
		SelectedAction:   protocol.BlockerActionEscalate,
		Status:           protocol.BlockerStatusEscalated,
		RerouteCount:     1,
		MaxReroutes:      2,
		RecommendedAction: &protocol.RecommendedAction{
			Kind: protocol.BlockerResolutionActionClarify,
			Note: "Clarify the missing product constraint",
		},
		CreatedAt:   now,
		UpdatedAt:   now,
		EscalatedAt: &now,
	}

	if err := mailbox.NewCoordinatorStore(stateDir).CreateBlockerCase(blocker); err != nil {
		t.Fatalf("create blocker case: %v", err)
	}

	return blocker
}

func mutateBlockerCaseDocument(t *testing.T, stateDir string, runID protocol.RunID, sourceTaskID protocol.TaskID, mutate func(caseDoc map[string]any)) {
	t.Helper()

	path := mailbox.RunBlockerCasePath(stateDir, runID, sourceTaskID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read blocker case yaml: %v", err)
	}

	caseDoc := make(map[string]any)
	if err := yaml.Unmarshal(data, &caseDoc); err != nil {
		t.Fatalf("unmarshal blocker case yaml document: %v", err)
	}

	mutate(caseDoc)

	updated, err := yaml.Marshal(caseDoc)
	if err != nil {
		t.Fatalf("marshal blocker case yaml document: %v", err)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatalf("write blocker case yaml: %v", err)
	}
}

func mutatePartialReplanDocument(t *testing.T, stateDir string, runID protocol.RunID, sourceTaskID protocol.TaskID, mutate func(replanDoc map[string]any)) {
	t.Helper()

	path := mailbox.RunPartialReplanPath(stateDir, runID, sourceTaskID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read partial replan yaml: %v", err)
	}

	replanDoc := make(map[string]any)
	if err := yaml.Unmarshal(data, &replanDoc); err != nil {
		t.Fatalf("unmarshal partial replan yaml document: %v", err)
	}

	mutate(replanDoc)

	updated, err := yaml.Marshal(replanDoc)
	if err != nil {
		t.Fatalf("marshal partial replan yaml document: %v", err)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatalf("write partial replan yaml: %v", err)
	}
}
