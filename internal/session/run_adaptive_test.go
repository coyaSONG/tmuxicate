package session

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"gopkg.in/yaml.v3"
)

func TestBuildAdaptiveRoutingPreferencesAggregatesPriorRunEvidence(t *testing.T) {
	t.Parallel()

	cfg := testAdaptiveRoutingConfig(t)
	store := mailbox.NewStore(cfg.Session.StateDir)

	firstRun, firstTask := seedAdaptiveCompletedRun(t, cfg, store, adaptiveCompletedRunOptions{
		goal:           "Complete protocol-heavy implementation work",
		taskGoal:       "Ship the protocol/session change",
		expectedOutput: "completed implementation work for adaptive scoring",
		reviewOutcome:  protocol.ReviewOutcomeApproved,
	})
	secondRun, secondTask := seedAdaptiveCompletedRun(t, cfg, store, adaptiveCompletedRunOptions{
		goal:           "Complete the same routing shape again",
		taskGoal:       "Ship the second protocol/session change",
		expectedOutput: "second completed implementation work for adaptive scoring",
	})

	preferences, err := BuildAdaptiveRoutingPreferences(cfg, cfg.Session.StateDir, protocol.AgentName("pm"))
	if err != nil {
		t.Fatalf("BuildAdaptiveRoutingPreferences() unexpected error: %v", err)
	}

	if preferences.Coordinator != "pm" {
		t.Fatalf("preferences.Coordinator = %q, want %q", preferences.Coordinator, "pm")
	}
	if preferences.LookbackRuns != cfg.Routing.Adaptive.LookbackRuns {
		t.Fatalf("preferences.LookbackRuns = %d, want %d", preferences.LookbackRuns, cfg.Routing.Adaptive.LookbackRuns)
	}
	if len(preferences.Preferences) != 1 {
		t.Fatalf("preferences.Preferences = %d, want 1", len(preferences.Preferences))
	}

	preference := preferences.Preferences[0]
	if preference.PreferenceKey != "implementation|protocol,session|backend-steady" {
		t.Fatalf("preference.PreferenceKey = %q, want %q", preference.PreferenceKey, "implementation|protocol,session|backend-steady")
	}
	if preference.TaskClass != protocol.TaskClassImplementation {
		t.Fatalf("preference.TaskClass = %q, want %q", preference.TaskClass, protocol.TaskClassImplementation)
	}
	if got, want := preference.NormalizedDomains, []string{"protocol", "session"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("preference.NormalizedDomains = %#v, want %#v", got, want)
	}
	if preference.PreferredOwner != "backend-steady" {
		t.Fatalf("preference.PreferredOwner = %q, want %q", preference.PreferredOwner, "backend-steady")
	}
	if preference.HistoricalScore != 11 {
		t.Fatalf("preference.HistoricalScore = %d, want 11", preference.HistoricalScore)
	}
	if preference.ManualWeight != 2 {
		t.Fatalf("preference.ManualWeight = %d, want 2", preference.ManualWeight)
	}
	if preference.TotalScore != 13 {
		t.Fatalf("preference.TotalScore = %d, want 13", preference.TotalScore)
	}
	if len(preference.Evidence) != 3 {
		t.Fatalf("preference.Evidence = %d, want 3", len(preference.Evidence))
	}

	wantEvidence := []struct {
		runID        protocol.RunID
		sourceTaskID protocol.TaskID
		status       string
		noteContains string
	}{
		{runID: firstRun.RunID, sourceTaskID: firstTask.TaskID, status: "completed", noteContains: "completed"},
		{runID: firstRun.RunID, sourceTaskID: firstTask.TaskID, status: "approved", noteContains: "approved"},
		{runID: secondRun.RunID, sourceTaskID: secondTask.TaskID, status: "completed", noteContains: "completed"},
	}
	for index, want := range wantEvidence {
		evidence := preference.Evidence[index]
		if evidence.RunID != want.runID {
			t.Fatalf("evidence[%d].RunID = %q, want %q", index, evidence.RunID, want.runID)
		}
		if evidence.SourceTaskID != want.sourceTaskID {
			t.Fatalf("evidence[%d].SourceTaskID = %q, want %q", index, evidence.SourceTaskID, want.sourceTaskID)
		}
		if evidence.MessageID == "" {
			t.Fatalf("evidence[%d].MessageID should not be empty", index)
		}
		if evidence.Status != want.status {
			t.Fatalf("evidence[%d].Status = %q, want %q", index, evidence.Status, want.status)
		}
		if !strings.Contains(evidence.Note, want.noteContains) {
			t.Fatalf("evidence[%d].Note = %q, want substring %q", index, evidence.Note, want.noteContains)
		}
	}
}

