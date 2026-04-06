package session

import (
	"strings"
	"testing"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestBuildRunSummaryDerivesStatusBucketsAndReferences(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		buildGraph func(t *testing.T) (*RunGraph, protocol.TaskID, verifySummaryItem)
		wantStatus RunSummaryStatus
	}{
		{
			name: "escalated takes precedence over pending review",
			buildGraph: func(t *testing.T) (*RunGraph, protocol.TaskID, verifySummaryItem) {
				t.Helper()

				fixture := seedPendingReviewFixture(t)
				blocker := createEscalatedBlockerCase(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask, escalatedBlockerOptions{})
				graph := mustLoadRunGraph(t, fixture.cfg.Session.StateDir, fixture.run.RunID)

				return graph, fixture.sourceTask.TaskID, func(t *testing.T, item RunSummaryItem) {
					t.Helper()

					if item.BlockerNextAction != blocker.SelectedAction {
						t.Fatalf("blocker next action = %q, want %q", item.BlockerNextAction, blocker.SelectedAction)
					}
					if item.BlockerRecommendedAction == nil || item.BlockerRecommendedAction.Kind != blocker.RecommendedAction.Kind {
						t.Fatalf("recommended action = %#v, want kind %q", item.BlockerRecommendedAction, blocker.RecommendedAction.Kind)
					}
					if item.ReviewTaskID != fixture.handoff.ReviewTaskID {
						t.Fatalf("review task id = %q, want %q", item.ReviewTaskID, fixture.handoff.ReviewTaskID)
					}
					if item.Owner != fixture.handoff.Reviewer {
						t.Fatalf("owner = %q, want %q", item.Owner, fixture.handoff.Reviewer)
					}
				}
			},
			wantStatus: RunSummaryStatusEscalated,
		},
		{
			name: "blocked maps active block state",
			buildGraph: func(t *testing.T) (*RunGraph, protocol.TaskID, verifySummaryItem) {
				t.Helper()

				fixture := seedReviewHandoffFixture(t)
				createEscalatedBlockerCase(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask, escalatedBlockerOptions{})
				mutateBlockerCaseDocument(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask.TaskID, func(caseDoc map[string]any) {
					caseDoc["status"] = "active"
					caseDoc["declared_state"] = "block"
				})
				graph := mustLoadRunGraph(t, fixture.cfg.Session.StateDir, fixture.run.RunID)

				return graph, fixture.sourceTask.TaskID, func(t *testing.T, item RunSummaryItem) {
					t.Helper()

					if item.Owner != fixture.sourceTask.Owner {
						t.Fatalf("owner = %q, want %q", item.Owner, fixture.sourceTask.Owner)
					}
					if item.CurrentTaskID != fixture.sourceTask.TaskID {
						t.Fatalf("current task id = %q, want %q", item.CurrentTaskID, fixture.sourceTask.TaskID)
					}
				}
			},
			wantStatus: RunSummaryStatusBlocked,
		},
		{
			name: "waiting stays distinct from blocked and pending",
			buildGraph: func(t *testing.T) (*RunGraph, protocol.TaskID, verifySummaryItem) {
				t.Helper()

				fixture := seedReviewHandoffFixture(t)
				createEscalatedBlockerCase(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask, escalatedBlockerOptions{})
				mutateBlockerCaseDocument(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask.TaskID, func(caseDoc map[string]any) {
					caseDoc["status"] = "active"
					caseDoc["declared_state"] = "wait"
					delete(caseDoc, "block_kind")
					caseDoc["wait_kind"] = string(protocol.WaitKindExternalEvent)
					caseDoc["selected_action"] = string(protocol.BlockerActionWatch)
				})
				graph := mustLoadRunGraph(t, fixture.cfg.Session.StateDir, fixture.run.RunID)

				return graph, fixture.sourceTask.TaskID, func(t *testing.T, item RunSummaryItem) {
					t.Helper()

					if item.BlockerNextAction != protocol.BlockerActionWatch {
						t.Fatalf("blocker next action = %q, want %q", item.BlockerNextAction, protocol.BlockerActionWatch)
					}
				}
			},
			wantStatus: RunSummaryStatusWaiting,
		},
		{
			name: "pending review stays under review",
			buildGraph: func(t *testing.T) (*RunGraph, protocol.TaskID, verifySummaryItem) {
				t.Helper()

				fixture := seedPendingReviewFixture(t)
				graph := mustLoadRunGraph(t, fixture.cfg.Session.StateDir, fixture.run.RunID)

				return graph, fixture.sourceTask.TaskID, func(t *testing.T, item RunSummaryItem) {
					t.Helper()

					if item.ReviewTaskID != fixture.handoff.ReviewTaskID {
						t.Fatalf("review task id = %q, want %q", item.ReviewTaskID, fixture.handoff.ReviewTaskID)
					}
					if item.ReviewMessageID != fixture.handoff.ReviewMessageID {
						t.Fatalf("review message id = %q, want %q", item.ReviewMessageID, fixture.handoff.ReviewMessageID)
					}
					if item.Owner != fixture.handoff.Reviewer {
						t.Fatalf("owner = %q, want %q", item.Owner, fixture.handoff.Reviewer)
					}
				}
			},
			wantStatus: RunSummaryStatusUnderReview,
		},
		{
			name: "changes requested stays under review",
			buildGraph: func(t *testing.T) (*RunGraph, protocol.TaskID, verifySummaryItem) {
				t.Helper()

				fixture := seedPendingReviewFixture(t)
				store := mailbox.NewStore(fixture.cfg.Session.StateDir)
				responseID, err := ReviewRespond(
					fixture.cfg.Session.StateDir,
					store,
					"reviewer",
					fixture.handoff.ReviewMessageID,
					protocol.ReviewOutcomeChangesRequested,
					[]byte("needs more work\n"),
				)
				if err != nil {
					t.Fatalf("review respond: %v", err)
				}
				graph := mustLoadRunGraph(t, fixture.cfg.Session.StateDir, fixture.run.RunID)

				return graph, fixture.sourceTask.TaskID, func(t *testing.T, item RunSummaryItem) {
					t.Helper()

					if item.ResponseMessageID != responseID {
						t.Fatalf("response message id = %q, want %q", item.ResponseMessageID, responseID)
					}
					if item.ReviewOutcome != protocol.ReviewOutcomeChangesRequested {
						t.Fatalf("review outcome = %q, want %q", item.ReviewOutcome, protocol.ReviewOutcomeChangesRequested)
					}
				}
			},
			wantStatus: RunSummaryStatusUnderReview,
		},
		{
			name: "handoff failed falls back to pending",
			buildGraph: func(t *testing.T) (*RunGraph, protocol.TaskID, verifySummaryItem) {
				t.Helper()

				fixture := seedPendingReviewFixture(t)
				mutateReviewHandoffDocument(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask.TaskID, func(handoffDoc map[string]any) {
					handoffDoc["status"] = string(protocol.ReviewHandoffStatusHandoffFailed)
					handoffDoc["failure_summary"] = "review routing failed after task completion"
				})
				graph := mustLoadRunGraph(t, fixture.cfg.Session.StateDir, fixture.run.RunID)

				return graph, fixture.sourceTask.TaskID, func(t *testing.T, item RunSummaryItem) {
					t.Helper()

					if item.ReviewFailureSummary != "review routing failed after task completion" {
						t.Fatalf("review failure summary = %q", item.ReviewFailureSummary)
					}
				}
			},
			wantStatus: RunSummaryStatusPending,
		},
		{
			name: "approved review resolves to completed",
			buildGraph: func(t *testing.T) (*RunGraph, protocol.TaskID, verifySummaryItem) {
				t.Helper()

				fixture := seedRespondedReviewFixture(t)
				graph := mustLoadRunGraph(t, fixture.cfg.Session.StateDir, fixture.run.RunID)

				return graph, fixture.sourceTask.TaskID, func(t *testing.T, item RunSummaryItem) {
					t.Helper()

					if item.ReviewOutcome != protocol.ReviewOutcomeApproved {
						t.Fatalf("review outcome = %q, want %q", item.ReviewOutcome, protocol.ReviewOutcomeApproved)
					}
					if item.ResponseMessageID != fixture.handoff.ResponseMessageID {
						t.Fatalf("response message id = %q, want %q", item.ResponseMessageID, fixture.handoff.ResponseMessageID)
					}
				}
			},
			wantStatus: RunSummaryStatusCompleted,
		},
		{
			name: "active source task falls back to pending",
			buildGraph: func(t *testing.T) (*RunGraph, protocol.TaskID, verifySummaryItem) {
				t.Helper()

				fixture := seedReviewHandoffFixture(t)
				graph := mustLoadRunGraph(t, fixture.cfg.Session.StateDir, fixture.run.RunID)

				return graph, fixture.sourceTask.TaskID, func(t *testing.T, item RunSummaryItem) {
					t.Helper()

					if item.Owner != fixture.sourceTask.Owner {
						t.Fatalf("owner = %q, want %q", item.Owner, fixture.sourceTask.Owner)
					}
				}
			},
			wantStatus: RunSummaryStatusPending,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			graph, sourceTaskID, verify := tc.buildGraph(t)
			summary := BuildRunSummary(graph)
			item := mustFindSummaryItem(t, summary, sourceTaskID)

			if item.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q", item.Status, tc.wantStatus)
			}
			if item.SourceTaskID != sourceTaskID {
				t.Fatalf("source task id = %q, want %q", item.SourceTaskID, sourceTaskID)
			}
			if item.SourceMessageID == "" {
				t.Fatalf("source message id should not be empty")
			}
			if strings.TrimSpace(item.SourceGoal) == "" {
				t.Fatalf("source goal should not be empty")
			}

			verify(t, item)
		})
	}
}

