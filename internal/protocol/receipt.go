package protocol

import "time"

const ReceiptSchemaV1 = "tmuxicate/receipt/v1"

type FolderState string

const (
	FolderStateUnread FolderState = "unread"
	FolderStateActive FolderState = "active"
	FolderStateDone   FolderState = "done"
	FolderStateDead   FolderState = "dead"
)

type Receipt struct {
	Schema         string      `yaml:"schema"`
	MessageID      MessageID   `yaml:"message_id"`
	Seq            int64       `yaml:"seq"`
	Recipient      AgentName   `yaml:"recipient"`
	FolderState    FolderState `yaml:"folder_state"`
	Revision       int64       `yaml:"revision"`
	AckedAt        *time.Time  `yaml:"acked_at,omitempty"`
	ClaimedBy      *AgentName  `yaml:"claimed_by,omitempty"`
	ClaimedAt      *time.Time  `yaml:"claimed_at,omitempty"`
	DoneAt         *time.Time  `yaml:"done_at,omitempty"`
	NotifyAttempts int         `yaml:"notify_attempts"`
	LastNotifiedAt *time.Time  `yaml:"last_notified_at,omitempty"`
	NextRetryAt    *time.Time  `yaml:"next_retry_at,omitempty"`
	LastError      *string     `yaml:"last_error,omitempty"`
}
