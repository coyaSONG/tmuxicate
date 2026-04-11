package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"github.com/coyaSONG/tmuxicate/internal/session"
	"gopkg.in/yaml.v3"
)

var stdoutCaptureMu sync.Mutex

func TestBlockerResolveCommandRequiresAction(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"blocker",
		"resolve",
		"run_000000000001",
		"task_000000000001",
		"--state-dir",
		t.TempDir(),
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected blocker resolve without --action to fail")
	}
	if !strings.Contains(err.Error(), `required flag(s) "action" not set`) {
		t.Fatalf("error = %q, want required action flag", err)
	}
}

func TestRunShowCommandPrintsSummaryUnderHeader(t *testing.T) {
	fixture := seedCLISummaryFixture(t)

	output, err := executeRootCommand(t,
		"run",
		"show",
		string(fixture.run.RunID),
		"--config",
		fixture.configPath,
	)
	if err != nil {
		t.Fatalf("run show command: %v", err)
	}

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
		t.Fatalf("expected summary block before the first task block\noutput:\n%s", output)
	}

	expectedSummary := loadExpectedSummary(t, fixture.cfg.Session.StateDir, fixture.run.RunID)
	actualSummary := output[summaryIndex:firstTaskIndex]
	if strings.TrimSpace(actualSummary) != strings.TrimSpace(expectedSummary) {
		t.Fatalf("summary block mismatch\nactual:\n%s\nexpected:\n%s", actualSummary, expectedSummary)
	}

	if goalIndex := strings.LastIndex(output, "Goal: "+fixture.completedTask.Goal); goalIndex == -1 || goalIndex < firstTaskIndex {
		t.Fatalf("expected task-local goal detail below summary\noutput:\n%s", output)
	}
}

func TestTaskDoneCommandPrintsSummaryOnlyForRootRunCompletion(t *testing.T) {
	t.Run("non-root child task prints done only", func(t *testing.T) {
		fixture := seedCLIChildTaskFixture(t)
		if _, err := session.ReadMsg(fixture.cfg.Session.StateDir, string(fixture.childTask.Owner), fixture.childTask.MessageID); err != nil {
			t.Fatalf("activate child task: %v", err)
		}

		output, err := executeRootCommand(t,
			"task",
			"done",
			string(fixture.childTask.MessageID),
			"--state-dir",
			fixture.cfg.Session.StateDir,
			"--agent",
			string(fixture.childTask.Owner),
		)
		if err != nil {
			t.Fatalf("task done command: %v", err)
		}

		if output != "done\n" {
			t.Fatalf("non-root completion output = %q, want %q", output, "done\n")
		}
	})

	t.Run("root task prints done and summary", func(t *testing.T) {
		fixture := seedCLISummaryFixture(t)
		if _, err := session.ReadMsg(fixture.cfg.Session.StateDir, string(fixture.run.Coordinator), fixture.run.RootMessageID); err != nil {
			t.Fatalf("activate root task: %v", err)
		}

		output, err := executeRootCommand(t,
			"task",
			"done",
			string(fixture.run.RootMessageID),
			"--state-dir",
			fixture.cfg.Session.StateDir,
			"--agent",
			string(fixture.run.Coordinator),
		)
		if err != nil {
			t.Fatalf("task done command: %v", err)
		}

		expectedSummary := loadExpectedSummary(t, fixture.cfg.Session.StateDir, fixture.run.RunID)
		if output != "done\n"+expectedSummary {
			t.Fatalf("root completion output mismatch\nactual:\n%s\nexpected:\n%s", output, "done\n"+expectedSummary)
		}
	})
}

func TestTaskDoneRootRefreshesAdaptiveRoutingPreferences(t *testing.T) {
	t.Parallel()

	fixture := seedAdaptiveRoutingCLIFixture(t)
	if _, err := session.ReadMsg(fixture.cfg.Session.StateDir, string(fixture.run.Coordinator), fixture.run.RootMessageID); err != nil {
		t.Fatalf("activate root task: %v", err)
	}

	output, err := executeRootCommand(t,
		"task",
		"done",
		string(fixture.run.RootMessageID),
		"--state-dir",
		fixture.cfg.Session.StateDir,
		"--agent",
		string(fixture.run.Coordinator),
	)
	if err != nil {
		t.Fatalf("task done command: %v", err)
	}
	if !strings.Contains(output, "done\n") {
		t.Fatalf("expected root completion output to include done marker, got %q", output)
	}

	preferencePath := mailbox.AdaptiveRoutingPreferencesPath(fixture.cfg.Session.StateDir, protocol.AgentName("pm"))
	if _, err := os.Stat(preferencePath); err != nil {
		t.Fatalf("expected adaptive preference artifact at %s: %v", preferencePath, err)
	}

	preferences, err := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir).ReadAdaptiveRoutingPreferences(protocol.AgentName("pm"))
	if err != nil {
		t.Fatalf("ReadAdaptiveRoutingPreferences() unexpected error: %v", err)
	}
	if preferences.Coordinator != "pm" {
		t.Fatalf("preferences.Coordinator = %q, want %q", preferences.Coordinator, "pm")
	}
}

