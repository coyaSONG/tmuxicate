package session

import (
	"fmt"
	"strings"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

type RunSummaryStatus string

const (
	RunSummaryStatusEscalated   RunSummaryStatus = "escalated"
	RunSummaryStatusBlocked     RunSummaryStatus = "blocked"
	RunSummaryStatusWaiting     RunSummaryStatus = "waiting"
	RunSummaryStatusUnderReview RunSummaryStatus = "under_review"
	RunSummaryStatusPending     RunSummaryStatus = "pending"
	RunSummaryStatusCompleted   RunSummaryStatus = "completed"
)

type RunSummary struct {
	RunID protocol.RunID
	Items []RunSummaryItem
}

type RunSummaryItem struct {
	Status RunSummaryStatus

	SourceTaskID    protocol.TaskID
	SourceMessageID protocol.MessageID
	SourceOwner     protocol.AgentName
	Owner           protocol.AgentName
	CurrentOwner    protocol.AgentName
	SourceGoal      string

	CurrentTaskID    protocol.TaskID
	CurrentMessageID protocol.MessageID

	ReviewTaskID         protocol.TaskID
	ReviewMessageID      protocol.MessageID
	ResponseMessageID    protocol.MessageID
	ReviewStatus         protocol.ReviewHandoffStatus
	ReviewOutcome        protocol.ReviewOutcome
	ReviewFailureSummary string

	BlockerStatus            protocol.BlockerStatus
	BlockerNextAction        protocol.BlockerAction
	BlockerRecommendedAction *protocol.RecommendedAction
}

const runSummarySectionLabels = `
Summary:
Escalated (
Blocked (
Waiting (
Under Review (
Pending (
Completed (
`

func BuildRunSummary(graph *RunGraph) *RunSummary {
	if graph == nil {
		return nil
	}

	taskByID := make(map[protocol.TaskID]*RunGraphTask, len(graph.Tasks))
	excludedTaskIDs := make(map[protocol.TaskID]struct{})
	for index := range graph.Tasks {
		task := &graph.Tasks[index]
		taskByID[task.Task.TaskID] = task
		if task.Task.TaskClass == protocol.TaskClassReview {
			excludedTaskIDs[task.Task.TaskID] = struct{}{}
		}
		if task.BlockerCase != nil && task.BlockerCase.CurrentTaskID != "" && task.BlockerCase.CurrentTaskID != task.Task.TaskID {
			excludedTaskIDs[task.BlockerCase.CurrentTaskID] = struct{}{}
		}
		if task.PartialReplan != nil && task.PartialReplan.ReplacementTaskID != "" {
			excludedTaskIDs[task.PartialReplan.ReplacementTaskID] = struct{}{}
		}
	}

	items := make([]RunSummaryItem, 0, len(graph.Tasks))
	for index := range graph.Tasks {
		task := &graph.Tasks[index]
		if _, excluded := excludedTaskIDs[task.Task.TaskID]; excluded {
			continue
		}

		items = append(items, buildRunSummaryItem(task, taskByID))
	}

	return &RunSummary{
		RunID: graph.Run.RunID,
		Items: items,
	}
}

func FormatRunSummary(summary *RunSummary) string {
	if summary == nil {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("Summary:\n")

	statusOrder := []RunSummaryStatus{
		RunSummaryStatusEscalated,
		RunSummaryStatusBlocked,
		RunSummaryStatusWaiting,
		RunSummaryStatusUnderReview,
		RunSummaryStatusPending,
		RunSummaryStatusCompleted,
	}

	for _, status := range statusOrder {
		items := summaryItemsForStatus(summary.Items, status)
		if len(items) == 0 {
			continue
		}

		fmt.Fprintf(&builder, "%s (%d)\n", summaryBucketTitle(status), len(items))
		for _, item := range items {
			fmt.Fprintf(&builder, "- %s | owner=%s | %s | %s\n",
				item.Status,
				formatSummaryOwner(item),
				normalizeDisplayValue(item.SourceGoal),
				formatSummaryRefs(item),
			)

			if detail := formatSummaryOptionalDetail(item); detail != "" {
				fmt.Fprintf(&builder, "  %s\n", detail)
			}
		}
	}

	return builder.String()
}

func buildRunSummaryItem(sourceTask *RunGraphTask, taskByID map[protocol.TaskID]*RunGraphTask) RunSummaryItem {
	effectiveTask := sourceTask
	if sourceTask.PartialReplan != nil && sourceTask.PartialReplan.ReplacementTaskID != "" {
		if replacementTask, ok := taskByID[sourceTask.PartialReplan.ReplacementTaskID]; ok {
			effectiveTask = replacementTask
		}
	}
	if sourceTask.BlockerCase != nil && sourceTask.BlockerCase.CurrentTaskID != "" && sourceTask.BlockerCase.CurrentTaskID != sourceTask.Task.TaskID {
		if currentTask, ok := taskByID[sourceTask.BlockerCase.CurrentTaskID]; ok {
			effectiveTask = currentTask
		}
	}

	effectiveReview := sourceTask.ReviewHandoff
	if effectiveTask != nil && effectiveTask != sourceTask && effectiveTask.ReviewHandoff != nil {
		effectiveReview = effectiveTask.ReviewHandoff
	}

	item := RunSummaryItem{
		Status:          deriveRunSummaryStatus(sourceTask, effectiveTask, effectiveReview),
		SourceTaskID:    sourceTask.Task.TaskID,
		SourceMessageID: sourceTask.Task.MessageID,
		SourceOwner:     sourceTask.Task.Owner,
		Owner:           sourceTask.Task.Owner,
		SourceGoal:      sourceTask.Task.Goal,
	}

	if effectiveTask != nil {
		item.CurrentTaskID = effectiveTask.Task.TaskID
		item.CurrentMessageID = effectiveTask.Task.MessageID
		item.CurrentOwner = effectiveTask.Task.Owner
		item.Owner = effectiveTask.Task.Owner
	}

	if effectiveReview != nil {
		item.ReviewTaskID = effectiveReview.ReviewTaskID
		item.ReviewMessageID = effectiveReview.ReviewMessageID
		item.ResponseMessageID = effectiveReview.ResponseMessageID
		item.ReviewStatus = effectiveReview.Status
		item.ReviewOutcome = effectiveReview.Outcome
		item.ReviewFailureSummary = effectiveReview.FailureSummary
		if effectiveReview.Reviewer != "" {
			item.Owner = effectiveReview.Reviewer
		}
	}

	if sourceTask.BlockerCase != nil {
		item.BlockerStatus = sourceTask.BlockerCase.Status
		item.BlockerNextAction = sourceTask.BlockerCase.SelectedAction
		item.BlockerRecommendedAction = sourceTask.BlockerCase.RecommendedAction
	}

	return item
}

func deriveRunSummaryStatus(sourceTask *RunGraphTask, effectiveTask *RunGraphTask, effectiveReview *protocol.ReviewHandoff) RunSummaryStatus {
	if sourceTask != nil && sourceTask.BlockerCase != nil {
		switch {
		case sourceTask.BlockerCase.Status == protocol.BlockerStatusEscalated:
			return RunSummaryStatusEscalated
		case sourceTask.BlockerCase.Status == protocol.BlockerStatusActive && sourceTask.BlockerCase.DeclaredState == "block":
			return RunSummaryStatusBlocked
		case sourceTask.BlockerCase.Status == protocol.BlockerStatusActive && sourceTask.BlockerCase.DeclaredState == "wait":
			return RunSummaryStatusWaiting
		}
	}

	if effectiveReview != nil {
		switch effectiveReview.Status {
		case protocol.ReviewHandoffStatusPending:
			// A pending review keeps completed task work under_review.
			return RunSummaryStatusUnderReview
		case protocol.ReviewHandoffStatusResponded:
			if effectiveReview.Outcome == protocol.ReviewOutcomeChangesRequested {
				// changes_requested keeps the logical item under_review until follow-up work lands.
				return RunSummaryStatusUnderReview
			}
		case protocol.ReviewHandoffStatusHandoffFailed:
			// handoff_failed falls back to pending because no review task materialized.
			return RunSummaryStatusPending
		}
	}

	if effectiveTask != nil && effectiveTask.ReceiptState == protocol.FolderStateDone {
		return RunSummaryStatusCompleted
	}

	return RunSummaryStatusPending
}

func summaryItemsForStatus(items []RunSummaryItem, status RunSummaryStatus) []RunSummaryItem {
	filtered := make([]RunSummaryItem, 0, len(items))
	for _, item := range items {
		if item.Status == status {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

func summaryBucketTitle(status RunSummaryStatus) string {
	switch status {
	case RunSummaryStatusEscalated:
		return "Escalated"
	case RunSummaryStatusBlocked:
		return "Blocked"
	case RunSummaryStatusWaiting:
		return "Waiting"
	case RunSummaryStatusUnderReview:
		return "Under Review"
	case RunSummaryStatusPending:
		return "Pending"
	case RunSummaryStatusCompleted:
		return "Completed"
	default:
		return strings.ReplaceAll(string(status), "_", " ")
	}
}

func formatSummaryOwner(item RunSummaryItem) string {
	switch {
	case item.Owner != "":
		return string(item.Owner)
	case item.CurrentOwner != "":
		return string(item.CurrentOwner)
	case item.SourceOwner != "":
		return string(item.SourceOwner)
	default:
		return "-"
	}
}

func formatSummaryRefs(item RunSummaryItem) string {
	refs := []string{
		fmt.Sprintf("task=%s", displayTaskID(item.SourceTaskID)),
		fmt.Sprintf("msg=%s", displayMessageID(item.SourceMessageID)),
	}
	// current task refs stay on the main line.
	if item.CurrentTaskID != "" && (item.CurrentTaskID != item.SourceTaskID || item.CurrentMessageID != item.SourceMessageID) {
		refs = append(refs, fmt.Sprintf("current=%s/%s", displayTaskID(item.CurrentTaskID), displayMessageID(item.CurrentMessageID)))
	}
	if item.ReviewTaskID != "" || item.ReviewMessageID != "" {
		refs = append(refs, fmt.Sprintf("review=%s/%s", displayTaskID(item.ReviewTaskID), displayMessageID(item.ReviewMessageID)))
	}
	if item.ResponseMessageID != "" {
		refs = append(refs, fmt.Sprintf("response=%s", displayMessageID(item.ResponseMessageID)))
	}

	return strings.Join(refs, " ")
}

func formatSummaryOptionalDetail(item RunSummaryItem) string {
	parts := make([]string, 0, 4)
	if item.ReviewOutcome != "" {
		parts = append(parts, fmt.Sprintf("review outcome=%s", item.ReviewOutcome))
	}
	if strings.TrimSpace(item.ReviewFailureSummary) != "" {
		parts = append(parts, fmt.Sprintf("failure=%s", item.ReviewFailureSummary))
	}
	if item.BlockerNextAction != "" {
		parts = append(parts, fmt.Sprintf("next action=%s", item.BlockerNextAction))
	}
	if item.BlockerRecommendedAction != nil {
		parts = append(parts, fmt.Sprintf("recommended action=%s", formatRecommendedAction(item.BlockerRecommendedAction)))
	}
	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " | ")
}
