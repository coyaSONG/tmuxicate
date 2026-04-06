package protocol

import (
	"fmt"
	"time"
)

type RunID string

type TaskID string

type TaskClass string

const (
	TaskClassImplementation TaskClass = "implementation"
	TaskClassResearch       TaskClass = "research"
	TaskClassReview         TaskClass = "review"
)

type ReviewOutcome string

const (
	ReviewOutcomeApproved         ReviewOutcome = "approved"
	ReviewOutcomeChangesRequested ReviewOutcome = "changes_requested"
)

type ReviewHandoffStatus string

const (
	ReviewHandoffStatusPending       ReviewHandoffStatus = "pending"
	ReviewHandoffStatusResponded     ReviewHandoffStatus = "responded"
	ReviewHandoffStatusHandoffFailed ReviewHandoffStatus = "handoff_failed"
)

type AgentSnapshot struct {
	Name      AgentName `yaml:"name"`
	Alias     string    `yaml:"alias"`
	Role      string    `yaml:"role"`
	Teammates []string  `yaml:"teammates,omitempty"`
}

type CoordinatorRun struct {
	RunID         RunID           `yaml:"run_id"`
	Goal          string          `yaml:"goal"`
	Coordinator   AgentName       `yaml:"coordinator"`
	CreatedBy     AgentName       `yaml:"created_by"`
	CreatedAt     time.Time       `yaml:"created_at"`
	RootMessageID MessageID       `yaml:"root_message_id"`
	RootThreadID  ThreadID        `yaml:"root_thread_id"`
	AllowedOwners []AgentName     `yaml:"allowed_owners"`
	TeamSnapshot  []AgentSnapshot `yaml:"team_snapshot"`
}

type ChildTask struct {
	TaskID         TaskID    `yaml:"task_id"`
	ParentRunID    RunID     `yaml:"parent_run_id"`
	Owner          AgentName `yaml:"owner"`
	Goal           string    `yaml:"goal"`
	ExpectedOutput string    `yaml:"expected_output"`
	DependsOn      []TaskID  `yaml:"depends_on,omitempty"`
	ReviewRequired bool      `yaml:"review_required"`
	// TaskClass persists as `yaml:"task_class"` when route metadata is present.
	TaskClass TaskClass `yaml:"task_class,omitempty"`
	// Domains persists as `yaml:"domains"` when route metadata is present.
	Domains []string `yaml:"domains,omitempty"`
	// NormalizedDomains persists as `yaml:"normalized_domains"` when route metadata is present.
	NormalizedDomains []string `yaml:"normalized_domains,omitempty"`
	// DuplicateKey persists as `yaml:"duplicate_key"` when route metadata is present.
	DuplicateKey string `yaml:"duplicate_key,omitempty"`
	// RoutingDecision persists as `yaml:"routing_decision"` when route metadata is present.
	RoutingDecision *RoutingDecision `yaml:"routing_decision,omitempty"`
	// OverrideReason persists as `yaml:"override_reason"` when route metadata is present.
	OverrideReason string    `yaml:"override_reason,omitempty"`
	MessageID      MessageID `yaml:"message_id,omitempty"`
	ThreadID       ThreadID  `yaml:"thread_id,omitempty"`
	CreatedAt      time.Time `yaml:"created_at"`
}

type ReviewHandoff struct {
	RunID             RunID               `yaml:"run_id"`
	SourceTaskID      TaskID              `yaml:"source_task_id"`
	SourceMessageID   MessageID           `yaml:"source_message_id"`
	ReviewTaskID      TaskID              `yaml:"review_task_id,omitempty"`
	ReviewMessageID   MessageID           `yaml:"review_message_id,omitempty"`
	Reviewer          AgentName           `yaml:"reviewer,omitempty"`
	Status            ReviewHandoffStatus `yaml:"status"`
	FailureSummary    string              `yaml:"failure_summary,omitempty"`
	ResponseMessageID MessageID           `yaml:"response_message_id,omitempty"`
	Outcome           ReviewOutcome       `yaml:"outcome,omitempty"`
	CreatedAt         time.Time           `yaml:"created_at"`
	RespondedAt       *time.Time          `yaml:"responded_at,omitempty"`
}

type RouteChildTaskRequest struct {
	RunID          RunID     `yaml:"run_id"`
	TaskClass      TaskClass `yaml:"task_class"`
	Domains        []string  `yaml:"domains"`
	Goal           string    `yaml:"goal"`
	ExpectedOutput string    `yaml:"expected_output"`
	ReviewRequired bool      `yaml:"review_required"`
	OwnerOverride  AgentName `yaml:"owner_override,omitempty"`
	OverrideReason string    `yaml:"override_reason,omitempty"`
}

type RoutingDecision struct {
	Status          string      `yaml:"status"`
	SelectedOwner   AgentName   `yaml:"selected_owner,omitempty"`
	Candidates      []AgentName `yaml:"candidates,omitempty"`
	TieBreak        string      `yaml:"tie_break,omitempty"`
	DuplicateStatus string      `yaml:"duplicate_status,omitempty"`
	MatchedTaskID   TaskID      `yaml:"matched_task_id,omitempty"`
	Suggestions     []string    `yaml:"suggestions,omitempty"`
}

type RouteRejection struct {
	TaskClass          TaskClass   `yaml:"task_class"`
	Domains            []string    `yaml:"domains"`
	AllowedOwners      []AgentName `yaml:"allowed_owners"`
	EligibleCandidates []AgentName `yaml:"eligible_candidates"`
	Suggestions        []string    `yaml:"suggestions"`
}

func (r *RouteRejection) Error() string {
	if r == nil {
		return "route rejected"
	}

	return fmt.Sprintf("route rejected for task_class=%q domains=%v", r.TaskClass, r.Domains)
}

func NewRunID(seq int64) RunID {
	return RunID(fmt.Sprintf("run_%012d", seq))
}

func NewTaskID(seq int64) TaskID {
	return TaskID(fmt.Sprintf("task_%012d", seq))
}