func TestBuildRunSummaryCollapsesReviewAndRerouteArtifactsIntoSourceRows(t *testing.T) {
	t.Parallel()

	fixture := seedBlockerPolicyFixture(t, 2)
	if err := callTaskBlockForPolicy(
		fixture.cfg.Session.StateDir,
		string(fixture.sourceTask.Owner),
		fixture.sourceTask.MessageID,
		protocol.BlockKindRerouteNeeded,
		"reroute this implementation task to a teammate",
	); err != nil {
		t.Fatalf("task block: %v", err)
	}

	coordinatorStore := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir)
	blockerCase, err := coordinatorStore.ReadBlockerCase(fixture.run.RunID, fixture.sourceTask.TaskID)
	if err != nil {
		t.Fatalf("read blocker case: %v", err)
	}
	if blockerCase.CurrentTaskID == fixture.sourceTask.TaskID {
		t.Fatalf("expected rerouted current task, got source task %q", blockerCase.CurrentTaskID)
	}

	currentTask, err := coordinatorStore.ReadTask(fixture.run.RunID, blockerCase.CurrentTaskID)
	if err != nil {
		t.Fatalf("read rerouted task: %v", err)
	}
	if _, err := ReadMsg(fixture.cfg.Session.StateDir, string(currentTask.Owner), currentTask.MessageID); err != nil {
		t.Fatalf("activate rerouted task: %v", err)
	}
	if err := TaskDone(fixture.cfg.Session.StateDir, string(currentTask.Owner), currentTask.MessageID, "rerouted implementation complete"); err != nil {
		t.Fatalf("complete rerouted task: %v", err)
	}

	handoff, err := coordinatorStore.ReadReviewHandoff(fixture.run.RunID, currentTask.TaskID)
	if err != nil {
		t.Fatalf("read rerouted review handoff: %v", err)
	}
	mutateBlockerCaseDocument(t, fixture.cfg.Session.StateDir, fixture.run.RunID, fixture.sourceTask.TaskID, func(caseDoc map[string]any) {
		caseDoc["status"] = string(protocol.BlockerStatusResolved)
		caseDoc["updated_at"] = "2026-04-06T10:00:00Z"
		caseDoc["resolved_at"] = "2026-04-06T10:00:00Z"
		caseDoc["resolution"] = map[string]any{
			"action":     string(protocol.BlockerResolutionActionManualReroute),
			"created_at": "2026-04-06T10:00:00Z",
		}
	})

	graph := mustLoadRunGraph(t, fixture.cfg.Session.StateDir, fixture.run.RunID)
	summary := BuildRunSummary(graph)

	if len(summary.Items) != 1 {
		t.Fatalf("summary item count = %d, want 1 logical source row", len(summary.Items))
	}

	item := mustFindSummaryItem(t, summary, fixture.sourceTask.TaskID)
	if item.Status != RunSummaryStatusUnderReview {
		t.Fatalf("status = %q, want %q", item.Status, RunSummaryStatusUnderReview)
	}
	if item.CurrentTaskID != currentTask.TaskID {
		t.Fatalf("current task id = %q, want %q", item.CurrentTaskID, currentTask.TaskID)
	}
	if item.CurrentMessageID != currentTask.MessageID {
		t.Fatalf("current message id = %q, want %q", item.CurrentMessageID, currentTask.MessageID)
	}
	if item.CurrentOwner != currentTask.Owner {
		t.Fatalf("current owner = %q, want %q", item.CurrentOwner, currentTask.Owner)
	}
	if item.Owner != handoff.Reviewer {
		t.Fatalf("effective owner = %q, want %q", item.Owner, handoff.Reviewer)
	}
	if item.ReviewTaskID != handoff.ReviewTaskID {
		t.Fatalf("review task id = %q, want %q", item.ReviewTaskID, handoff.ReviewTaskID)
	}
	if item.ReviewMessageID != handoff.ReviewMessageID {
		t.Fatalf("review message id = %q, want %q", item.ReviewMessageID, handoff.ReviewMessageID)
	}
	if hasSummaryItem(summary, currentTask.TaskID) {
		t.Fatalf("rerouted current task %q should fold into source row, not render separately", currentTask.TaskID)
	}
	if hasSummaryItem(summary, handoff.ReviewTaskID) {
		t.Fatalf("review task %q should fold into source row, not render separately", handoff.ReviewTaskID)
	}
}

