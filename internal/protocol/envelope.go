package protocol

import "time"

const (
	MessageSchemaV1 = "tmuxicate/message/v1"
	BodyFormatMD    = "markdown"
)

type Kind string

const (
	KindTask           Kind = "task"
	KindQuestion       Kind = "question"
	KindReviewRequest  Kind = "review_request"
	KindReviewResponse Kind = "review_response"
	KindDecision       Kind = "decision"
	KindStatusRequest  Kind = "status_request"
	KindStatusResponse Kind = "status_response"
	KindNote           Kind = "note"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

type Budget struct {
	MaxTurns  int        `yaml:"max_turns,omitempty"`
	MaxLines  int        `yaml:"max_lines,omitempty"`
	RespondBy *time.Time `yaml:"respond_by,omitempty"`
}

type Attachment struct {
	Path      string `yaml:"path"`
	MediaType string `yaml:"media_type"`
	SHA256    string `yaml:"sha256"`
}

type Envelope struct {
	Schema        string            `yaml:"schema"`
	ID            MessageID         `yaml:"id"`
	Seq           int64             `yaml:"seq"`
	Session       string            `yaml:"session"`
	Thread        ThreadID          `yaml:"thread"`
	Kind          Kind              `yaml:"kind"`
	From          AgentName         `yaml:"from"`
	To            []AgentName       `yaml:"to"`
	CreatedAt     time.Time         `yaml:"created_at"`
	BodyFormat    string            `yaml:"body_format"`
	BodySHA256    string            `yaml:"body_sha256"`
	BodyBytes     int64             `yaml:"body_bytes"`
	ReplyTo       *MessageID        `yaml:"reply_to,omitempty"`
	Subject       string            `yaml:"subject,omitempty"`
	Priority      Priority          `yaml:"priority,omitempty"`
	RequiresAck   bool              `yaml:"requires_ack,omitempty"`
	RequiresClaim bool              `yaml:"requires_claim,omitempty"`
	DeliverAfter  *time.Time        `yaml:"deliver_after,omitempty"`
	ExpiresAt     *time.Time        `yaml:"expires_at,omitempty"`
	Budget        *Budget           `yaml:"budget,omitempty"`
	Attachments   []Attachment      `yaml:"attachments,omitempty"`
	Meta          map[string]string `yaml:"meta,omitempty"`
}
