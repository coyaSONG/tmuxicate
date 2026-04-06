package session

import "github.com/coyaSONG/tmuxicate/internal/protocol"

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

	return "Summary:\n"
}

func buildRunSummaryItem(sourceTask *RunGraphTask, taskByID map[protocol.TaskID]*RunGraphTask) RunSummaryItem {
	effectiveTask := sourceTask
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
