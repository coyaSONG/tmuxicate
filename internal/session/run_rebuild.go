package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"gopkg.in/yaml.v3"
)

type RunGraph struct {
	Run   protocol.CoordinatorRun
	Tasks []RunGraphTask
}

type RunGraphTask struct {
	Task          protocol.ChildTask
	ReceiptState  protocol.FolderState
	DeclaredState string
	ReviewHandoff *protocol.ReviewHandoff
	BlockerCase   *protocol.BlockerCase
	PartialReplan *protocol.PartialReplan
	ReplanSource  protocol.TaskID
	Supersedes    protocol.TaskID
}

type runMessageSummary struct {
	Thread  protocol.ThreadID
	Kind    protocol.Kind
	ReplyTo *protocol.MessageID
}

func LoadRunGraph(stateDir string, runID protocol.RunID) (*RunGraph, error) {
	store := mailbox.NewCoordinatorStore(stateDir)
	run, err := store.ReadRun(runID)
	if err != nil {
		return nil, err
	}

	messages, err := loadRunMessages(stateDir)
	if err != nil {
		return nil, fmt.Errorf("scan messages: %w", err)
	}

	rootMessage, ok := messages[run.RootMessageID]
	if !ok {
		return nil, coordinatorArtifactMismatch("run %s references missing root message %s", run.RunID, run.RootMessageID)
	}
	if rootMessage.Thread != run.RootThreadID {
		return nil, coordinatorArtifactMismatch("run %s root thread mismatch for message %s", run.RunID, run.RootMessageID)
	}

	tasks, err := loadRunTasks(stateDir, run.RunID)
	if err != nil {
		return nil, err
	}

	graph := &RunGraph{
		Run:   *run,
		Tasks: make([]RunGraphTask, 0, len(tasks)),
	}
	taskByID := make(map[protocol.TaskID]*RunGraphTask, len(tasks))
	for _, task := range tasks {
		if task.ParentRunID != run.RunID {
			return nil, coordinatorArtifactMismatch("task %s belongs to run %s, not %s", task.TaskID, task.ParentRunID, run.RunID)
		}

		message, ok := messages[task.MessageID]
		if !ok {
			return nil, coordinatorArtifactMismatch("task %s references missing message %s", task.TaskID, task.MessageID)
		}
		if message.Thread != task.ThreadID {
			return nil, coordinatorArtifactMismatch("task %s thread mismatch for message %s", task.TaskID, task.MessageID)
		}
		if task.ThreadID != run.RootThreadID {
			return nil, coordinatorArtifactMismatch("task %s thread %s does not match run root thread %s", task.TaskID, task.ThreadID, run.RootThreadID)
		}

		receiptState, err := loadTaskReceiptState(stateDir, string(task.Owner), task.MessageID)
		if err != nil {
			return nil, err
		}

		declaredState, _, err := readDeclaredState(filepath.Join(stateDir, "agents", string(task.Owner), "events", "state.current.json"))
		if err != nil {
			return nil, fmt.Errorf("read declared state for %s: %w", task.Owner, err)
		}

		node := RunGraphTask{
			Task:          task,
			ReceiptState:  receiptState,
			DeclaredState: declaredState,
		}
		graph.Tasks = append(graph.Tasks, node)
		taskByID[task.TaskID] = &graph.Tasks[len(graph.Tasks)-1]
	}

	for _, node := range graph.Tasks {
		for _, dependency := range node.Task.DependsOn {
			if _, ok := taskByID[dependency]; !ok {
				return nil, coordinatorArtifactMismatch("task %s depends on missing task %s", node.Task.TaskID, dependency)
			}
		}
	}

	handoffs, err := loadRunReviewHandoffs(stateDir, run.RunID)
	if err != nil {
		return nil, err
	}
	for _, handoff := range handoffs {
		if handoff.RunID != run.RunID {
			return nil, coordinatorArtifactMismatch("review handoff %s belongs to run %s, not %s", handoff.SourceTaskID, handoff.RunID, run.RunID)
		}

		sourceTask, ok := taskByID[handoff.SourceTaskID]
		if !ok {
			return nil, coordinatorArtifactMismatch("review handoff references missing source task %s", handoff.SourceTaskID)
		}
		if sourceTask.Task.MessageID != handoff.SourceMessageID {
			return nil, coordinatorArtifactMismatch("review handoff source message mismatch for task %s", handoff.SourceTaskID)
		}

		switch handoff.Status {
		case protocol.ReviewHandoffStatusPending, protocol.ReviewHandoffStatusResponded:
			reviewTask, ok := taskByID[handoff.ReviewTaskID]
			if !ok {
				return nil, coordinatorArtifactMismatch("review handoff references missing review task %s", handoff.ReviewTaskID)
			}
			if reviewTask.Task.MessageID != handoff.ReviewMessageID {
				return nil, coordinatorArtifactMismatch("review handoff review message mismatch for task %s", handoff.ReviewTaskID)
			}
			if reviewTask.Task.Owner != handoff.Reviewer {
				return nil, coordinatorArtifactMismatch("review handoff reviewer mismatch for task %s", handoff.ReviewTaskID)
			}
			if reviewTask.Task.TaskClass != protocol.TaskClassReview {
				return nil, coordinatorArtifactMismatch("review handoff review task %s is not a review task", handoff.ReviewTaskID)
			}
		}

		if handoff.Status == protocol.ReviewHandoffStatusResponded {
			responseMessage, ok := messages[handoff.ResponseMessageID]
			if !ok {
				return nil, coordinatorArtifactMismatch("review handoff response message %s is missing", handoff.ResponseMessageID)
			}
			if responseMessage.Kind != protocol.KindReviewResponse {
				return nil, coordinatorArtifactMismatch("review handoff response %s is not a review_response", handoff.ResponseMessageID)
			}
			if responseMessage.ReplyTo == nil || *responseMessage.ReplyTo != handoff.ReviewMessageID {
				return nil, coordinatorArtifactMismatch("review handoff response %s does not reply to %s", handoff.ResponseMessageID, handoff.ReviewMessageID)
			}
			if responseMessage.Thread != run.RootThreadID {
				return nil, coordinatorArtifactMismatch("review handoff response %s thread mismatch", handoff.ResponseMessageID)
			}
		}

		sourceTask.ReviewHandoff = handoff
	}

	blockers, err := loadRunBlockers(stateDir, run.RunID)
	if err != nil {
		return nil, err
	}
	for _, blocker := range blockers {
		if blocker.RunID != run.RunID {
			return nil, coordinatorArtifactMismatch("blocker case %s belongs to run %s, not %s", blocker.SourceTaskID, blocker.RunID, run.RunID)
		}

		sourceTask, ok := taskByID[blocker.SourceTaskID]
		if !ok {
			return nil, coordinatorArtifactMismatch("blocker case references missing source task %s", blocker.SourceTaskID)
		}
		if sourceTask.Task.MessageID != blocker.SourceMessageID {
			return nil, coordinatorArtifactMismatch("blocker case source message mismatch for task %s", blocker.SourceTaskID)
		}

		currentTask := sourceTask
		if blocker.CurrentTaskID != "" {
			currentTask, ok = taskByID[blocker.CurrentTaskID]
			if !ok {
				return nil, coordinatorArtifactMismatch("blocker case references missing current task %s", blocker.CurrentTaskID)
			}
		}
		if blocker.CurrentMessageID != "" && currentTask.Task.MessageID != blocker.CurrentMessageID {
			return nil, coordinatorArtifactMismatch("blocker case current message mismatch for task %s", currentTask.Task.TaskID)
		}
		if blocker.CurrentOwner != "" && currentTask.Task.Owner != blocker.CurrentOwner {
			return nil, coordinatorArtifactMismatch("blocker case current owner mismatch for task %s", currentTask.Task.TaskID)
		}

		if blocker.Resolution != nil {
			var createdTask *RunGraphTask
			if blocker.Resolution.CreatedTaskID != "" {
				createdTask, ok = taskByID[blocker.Resolution.CreatedTaskID]
				if !ok {
					return nil, coordinatorArtifactMismatch("blocker case resolution references missing task %s", blocker.Resolution.CreatedTaskID)
				}
			}
			if blocker.Resolution.CreatedMessageID != "" {
				if _, ok := messages[blocker.Resolution.CreatedMessageID]; !ok {
					return nil, coordinatorArtifactMismatch("blocker case resolution references missing message %s", blocker.Resolution.CreatedMessageID)
				}
				if createdTask != nil && createdTask.Task.MessageID != blocker.Resolution.CreatedMessageID {
					return nil, coordinatorArtifactMismatch("blocker case resolution message mismatch for task %s", blocker.Resolution.CreatedTaskID)
				}
			}
		}

		sourceTask.BlockerCase = blocker
	}

	replans, err := loadRunPartialReplans(stateDir, run.RunID)
	if err != nil {
		return nil, err
	}
	for _, replan := range replans {
		if replan.RunID != run.RunID {
			return nil, coordinatorArtifactMismatch("partial replan %s belongs to run %s, not %s", replan.SourceTaskID, replan.RunID, run.RunID)
		}

		sourceTask, ok := taskByID[replan.SourceTaskID]
		if !ok {
			return nil, coordinatorArtifactMismatch("partial replan references missing source task %s", replan.SourceTaskID)
		}
		if sourceTask.Task.MessageID != replan.SourceMessageID {
			return nil, coordinatorArtifactMismatch("partial replan source message mismatch for task %s", replan.SourceTaskID)
		}

		supersededTask, ok := taskByID[replan.SupersededTaskID]
		if !ok {
			return nil, coordinatorArtifactMismatch("partial replan references missing superseded task %s", replan.SupersededTaskID)
		}
		if supersededTask.Task.MessageID != replan.SupersededMessageID {
			return nil, coordinatorArtifactMismatch("partial replan superseded message mismatch for task %s", replan.SupersededTaskID)
		}

		replacementTask, ok := taskByID[replan.ReplacementTaskID]
		if !ok {
			return nil, coordinatorArtifactMismatch("partial replan references missing replacement task %s", replan.ReplacementTaskID)
		}
		if replacementTask.Task.MessageID != replan.ReplacementMessageID {
			return nil, coordinatorArtifactMismatch("partial replan replacement message mismatch for task %s", replan.ReplacementTaskID)
		}

		resolvedReplan, err := store.FindPartialReplanByReplacementTaskID(run.RunID, replan.ReplacementTaskID)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, coordinatorArtifactMismatch("partial replan replacement lookup missing for task %s", replan.ReplacementTaskID)
			}
			return nil, err
		}
		if resolvedReplan.SourceTaskID != replan.SourceTaskID {
			return nil, coordinatorArtifactMismatch("partial replan replacement lookup mismatch for source task %s", replan.SourceTaskID)
		}

		if sourceTask.BlockerCase == nil {
			return nil, coordinatorArtifactMismatch("partial replan source task %s is missing blocker case", replan.SourceTaskID)
		}
		if sourceTask.BlockerCase.Resolution == nil {
			return nil, coordinatorArtifactMismatch("partial replan source task %s is missing blocker resolution", replan.SourceTaskID)
		}
		if sourceTask.BlockerCase.Resolution.Action != protocol.BlockerResolutionActionPartialReplan {
			return nil, coordinatorArtifactMismatch("partial replan source task %s blocker resolution is not partial_replan", replan.SourceTaskID)
		}
		if sourceTask.BlockerCase.Resolution.CreatedTaskID != replan.ReplacementTaskID {
			return nil, coordinatorArtifactMismatch("partial replan replacement task mismatch for source task %s", replan.SourceTaskID)
		}
		if sourceTask.BlockerCase.Resolution.CreatedMessageID != replan.ReplacementMessageID {
			return nil, coordinatorArtifactMismatch("partial replan replacement message mismatch for source task %s", replan.SourceTaskID)
		}

		sourceTask.PartialReplan = replan
		replacementTask.ReplanSource = replan.SourceTaskID
		replacementTask.Supersedes = replan.SupersededTaskID
	}

	sort.Slice(graph.Tasks, func(i, j int) bool {
		return graph.Tasks[i].Task.TaskID < graph.Tasks[j].Task.TaskID
	})

	return graph, nil
}