func TestFormatRunSummaryGroupsItemsWithoutTaskDetailSprawl(t *testing.T) {
	t.Parallel()

	summary := &RunSummary{
		RunID: "run_20260406_0001",
		Items: []RunSummaryItem{
			{
				Status:            RunSummaryStatusEscalated,
				SourceTaskID:      "task_001",
				SourceMessageID:   "msg_001",
				Owner:             "pm",
				SourceGoal:        "Decide whether the blocker needs operator escalation",
				CurrentTaskID:     "task_009",
				CurrentMessageID:  "msg_009",
				BlockerNextAction: protocol.BlockerActionEscalate,
				BlockerRecommendedAction: &protocol.RecommendedAction{
					Kind: protocol.BlockerResolutionActionClarify,
					Note: "Clarify the missing constraint",
				},
			},
			{
				Status:               RunSummaryStatusUnderReview,
				SourceTaskID:         "task_002",
				SourceMessageID:      "msg_002",
				Owner:                "reviewer",
				SourceGoal:           "Review the rerouted run-summary implementation",
				ReviewTaskID:         "task_102",
				ReviewMessageID:      "msg_102",
				ResponseMessageID:    "msg_103",
				ReviewOutcome:        protocol.ReviewOutcomeChangesRequested,
				ReviewFailureSummary: "review routing had to be retried once",
			},
			{
				Status:            RunSummaryStatusCompleted,
				SourceTaskID:      "task_003",
				SourceMessageID:   "msg_003",
				Owner:             "reviewer",
				SourceGoal:        "Approve the final summary output",
				ReviewTaskID:      "task_103",
				ReviewMessageID:   "msg_104",
				ResponseMessageID: "msg_105",
				ReviewOutcome:     protocol.ReviewOutcomeApproved,
			},
		},
	}

	output := FormatRunSummary(summary)
	if !strings.HasPrefix(output, "Summary:\n") {
		t.Fatalf("summary output must start with Summary header\noutput:\n%s", output)
	}

	escalatedIndex := strings.Index(output, "Escalated (1)")
	underReviewIndex := strings.Index(output, "Under Review (1)")
	completedIndex := strings.Index(output, "Completed (1)")
	if escalatedIndex == -1 || underReviewIndex == -1 || completedIndex == -1 {
		t.Fatalf("expected grouped bucket headers in output\noutput:\n%s", output)
	}
	if !(escalatedIndex < underReviewIndex && underReviewIndex < completedIndex) {
		t.Fatalf("expected stable bucket ordering\noutput:\n%s", output)
	}
	if strings.Contains(output, "Blocked (") || strings.Contains(output, "Waiting (") || strings.Contains(output, "Pending (") {
		t.Fatalf("expected empty buckets to be omitted\noutput:\n%s", output)
	}

	requiredSnippets := []string{
		"task=task_001",
		"msg=msg_001",
		"current=task_009/msg_009",
		"review=task_102/msg_102",
		"response=msg_103",
		"review outcome=changes_requested",
		"failure=review routing had to be retried once",
		"recommended action=clarify (Clarify the missing constraint)",
		"next action=escalate",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected summary output to contain %q\noutput:\n%s", snippet, output)
		}
	}

	forbiddenSnippets := []string{
		"Expected Output:",
		"Depends On:",
		"Routing Decision:",
		"Review Handoff:",
		"Blocker:",
	}
	for _, snippet := range forbiddenSnippets {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected summary output to stay shorter than task detail and omit %q\noutput:\n%s", snippet, output)
		}
	}
}

type verifySummaryItem func(t *testing.T, item RunSummaryItem)

func mustLoadRunGraph(t *testing.T, stateDir string, runID protocol.RunID) *RunGraph {
	t.Helper()

	graph, err := LoadRunGraph(stateDir, runID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	return graph
}

func mustFindSummaryItem(t *testing.T, summary *RunSummary, sourceTaskID protocol.TaskID) RunSummaryItem {
	t.Helper()

	if summary == nil {
		t.Fatalf("summary should not be nil")
	}

	for _, item := range summary.Items {
		if item.SourceTaskID == sourceTaskID {
			return item
		}
	}

	t.Fatalf("summary item for source task %q not found", sourceTaskID)
	return RunSummaryItem{}
}

func hasSummaryItem(summary *RunSummary, sourceTaskID protocol.TaskID) bool {
	if summary == nil {
		return false
	}

	for _, item := range summary.Items {
		if item.SourceTaskID == sourceTaskID {
			return true
		}
	}

	return false
}
