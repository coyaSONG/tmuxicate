package protocol

import (
	"errors"
	"fmt"
	"strings"
)

func (e Envelope) Validate() error {
	if e.Schema != MessageSchemaV1 {
		return fmt.Errorf("schema must be %q", MessageSchemaV1)
	}

	if e.ID == "" {
		return errors.New("id is required")
	}
	if e.Seq <= 0 {
		return errors.New("seq must be > 0")
	}
	if e.Session == "" {
		return errors.New("session is required")
	}
	if e.Thread == "" {
		return errors.New("thread is required")
	}
	if !isValidKind(e.Kind) {
		return fmt.Errorf("invalid kind %q", e.Kind)
	}
	if e.From == "" {
		return errors.New("from is required")
	}
	if len(e.To) == 0 {
		return errors.New("to must contain at least one recipient")
	}
	for i, to := range e.To {
		if to == "" {
			return fmt.Errorf("to[%d] is required", i)
		}
	}
	if e.CreatedAt.IsZero() {
		return errors.New("created_at is required")
	}
	if e.BodyFormat != BodyFormatMD {
		return fmt.Errorf("body_format must be %q", BodyFormatMD)
	}
	if !isHexSHA256(e.BodySHA256) {
		return errors.New("body_sha256 must be a 64-character lowercase hex string")
	}
	if e.BodyBytes < 0 {
		return errors.New("body_bytes must be >= 0")
	}
	if e.Priority != "" && !isValidPriority(e.Priority) {
		return fmt.Errorf("invalid priority %q", e.Priority)
	}
	if e.DeliverAfter != nil && e.DeliverAfter.IsZero() {
		return errors.New("deliver_after must not be zero")
	}
	if e.ExpiresAt != nil && e.ExpiresAt.IsZero() {
		return errors.New("expires_at must not be zero")
	}
	if e.DeliverAfter != nil && e.ExpiresAt != nil && e.ExpiresAt.Before(*e.DeliverAfter) {
		return errors.New("expires_at must be >= deliver_after")
	}
	if e.Budget != nil {
		if e.Budget.MaxTurns < 0 {
			return errors.New("budget.max_turns must be >= 0")
		}
		if e.Budget.MaxLines < 0 {
			return errors.New("budget.max_lines must be >= 0")
		}
		if e.Budget.RespondBy != nil && e.Budget.RespondBy.IsZero() {
			return errors.New("budget.respond_by must not be zero")
		}
	}
	for i, att := range e.Attachments {
		if att.Path == "" {
			return fmt.Errorf("attachments[%d].path is required", i)
		}
		if att.MediaType == "" {
			return fmt.Errorf("attachments[%d].media_type is required", i)
		}
		if !isHexSHA256(att.SHA256) {
			return fmt.Errorf("attachments[%d].sha256 must be a 64-character lowercase hex string", i)
		}
	}

	return nil
}

func (r Receipt) Validate() error {
	if r.Schema != ReceiptSchemaV1 {
		return fmt.Errorf("schema must be %q", ReceiptSchemaV1)
	}
	if r.MessageID == "" {
		return errors.New("message_id is required")
	}
	if r.Seq <= 0 {
		return errors.New("seq must be > 0")
	}
	if r.Recipient == "" {
		return errors.New("recipient is required")
	}
	if !isValidFolderState(r.FolderState) {
		return fmt.Errorf("invalid folder_state %q", r.FolderState)
	}
	if r.Revision < 0 {
		return errors.New("revision must be >= 0")
	}
	if r.NotifyAttempts < 0 {
		return errors.New("notify_attempts must be >= 0")
	}
	if r.AckedAt != nil && r.AckedAt.IsZero() {
		return errors.New("acked_at must not be zero")
	}
	if r.ClaimedBy != nil && *r.ClaimedBy == "" {
		return errors.New("claimed_by must not be empty")
	}
	if r.ClaimedAt != nil && r.ClaimedAt.IsZero() {
		return errors.New("claimed_at must not be zero")
	}
	if r.DoneAt != nil && r.DoneAt.IsZero() {
		return errors.New("done_at must not be zero")
	}
	if r.LastNotifiedAt != nil && r.LastNotifiedAt.IsZero() {
		return errors.New("last_notified_at must not be zero")
	}
	if r.NextRetryAt != nil && r.NextRetryAt.IsZero() {
		return errors.New("next_retry_at must not be zero")
	}
	if r.LastError != nil && strings.TrimSpace(*r.LastError) == "" {
		return errors.New("last_error must not be blank")
	}

	switch r.FolderState {
	case FolderStateUnread:
		if r.DoneAt != nil {
			return errors.New("done_at must be nil for unread receipts")
		}
	case FolderStateActive:
		// v0.1 task completion updates done_at before moving the receipt to done.
		// Allow that short-lived intermediate state.
	case FolderStateDone:
		if r.DoneAt == nil {
			return errors.New("done receipts require done_at")
		}
	case FolderStateDead:
		// No extra invariant in v0.1.
	}

	return nil
}

func isValidKind(k Kind) bool {
	switch k {
	case KindTask, KindQuestion, KindReviewRequest, KindReviewResponse, KindDecision, KindStatusRequest, KindStatusResponse, KindNote:
		return true
	default:
		return false
	}
}

func isValidPriority(p Priority) bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
		return true
	default:
		return false
	}
}

func isValidFolderState(s FolderState) bool {
	switch s {
	case FolderStateUnread, FolderStateActive, FolderStateDone, FolderStateDead:
		return true
	default:
		return false
	}
}

func isHexSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}