func TestBuildAdaptiveRoutingPreferencesRespectsLookbackWindow(t *testing.T) {
	t.Parallel()

	cfg := testAdaptiveRoutingConfig(t)
	cfg.Routing.Adaptive.LookbackRuns = 2
	store := mailbox.NewStore(cfg.Session.StateDir)

	oldestRun, _ := seedAdaptiveCompletedRun(t, cfg, store, adaptiveCompletedRunOptions{
		goal:           "Oldest run should fall out of the window",
		taskGoal:       "oldest task",
		expectedOutput: "oldest completed work",
	})
	middleRun, _ := seedAdaptiveCompletedRun(t, cfg, store, adaptiveCompletedRunOptions{
		goal:           "Middle run should remain",
		taskGoal:       "middle task",
		expectedOutput: "middle completed work",
	})
	newestRun, _ := seedAdaptiveCompletedRun(t, cfg, store, adaptiveCompletedRunOptions{
		goal:           "Newest run should remain",
		taskGoal:       "newest task",
		expectedOutput: "newest completed work",
	})

	preferences, err := BuildAdaptiveRoutingPreferences(cfg, cfg.Session.StateDir, protocol.AgentName("pm"))
	if err != nil {
		t.Fatalf("BuildAdaptiveRoutingPreferences() unexpected error: %v", err)
	}
	if len(preferences.Preferences) != 1 {
		t.Fatalf("preferences.Preferences = %d, want 1", len(preferences.Preferences))
	}

	preference := preferences.Preferences[0]
	if preference.HistoricalScore != 8 {
		t.Fatalf("preference.HistoricalScore = %d, want 8", preference.HistoricalScore)
	}

	for _, evidence := range preference.Evidence {
		if evidence.RunID == oldestRun.RunID {
			t.Fatalf("oldest run %s should not contribute evidence inside lookback window", oldestRun.RunID)
		}
	}

	gotRunIDs := []protocol.RunID{
		preference.Evidence[0].RunID,
		preference.Evidence[1].RunID,
	}
	wantRunIDs := []protocol.RunID{middleRun.RunID, newestRun.RunID}
	if !reflect.DeepEqual(gotRunIDs, wantRunIDs) {
		t.Fatalf("evidence run ids = %#v, want %#v", gotRunIDs, wantRunIDs)
	}
}

type adaptiveCompletedRunOptions struct {
	goal           string
	taskGoal       string
	expectedOutput string
	reviewOutcome  protocol.ReviewOutcome
}