func TestTaskDoneChildTaskDoesNotRefreshAdaptiveRoutingPreferences(t *testing.T) {
	t.Parallel()

	fixture := seedAdaptiveRoutingCLIFixture(t)
	if _, err := session.ReadMsg(fixture.cfg.Session.StateDir, string(fixture.childTask.Owner), fixture.childTask.MessageID); err != nil {
		t.Fatalf("activate child task: %v", err)
	}

	if _, err := executeRootCommand(t,
		"task",
		"done",
		string(fixture.childTask.MessageID),
		"--state-dir",
		fixture.cfg.Session.StateDir,
		"--agent",
		string(fixture.childTask.Owner),
	); err != nil {
		t.Fatalf("task done command: %v", err)
	}

	preferencePath := mailbox.AdaptiveRoutingPreferencesPath(fixture.cfg.Session.StateDir, protocol.AgentName("pm"))
	if _, err := os.Stat(preferencePath); !os.IsNotExist(err) {
		t.Fatalf("expected no adaptive preference artifact after child completion, stat err = %v", err)
	}
}

func TestRunRouteTaskCommandPrintsAdaptiveDecisionEvidence(t *testing.T) {
	t.Parallel()

	cfg, configPath := writeCLIConfigFiles(t, testAdaptiveCLIConfig(t))
	store := mailbox.NewStore(cfg.Session.StateDir)
	run, err := session.Run(cfg, store, session.RunRequest{
		Goal:        "Route one implementation task through the CLI with adaptive evidence",
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

	output, err := executeRootCommand(t,
		"run",
		"route-task",
		"--config",
		configPath,
		"--run",
		string(run.RunID),
		"--task-class",
		"implementation",
		"--domain",
		"session",
		"--domain",
		"protocol",
		"--goal",
		"Route with adaptive CLI evidence",
		"--expected-output",
		"selected owner and adaptive explanation are printed",
	)
	if err != nil {
		t.Fatalf("run route-task command: %v", err)
	}

	requiredSnippets := []string{
		"backend-steady",
		"Adaptive Routing:",
		"Adaptive Baseline:",
		"Adaptive Score:",
		"Adaptive Evidence:",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected route-task output to contain %q\noutput:\n%s", snippet, output)
		}
	}
}

type cliSummaryFixture struct {
	cfg           *config.ResolvedConfig
	configPath    string
	run           *protocol.CoordinatorRun
	completedTask *protocol.ChildTask
	pendingTask   *protocol.ChildTask
}

type cliChildTaskFixture struct {
	cfg        *config.ResolvedConfig
	configPath string
	run        *protocol.CoordinatorRun
	childTask  *protocol.ChildTask
}

type adaptiveRoutingCLIFixture struct {
	cfg        *config.ResolvedConfig
	configPath string
	run        *protocol.CoordinatorRun
	childTask  *protocol.ChildTask
}

func seedCLISummaryFixture(t *testing.T) cliSummaryFixture {
	t.Helper()

	cfg, configPath := writeCLIConfigFiles(t, testCLIConfig(t))
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := session.Run(cfg, store, session.RunRequest{
		Goal:        "Show summary output through existing CLI surfaces",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	completedTask, err := session.AddChildTask(cfg, store, session.ChildTaskRequest{
		ParentRunID:    run.RunID,
		Owner:          "backend",
		Goal:           "Finish the completed logical work item",
		ExpectedOutput: "completed work is visible in the derived summary",
	})
	if err != nil {
		t.Fatalf("add completed task: %v", err)
	}

	pendingTask, err := session.AddChildTask(cfg, store, session.ChildTaskRequest{
		ParentRunID:    run.RunID,
		Owner:          "reviewer",
		Goal:           "Leave one logical work item pending",
		ExpectedOutput: "pending work stays visible below the same summary header",
	})
	if err != nil {
		t.Fatalf("add pending task: %v", err)
	}

	if _, err := session.ReadMsg(cfg.Session.StateDir, string(completedTask.Owner), completedTask.MessageID); err != nil {
		t.Fatalf("activate completed task: %v", err)
	}
	if err := session.TaskDone(cfg.Session.StateDir, string(completedTask.Owner), completedTask.MessageID, "completed for CLI summary coverage"); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	return cliSummaryFixture{
		cfg:           cfg,
		configPath:    configPath,
		run:           run,
		completedTask: completedTask,
		pendingTask:   pendingTask,
	}
}

func seedCLIChildTaskFixture(t *testing.T) cliChildTaskFixture {
	t.Helper()

	cfg, configPath := writeCLIConfigFiles(t, testCLIConfig(t))
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := session.Run(cfg, store, session.RunRequest{
		Goal:        "Complete one child task without printing a run summary",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	childTask, err := session.AddChildTask(cfg, store, session.ChildTaskRequest{
		ParentRunID:    run.RunID,
		Owner:          "backend",
		Goal:           "Finish a non-root task",
		ExpectedOutput: "task done still prints the legacy done output",
	})
	if err != nil {
		t.Fatalf("add child task: %v", err)
	}

	return cliChildTaskFixture{
		cfg:        cfg,
		configPath: configPath,
		run:        run,
		childTask:  childTask,
	}
}

func seedAdaptiveRoutingCLIFixture(t *testing.T) adaptiveRoutingCLIFixture {
	t.Helper()

	cfg, configPath := writeCLIConfigFiles(t, testAdaptiveCLIConfig(t))
	store := mailbox.NewStore(cfg.Session.StateDir)

	run, err := session.Run(cfg, store, session.RunRequest{
		Goal:        "Refresh adaptive preferences only from root completion",
		Coordinator: "pm",
		CreatedBy:   "human",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	childTask, err := session.AddChildTask(cfg, store, session.ChildTaskRequest{
		ParentRunID:       run.RunID,
		Owner:             "backend-steady",
		Goal:              "Complete one adaptive implementation task",
		ExpectedOutput:    "task completion feeds later adaptive preference rebuild",
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

	return adaptiveRoutingCLIFixture{
		cfg:        cfg,
		configPath: configPath,
		run:        run,
		childTask:  childTask,
	}
}

func testCLIConfig(t *testing.T) *config.ResolvedConfig {
	t.Helper()

	baseDir := t.TempDir()
	workspaceDir := filepath.Join(baseDir, "workspace")
	stateDir := filepath.Join(baseDir, "state")

	return &config.ResolvedConfig{
		Config: config.Config{
			Version: 1,
			Session: config.SessionConfig{
				Name:       "cli-summary",
				Workspace:  workspaceDir,
				StateDir:   stateDir,
				WindowName: "agents",
				Layout:     "triad",
			},
			Delivery: config.DeliveryConfig{
				Mode:          "notify_then_read",
				AckTimeout:    config.Duration(2 * time.Minute),
				RetryInterval: config.Duration(30 * time.Second),
				MaxRetries:    3,
			},
			Transcript: config.TranscriptConfig{
				Mode: "pipe-pane",
				Dir:  filepath.Join(stateDir, "transcripts"),
			},
			Routing: config.RoutingConfig{
				Coordinator: "pm",
			},
			Defaults: config.DefaultsConfig{
				Workdir: workspaceDir,
				Notify: config.NotifyConfig{
					Enabled: boolPtr(true),
				},
			},
			Agents: []config.AgentConfig{
				{
					Name:    "pm",
					Alias:   "lead",
					Adapter: "generic",
					Command: "fake-agent",
					Role: config.RoleSpec{
						Kind:        string(protocol.TaskClassResearch),
						Domains:     []string{"routing"},
						Description: "Coordinates CLI summary tests",
					},
					Pane:      config.PaneConfig{Slot: "main"},
					Teammates: []string{"backend", "reviewer"},
				},
				{
					Name:          "backend",
					Alias:         "dev",
					Adapter:       "generic",
					Command:       "fake-agent",
					RoutePriority: 20,
					Role: config.RoleSpec{
						Kind:        string(protocol.TaskClassImplementation),
						Domains:     []string{"session"},
						Description: "Owns implementation work in CLI summary tests",
					},
					Pane:      config.PaneConfig{Slot: "right-top"},
					Teammates: []string{"pm", "reviewer"},
				},
				{
					Name:          "reviewer",
					Alias:         "qa",
					Adapter:       "generic",
					Command:       "fake-agent",
					RoutePriority: 10,
					Role: config.RoleSpec{
						Kind:        string(protocol.TaskClassReview),
						Domains:     []string{"session"},
						Description: "Owns review work in CLI summary tests",
					},
					Pane:      config.PaneConfig{Slot: "right-bottom"},
					Teammates: []string{"pm", "backend"},
				},
			},
		},
		ConfigDir: baseDir,
	}
}

func testAdaptiveCLIConfig(t *testing.T) *config.ResolvedConfig {
	t.Helper()

	cfg := testCLIConfig(t)
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
		{
			Name:    "pm",
			Alias:   "lead",
			Adapter: "generic",
			Command: "fake-agent",
			Role: config.RoleSpec{
				Kind:        string(protocol.TaskClassResearch),
				Domains:     []string{"routing"},
				Description: "Coordinates adaptive routing CLI tests",
			},
			Pane:      config.PaneConfig{Slot: "main"},
			Teammates: []string{"backend-fast", "backend-steady", "reviewer"},
		},
		{
			Name:          "backend-fast",
			Alias:         "fast",
			Adapter:       "generic",
			Command:       "fake-agent",
			RoutePriority: 30,
			Role: config.RoleSpec{
				Kind:        string(protocol.TaskClassImplementation),
				Domains:     []string{"protocol", "session"},
				Description: "Fast implementation owner",
			},
			Pane:      config.PaneConfig{Slot: "right-top"},
			Teammates: []string{"pm", "reviewer"},
		},
		{
			Name:          "backend-steady",
			Alias:         "steady",
			Adapter:       "generic",
			Command:       "fake-agent",
			RoutePriority: 20,
			Role: config.RoleSpec{
				Kind:        string(protocol.TaskClassImplementation),
				Domains:     []string{"protocol", "session"},
				Description: "Steady implementation owner",
			},
			Pane:      config.PaneConfig{Slot: "right-bottom"},
			Teammates: []string{"pm", "reviewer"},
		},
		{
			Name:          "reviewer",
			Alias:         "qa",
			Adapter:       "generic",
			Command:       "fake-agent",
			RoutePriority: 10,
			Role: config.RoleSpec{
				Kind:        string(protocol.TaskClassReview),
				Domains:     []string{"protocol", "session"},
				Description: "Review owner",
			},
			Pane:      config.PaneConfig{Slot: "bottom"},
			Teammates: []string{"pm", "backend-fast", "backend-steady"},
		},
	}

	return cfg
}

func writeCLIConfigFiles(t *testing.T, cfg *config.ResolvedConfig) (*config.ResolvedConfig, string) {
	t.Helper()

	if err := os.MkdirAll(cfg.Session.Workspace, 0o755); err != nil {
		t.Fatalf("create workspace dir: %v", err)
	}
	if err := os.MkdirAll(cfg.Session.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	configPath := filepath.Join(cfg.ConfigDir, "tmuxicate.yaml")
	data, err := yaml.Marshal(&cfg.Config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Session.StateDir, "config.resolved.yaml"), data, 0o644); err != nil {
		t.Fatalf("write resolved config file: %v", err)
	}

	cfg.ConfigPath = configPath
	return cfg, configPath
}

func executeRootCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	stdoutCaptureMu.Lock()
	defer stdoutCaptureMu.Unlock()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	defer reader.Close()

	originalStdout := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()

	cmd := newRootCmd()
	cmd.SetOut(writer)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)

	execErr := cmd.Execute()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}

	output, readErr := io.ReadAll(reader)
	if readErr != nil {
		t.Fatalf("read captured output: %v", readErr)
	}

	return string(output), execErr
}

func loadExpectedSummary(t *testing.T, stateDir string, runID protocol.RunID) string {
	t.Helper()

	graph, err := session.LoadRunGraph(stateDir, runID)
	if err != nil {
		t.Fatalf("load run graph: %v", err)
	}

	return session.FormatRunSummary(session.BuildRunSummary(graph))
}

func boolPtr(value bool) *bool {
	return &value
}
