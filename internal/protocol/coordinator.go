package protocol

import (
	"fmt"
	"time"
)

type RunID string

type TaskID string

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

func NewRunID(seq int64) RunID {
	return RunID(fmt.Sprintf("run_%012d", seq))
}

func NewTaskID(seq int64) TaskID {
	return TaskID(fmt.Sprintf("task_%012d", seq))
}
