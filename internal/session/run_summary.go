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

	return &RunSummary{
		RunID: graph.Run.RunID,
		Items: nil,
	}
}

func FormatRunSummary(summary *RunSummary) string {
	if summary == nil {
		return ""
	}

	return "Summary:\n"
}
