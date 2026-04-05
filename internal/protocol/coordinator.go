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
	MessageID      MessageID `yaml:"message_id,omitempty"`
	ThreadID       ThreadID  `yaml:"thread_id,omitempty"`
	CreatedAt      time.Time `yaml:"created_at"`
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
	TaskClass          TaskClass   `yaml:"task_class"`
	Domains            []string    `yaml:"domains"`
	AllowedOwners      []AgentName `yaml:"allowed_owners"`
	EligibleCandidates []AgentName `yaml:"eligible_candidates"`
	SelectedOwner      AgentName   `yaml:"selected_owner"`
	TieBreak           string      `yaml:"tie_break"`
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