func seedAdaptiveCompletedRun(t *testing.T, cfg *config.ResolvedConfig, store *mailbox.Store, opts adaptiveCompletedRunOptions) (*protocol.CoordinatorRun, *protocol.ChildTask) {
	t.Helper()

	run, err := Run(cfg, store, RunRequest{
		Goal:        opts.goal,
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	task, err := AddChildTask(cfg, store, ChildTaskRequest{
		ParentRunID:       run.RunID,
		Owner:             "backend-steady",
		Goal:              opts.taskGoal,
		ExpectedOutput:    opts.expectedOutput,
		ReviewRequired:    opts.reviewOutcome != "",
		TaskClass:         protocol.TaskClassImplementation,
		Domains:           []string{"session", "protocol"},
		NormalizedDomains: []string{"protocol", "session"},
		DuplicateKey:      string(run.RunID) + "|implementation|protocol,session",
		RoutingDecision: protocol.RoutingDecision{
			Status:          "selected",
			SelectedOwner:   "backend-steady",
			Candidates:      []protocol.AgentName{"backend-fast", "backend-steady"},
			TieBreak:        "route_priority desc, config_order asc",
			DuplicateStatus: "unique",
		},
	})
	if err != nil {
		t.Fatalf("add child task: %v", err)
	}

	if _, err := ReadMsg(cfg.Session.StateDir, string(task.Owner), task.MessageID); err != nil {
		t.Fatalf("activate task: %v", err)
	}
	if err := TaskDone(cfg.Session.StateDir, string(task.Owner), task.MessageID, "completed adaptive routing source task"); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	if opts.reviewOutcome != "" {
		handoff, err := mailbox.NewCoordinatorStore(cfg.Session.StateDir).ReadReviewHandoff(run.RunID, task.TaskID)
		if err != nil {
			t.Fatalf("read review handoff: %v", err)
		}
		if _, err := ReadMsg(cfg.Session.StateDir, string(handoff.Reviewer), handoff.ReviewMessageID); err != nil {
			t.Fatalf("activate review task: %v", err)
		}
		if _, err := ReviewRespond(
			cfg.Session.StateDir,
			store,
			string(handoff.Reviewer),
			handoff.ReviewMessageID,
			opts.reviewOutcome,
			[]byte("approved adaptive routing output\n"),
		); err != nil {
			t.Fatalf("review respond: %v", err)
		}
	}

	if _, err := ReadMsg(cfg.Session.StateDir, string(run.Coordinator), run.RootMessageID); err != nil {
		t.Fatalf("activate root message: %v", err)
	}
	if err := TaskDone(cfg.Session.StateDir, string(run.Coordinator), run.RootMessageID, "completed coordinator run for adaptive preference rebuild"); err != nil {
		t.Fatalf("complete root task: %v", err)
	}

	return run, task
}

func testAdaptiveRoutingConfig(t *testing.T) *config.ResolvedConfig {
	t.Helper()

	cfg := testRouteTaskConfig(t)
	cfg.Routing.Adaptive = config.AdaptiveRoutingConfig{
		Enabled:                 true,
		LookbackRuns:            3,
		SuccessWeight:           4,
		ApprovalWeight:          3,
		ChangesRequestedPenalty: 2,
		BlockedPenalty:          5,
		WaitPenalty:             1,
		ManualPreferences: []config.AdaptiveManualPreference{
			{
				TaskClass:      protocol.TaskClassImplementation,
				Domains:        []string{"protocol", "session"},
				PreferredOwner: "backend-steady",
				Weight:         2,
				Reason:         "Keeps protocol-heavy work on the same owner when the signal is explicit",
			},
		},
	}
	cfg.Agents = []config.AgentConfig{
		{Name: "pm", Alias: "lead", Adapter: "generic", Command: "fake-agent", Role: config.RoleSpec{Kind: string(protocol.TaskClassResearch), Domains: []string{"routing"}, Description: "Coordinates routing work"}, Pane: config.PaneConfig{Slot: "main"}, Teammates: []string{"backend-fast", "backend-steady", "reviewer"}},
		{Name: "backend-fast", Alias: "api-fast", Adapter: "generic", Command: "fake-agent", RoutePriority: 30, Role: config.RoleSpec{Kind: string(protocol.TaskClassImplementation), Domains: []string{"protocol", "session"}, Description: "Fast implementation owner"}, Pane: config.PaneConfig{Slot: "right-top"}, Teammates: []string{"pm", "reviewer"}},
		{Name: "backend-steady", Alias: "api-steady", Adapter: "generic", Command: "fake-agent", RoutePriority: 20, Role: config.RoleSpec{Kind: string(protocol.TaskClassImplementation), Domains: []string{"protocol", "session"}, Description: "Steady implementation owner"}, Pane: config.PaneConfig{Slot: "right-bottom"}, Teammates: []string{"pm", "reviewer"}},
		{Name: "reviewer", Alias: "qa", Adapter: "generic", Command: "fake-agent", RoutePriority: 10, Role: config.RoleSpec{Kind: string(protocol.TaskClassReview), Domains: []string{"protocol", "session"}, Description: "Review owner"}, Pane: config.PaneConfig{Slot: "bottom"}, Teammates: []string{"pm", "backend-fast", "backend-steady"}},
	}
	writeAdaptiveResolvedConfig(t, cfg)

	return cfg
}

func writeAdaptiveResolvedConfig(t *testing.T, cfg *config.ResolvedConfig) {
	t.Helper()

	if err := os.MkdirAll(cfg.Session.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.MkdirAll(cfg.Session.Workspace, 0o755); err != nil {
		t.Fatalf("create workspace dir: %v", err)
	}

	data, err := yaml.Marshal(&cfg.Config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Session.StateDir, "config.resolved.yaml"), data, 0o644); err != nil {
		t.Fatalf("write config.resolved.yaml: %v", err)
	}
}