func FormatRunGraph(graph *RunGraph) string {
	if graph == nil {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Run: %s\n", graph.Run.RunID)
	fmt.Fprintf(&builder, "Coordinator: %s\n", graph.Run.Coordinator)
	fmt.Fprintf(&builder, "Goal: %s\n", graph.Run.Goal)
	fmt.Fprintf(&builder, "Root Message: %s\n", graph.Run.RootMessageID)
	if summary := FormatRunSummary(BuildRunSummary(graph)); strings.Contains(summary, "Summary:") {
		builder.WriteString(summary)
	}
	for _, task := range graph.Tasks {
		fmt.Fprintf(&builder, "\nTask: %s\n", task.Task.TaskID)
		fmt.Fprintf(&builder, "Owner: %s\n", task.Task.Owner)
		if task.Task.TaskClass != "" {
			fmt.Fprintf(&builder, "Task Class: %s\n", task.Task.TaskClass)
		}
		if domains := formatTaskDomains(task.Task); domains != "" {
			fmt.Fprintf(&builder, "Domains: %s\n", domains)
		}
		if strings.TrimSpace(task.Task.DuplicateKey) != "" {
			fmt.Fprintf(&builder, "Duplicate Key: %s\n", task.Task.DuplicateKey)
		}
		if task.Task.RoutingDecision != nil {
			fmt.Fprintf(&builder, "Routing Decision: %s\n", formatRoutingDecision(task.Task.RoutingDecision))
			if candidates := formatRoutingCandidates(task.Task.RoutingDecision.Candidates); candidates != "" {
				fmt.Fprintf(&builder, "Candidates: %s\n", candidates)
			}
			if adaptive := task.Task.RoutingDecision.Adaptive; adaptive != nil && adaptive.Applied {
				fmt.Fprintf(&builder, "Adaptive Routing: %s\n", adaptive.Reason)
				fmt.Fprintf(&builder, "Adaptive Baseline: %s\n", normalizeDisplayValue(string(adaptive.BaselineOwner)))
				fmt.Fprintf(&builder, "Adaptive Score: historical=%d manual=%d total=%d\n", adaptive.HistoricalScore, adaptive.ManualWeight, adaptive.TotalScore)
				fmt.Fprintf(&builder, "Adaptive Evidence: %s\n", formatAdaptiveRoutingEvidence(adaptive.Evidence))
			}
		}
		if strings.TrimSpace(task.Task.OverrideReason) != "" {
			fmt.Fprintf(&builder, "Override Reason: %s\n", task.Task.OverrideReason)
		}
		fmt.Fprintf(&builder, "Goal: %s\n", task.Task.Goal)
		fmt.Fprintf(&builder, "Expected Output: %s\n", task.Task.ExpectedOutput)
		fmt.Fprintf(&builder, "Depends On: %s\n", formatDependsOn(task.Task.DependsOn))
		fmt.Fprintf(&builder, "State: %s [%s]\n", normalizeDisplayValue(task.DeclaredState), normalizeDisplayValue(string(task.ReceiptState)))
		fmt.Fprintf(&builder, "Message: %s\n", task.Task.MessageID)
		if task.ReviewHandoff != nil {
			fmt.Fprintf(&builder, "Review Handoff: %s\n", task.ReviewHandoff.Status)
			fmt.Fprintf(&builder, "Review Task: %s\n", displayTaskID(task.ReviewHandoff.ReviewTaskID))
			fmt.Fprintf(&builder, "Reviewer: %s\n", normalizeDisplayValue(string(task.ReviewHandoff.Reviewer)))
			fmt.Fprintf(&builder, "Response: %s\n", displayMessageID(task.ReviewHandoff.ResponseMessageID))
			fmt.Fprintf(&builder, "Outcome: %s\n", normalizeDisplayValue(string(task.ReviewHandoff.Outcome)))
			fmt.Fprintf(&builder, "Failure: %s\n", normalizeDisplayValue(task.ReviewHandoff.FailureSummary))
		}
		if task.BlockerCase != nil {
			fmt.Fprintf(&builder, "Blocker: %s\n", normalizeDisplayValue(string(task.BlockerCase.Status)))
			if task.BlockerCase.CurrentTaskID != "" {
				fmt.Fprintf(&builder, "Current Task: %s\n", displayTaskID(task.BlockerCase.CurrentTaskID))
			}
			if task.BlockerCase.CurrentOwner != "" {
				fmt.Fprintf(&builder, "Current Owner: %s\n", normalizeDisplayValue(string(task.BlockerCase.CurrentOwner)))
			}
			if task.BlockerCase.CurrentMessageID != "" {
				fmt.Fprintf(&builder, "Current Message: %s\n", displayMessageID(task.BlockerCase.CurrentMessageID))
			}
			if task.BlockerCase.DeclaredState != "" {
				fmt.Fprintf(&builder, "Declared State: %s\n", normalizeDisplayValue(task.BlockerCase.DeclaredState))
			}
			if task.BlockerCase.WaitKind != "" {
				fmt.Fprintf(&builder, "Wait Kind: %s\n", normalizeDisplayValue(string(task.BlockerCase.WaitKind)))
			}
			if task.BlockerCase.BlockKind != "" {
				fmt.Fprintf(&builder, "Block Kind: %s\n", normalizeDisplayValue(string(task.BlockerCase.BlockKind)))
			}
			if task.BlockerCase.Reason != "" {
				fmt.Fprintf(&builder, "Reason: %s\n", normalizeDisplayValue(task.BlockerCase.Reason))
			}
			fmt.Fprintf(&builder, "Next Action: %s\n", normalizeDisplayValue(string(task.BlockerCase.SelectedAction)))
			fmt.Fprintf(&builder, "Reroutes: %s\n", formatBlockerReroutes(task.BlockerCase))
			if task.BlockerCase.RecommendedAction != nil {
				fmt.Fprintf(&builder, "Recommended Action: %s\n", formatRecommendedAction(task.BlockerCase.RecommendedAction))
			}
			if task.BlockerCase.Resolution != nil {
				fmt.Fprintf(&builder, "Resolution: %s\n", formatBlockerResolution(task.BlockerCase.Resolution))
			}
		}
		if task.PartialReplan != nil {
			fmt.Fprintf(&builder, "Partial Replan: %s\n", normalizeDisplayValue(string(task.PartialReplan.Status)))
			fmt.Fprintf(&builder, "Superseded Task: %s\n", displayTaskID(task.PartialReplan.SupersededTaskID))
			fmt.Fprintf(&builder, "Replacement Task: %s\n", displayTaskID(task.PartialReplan.ReplacementTaskID))
			fmt.Fprintf(&builder, "Replacement Owner: %s\n", normalizeDisplayValue(string(task.PartialReplan.ReplacementOwner)))
			fmt.Fprintf(&builder, "Replan Reason: %s\n", normalizeDisplayValue(task.PartialReplan.Reason))
		}
		if task.ReplanSource != "" {
			fmt.Fprintf(&builder, "Replan Source: %s\n", displayTaskID(task.ReplanSource))
			fmt.Fprintf(&builder, "Supersedes: %s\n", displayTaskID(task.Supersedes))
		}
	}

	return builder.String()
}

func loadRunMessages(stateDir string) (map[protocol.MessageID]runMessageSummary, error) {
	messages := make(map[protocol.MessageID]runMessageSummary)
	if err := scanMessages(stateDir, func(path string) error {
		data, err := os.ReadFile(filepath.Join(path, "envelope.yaml"))
		if err != nil {
			return err
		}

		var envelope struct {
			ID      protocol.MessageID  `yaml:"id"`
			Thread  protocol.ThreadID   `yaml:"thread"`
			Kind    protocol.Kind       `yaml:"kind"`
			ReplyTo *protocol.MessageID `yaml:"reply_to"`
		}
		if err := yaml.Unmarshal(data, &envelope); err != nil {
			return err
		}
		if envelope.ID == "" {
			return nil
		}

		messages[envelope.ID] = runMessageSummary{
			Thread:  envelope.Thread,
			Kind:    envelope.Kind,
			ReplyTo: envelope.ReplyTo,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return messages, nil
}

func loadRunTasks(stateDir string, runID protocol.RunID) ([]protocol.ChildTask, error) {
	entries, err := os.ReadDir(mailbox.RunTasksDir(stateDir, runID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read run tasks dir: %w", err)
	}

	tasks := make([]protocol.ChildTask, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		path := filepath.Join(mailbox.RunTasksDir(stateDir, runID), entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read task yaml: %w", err)
		}

		var task protocol.ChildTask
		if err := yaml.Unmarshal(data, &task); err != nil {
			return nil, fmt.Errorf("unmarshal task yaml: %w", err)
		}
		if err := task.Validate(); err != nil {
			return nil, fmt.Errorf("validate task yaml: %w", err)
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

func loadTaskReceiptState(stateDir, agent string, msgID protocol.MessageID) (protocol.FolderState, error) {
	var (
		found bool
		state protocol.FolderState
	)

	if err := scanReceiptsForAgent(stateDir, agent, func(folder string, _ string, receipt receiptSummary) {
		if receipt.MessageID != string(msgID) {
			return
		}
		found = true
		state = protocol.FolderState(folder)
	}); err != nil {
		return "", fmt.Errorf("scan receipts for %s: %w", agent, err)
	}

	if !found {
		return "", coordinatorArtifactMismatch("task message %s has no receipt for owner %s", msgID, agent)
	}

	return state, nil
}

func loadRunReviewHandoffs(stateDir string, runID protocol.RunID) (map[protocol.TaskID]*protocol.ReviewHandoff, error) {
	entries, err := os.ReadDir(mailbox.RunReviewsDir(stateDir, runID))
	if err != nil {
		if os.IsNotExist(err) {
			return map[protocol.TaskID]*protocol.ReviewHandoff{}, nil
		}
		return nil, fmt.Errorf("read run reviews dir: %w", err)
	}

	handoffs := make(map[protocol.TaskID]*protocol.ReviewHandoff, len(entries))
	coordinatorStore := mailbox.NewCoordinatorStore(stateDir)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		sourceTaskID := protocol.TaskID(entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))])
		handoff, err := coordinatorStore.ReadReviewHandoff(runID, sourceTaskID)
		if err != nil {
			return nil, err
		}
		handoffs[sourceTaskID] = handoff
	}

	return handoffs, nil
}

func loadRunBlockers(stateDir string, runID protocol.RunID) (map[protocol.TaskID]*protocol.BlockerCase, error) {
	entries, err := os.ReadDir(mailbox.RunBlockersDir(stateDir, runID))
	if err != nil {
		if os.IsNotExist(err) {
			return map[protocol.TaskID]*protocol.BlockerCase{}, nil
		}
		return nil, fmt.Errorf("read run blockers dir: %w", err)
	}

	blockers := make(map[protocol.TaskID]*protocol.BlockerCase, len(entries))
	coordinatorStore := mailbox.NewCoordinatorStore(stateDir)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		sourceTaskID := protocol.TaskID(entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))])
		blocker, err := coordinatorStore.ReadBlockerCase(runID, sourceTaskID)
		if err != nil {
			if strings.Contains(err.Error(), "does not match path") {
				return nil, coordinatorArtifactMismatch("blocker case %s path mismatch", sourceTaskID)
			}
			return nil, err
		}
		blockers[sourceTaskID] = blocker
	}

	return blockers, nil
}

func loadRunPartialReplans(stateDir string, runID protocol.RunID) (map[protocol.TaskID]*protocol.PartialReplan, error) {
	entries, err := os.ReadDir(mailbox.RunPartialReplansDir(stateDir, runID))
	if err != nil {
		if os.IsNotExist(err) {
			return map[protocol.TaskID]*protocol.PartialReplan{}, nil
		}
		return nil, fmt.Errorf("read run partial replans dir: %w", err)
	}

	replans := make(map[protocol.TaskID]*protocol.PartialReplan, len(entries))
	coordinatorStore := mailbox.NewCoordinatorStore(stateDir)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		sourceTaskID := protocol.TaskID(entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))])
		replan, err := coordinatorStore.ReadPartialReplan(runID, sourceTaskID)
		if err != nil {
			if strings.Contains(err.Error(), "does not match path") {
				return nil, coordinatorArtifactMismatch("partial replan %s path mismatch", sourceTaskID)
			}
			return nil, err
		}
		replans[sourceTaskID] = replan
	}

	return replans, nil
}

func coordinatorArtifactMismatch(format string, args ...any) error {
	return fmt.Errorf("coordinator artifact mismatch: "+format, args...)
}

func formatAdaptiveRoutingEvidence(evidence []protocol.AdaptiveRoutingEvidenceRef) string {
	if len(evidence) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(evidence))
	for _, ref := range evidence {
		parts = append(parts, fmt.Sprintf("run=%s task=%s status=%s", ref.RunID, ref.SourceTaskID, ref.Status))
	}

	return strings.Join(parts, "; ")
}

func formatDependsOn(dependsOn []protocol.TaskID) string {
	if len(dependsOn) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(dependsOn))
	for _, dependency := range dependsOn {
		parts = append(parts, string(dependency))
	}

	return strings.Join(parts, ", ")
}

func normalizeDisplayValue(value string) string {
	if strings.TrimSpace(value) == "" || value == "-" {
		return "-"
	}

	return value
}

func formatTaskDomains(task protocol.ChildTask) string {
	values := task.NormalizedDomains
	if len(values) == 0 {
		values = task.Domains
	}
	if len(values) == 0 {
		return ""
	}

	return strings.Join(values, ", ")
}

func formatRoutingDecision(decision *protocol.RoutingDecision) string {
	if decision == nil {
		return ""
	}
	if strings.TrimSpace(string(decision.SelectedOwner)) == "" {
		return normalizeDisplayValue(decision.Status)
	}

	return strings.TrimSpace(fmt.Sprintf("%s %s", decision.Status, decision.SelectedOwner))
}

func formatRoutingCandidates(candidates []protocol.AgentName) string {
	if len(candidates) == 0 {
		return ""
	}

	parts := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		parts = append(parts, string(candidate))
	}

	return strings.Join(parts, ", ")
}

func displayTaskID(taskID protocol.TaskID) string {
	if taskID == "" {
		return "-"
	}

	return string(taskID)
}

func displayMessageID(messageID protocol.MessageID) string {
	if messageID == "" {
		return "-"
	}

	return string(messageID)
}

func formatBlockerReroutes(blocker *protocol.BlockerCase) string {
	if blocker == nil {
		return "-"
	}

	return fmt.Sprintf("%d/%d", blocker.RerouteCount, blocker.MaxReroutes)
}

func formatRecommendedAction(action *protocol.RecommendedAction) string {
	if action == nil {
		return "-"
	}
	if strings.TrimSpace(action.Note) == "" {
		return string(action.Kind)
	}

	return fmt.Sprintf("%s (%s)", action.Kind, action.Note)
}

func formatBlockerResolution(resolution *protocol.BlockerResolution) string {
	if resolution == nil {
		return "-"
	}

	parts := []string{string(resolution.Action)}
	if resolution.CreatedTaskID != "" {
		parts = append(parts, fmt.Sprintf("task=%s", resolution.CreatedTaskID))
	}
	if resolution.CreatedMessageID != "" {
		parts = append(parts, fmt.Sprintf("message=%s", resolution.CreatedMessageID))
	}
	if strings.TrimSpace(resolution.Note) != "" {
		parts = append(parts, resolution.Note)
	}

	return strings.Join(parts, "; ")
}
