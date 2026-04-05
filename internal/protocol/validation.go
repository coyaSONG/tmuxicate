package protocol

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

func (e *Envelope) Validate() error {
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

func (r *Receipt) Validate() error {
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

func (r *CoordinatorRun) Validate() error {
	if r == nil {
		return errors.New("run is required")
	}
	if !isGeneratedRunID(r.RunID) {
		return errors.New("run_id must use generated run_ identifier")
	}
	if strings.TrimSpace(r.Goal) == "" {
		return errors.New("goal is required")
	}
	if strings.TrimSpace(string(r.Coordinator)) == "" {
		return errors.New("coordinator is required")
	}
	if strings.TrimSpace(string(r.CreatedBy)) == "" {
		return errors.New("created_by is required")
	}
	if r.CreatedAt.IsZero() {
		return errors.New("created_at is required")
	}
	if strings.TrimSpace(string(r.RootMessageID)) == "" {
		return errors.New("root_message_id is required")
	}
	if strings.TrimSpace(string(r.RootThreadID)) == "" {
		return errors.New("root_thread_id is required")
	}
	if len(r.AllowedOwners) == 0 {
		return errors.New("allowed_owners must contain at least one owner")
	}
	for i, owner := range r.AllowedOwners {
		if strings.TrimSpace(string(owner)) == "" {
			return fmt.Errorf("allowed_owners[%d] is required", i)
		}
	}
	if len(r.TeamSnapshot) == 0 {
		return errors.New("team_snapshot must contain at least one agent")
	}
	for i, snapshot := range r.TeamSnapshot {
		if strings.TrimSpace(string(snapshot.Name)) == "" {
			return fmt.Errorf("team_snapshot[%d].name is required", i)
		}
		if strings.TrimSpace(snapshot.Alias) == "" {
			return fmt.Errorf("team_snapshot[%d].alias is required", i)
		}
		if strings.TrimSpace(snapshot.Role) == "" {
			return fmt.Errorf("team_snapshot[%d].role is required", i)
		}
		for j, teammate := range snapshot.Teammates {
			if strings.TrimSpace(teammate) == "" {
				return fmt.Errorf("team_snapshot[%d].teammates[%d] is required", i, j)
			}
		}
	}

	return nil
}

func (t *ChildTask) Validate() error {
	if t == nil {
		return errors.New("task is required")
	}
	if !isGeneratedTaskID(t.TaskID) {
		return errors.New("task_id must use generated task_ identifier")
	}
	if !isGeneratedRunID(t.ParentRunID) {
		return errors.New("parent_run_id must use generated run_ identifier")
	}
	if strings.TrimSpace(string(t.Owner)) == "" {
		return errors.New("owner is required")
	}
	if strings.TrimSpace(t.Goal) == "" {
		return errors.New("goal is required")
	}
	if strings.TrimSpace(t.ExpectedOutput) == "" {
		return errors.New("expected_output is required")
	}
	for i, dep := range t.DependsOn {
		if !isGeneratedTaskID(dep) {
			return fmt.Errorf("depends_on[%d] must use generated task_ identifier", i)
		}
	}
	if t.MessageID != "" && strings.TrimSpace(string(t.MessageID)) == "" {
		return errors.New("message_id must not be blank")
	}
	if t.ThreadID != "" && strings.TrimSpace(string(t.ThreadID)) == "" {
		return errors.New("thread_id must not be blank")
	}
	if t.CreatedAt.IsZero() {
		return errors.New("created_at is required")
	}

	return nil
}

func (c TaskClass) Validate() error {
	switch c {
	case TaskClassImplementation, TaskClassResearch, TaskClassReview:
		return nil
	default:
		return fmt.Errorf("must be one of %q, %q, or %q", TaskClassImplementation, TaskClassResearch, TaskClassReview)
	}
}

func (r *RouteChildTaskRequest) Validate() error {
	if r == nil {
		return errors.New("route request is required")
	}
	if !isGeneratedRunID(r.RunID) {
		return errors.New("run_id must use generated run_ identifier")
	}
	if err := r.TaskClass.Validate(); err != nil {
		return fmt.Errorf("task_class: %w", err)
	}
	domains, err := NormalizeRouteDomains(r.Domains)
	if err != nil {
		return fmt.Errorf("domains: %w", err)
	}
	if len(domains) == 0 {
		return errors.New("domains must contain at least one domain")
	}
	if strings.TrimSpace(r.Goal) == "" {
		return errors.New("goal is required")
	}
	if strings.TrimSpace(r.ExpectedOutput) == "" {
		return errors.New("expected_output is required")
	}
	if r.OwnerOverride != "" && strings.TrimSpace(r.OverrideReason) == "" {
		return errors.New("override_reason is required when owner_override is set")
	}

	r.Domains = domains
	return nil
}

func (d *RoutingDecision) Validate() error {
	if d == nil {
		return errors.New("routing decision is required")
	}
	if err := d.TaskClass.Validate(); err != nil {
		return fmt.Errorf("task_class: %w", err)
	}
	domains, err := NormalizeRouteDomains(d.Domains)
	if err != nil {
		return fmt.Errorf("domains: %w", err)
	}
	if len(domains) == 0 {
		return errors.New("domains must contain at least one domain")
	}
	if len(d.AllowedOwners) == 0 {
		return errors.New("allowed_owners must contain at least one owner")
	}
	if len(d.EligibleCandidates) == 0 {
		return errors.New("eligible_candidates must contain at least one candidate")
	}
	if strings.TrimSpace(string(d.SelectedOwner)) == "" {
		return errors.New("selected_owner is required")
	}
	if strings.TrimSpace(d.TieBreak) == "" {
		return errors.New("tie_break is required")
	}

	d.Domains = domains
	return nil
}

func (r *RouteRejection) Validate() error {
	if r == nil {
		return errors.New("route rejection is required")
	}
	if err := r.TaskClass.Validate(); err != nil {
		return fmt.Errorf("task_class: %w", err)
	}
	domains, err := NormalizeRouteDomains(r.Domains)
	if err != nil {
		return fmt.Errorf("domains: %w", err)
	}
	if len(domains) == 0 {
		return errors.New("domains must contain at least one domain")
	}
	if len(r.AllowedOwners) == 0 {
		return errors.New("allowed_owners must contain at least one owner")
	}
	if len(r.Suggestions) == 0 {
		return errors.New("suggestions must contain at least one retry hint")
	}

	r.Domains = domains
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

func NormalizeRouteDomains(domains []string) ([]string, error) {
	normalized := make([]string, 0, len(domains))
	seen := make(map[string]struct{}, len(domains))

	for i, domain := range domains {
		value := strings.ToLower(strings.TrimSpace(domain))
		if value == "" {
			return nil, fmt.Errorf("domains[%d] must not be blank", i)
		}
		for _, r := range value {
			switch {
			case r >= 'a' && r <= 'z':
			case r >= '0' && r <= '9':
			case r == '-' || r == '_':
			default:
				return nil, fmt.Errorf("domains[%d] must contain only lowercase letters, digits, hyphen, or underscore", i)
			}
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	if len(normalized) == 0 {
		return nil, nil
	}

	slices.Sort(normalized)
	return normalized, nil
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

func isGeneratedRunID(id RunID) bool {
	return isGeneratedIdentifier(string(id), "run_")
}

func isGeneratedTaskID(id TaskID) bool {
	return isGeneratedIdentifier(string(id), "task_")
}

func isGeneratedIdentifier(value string, prefix string) bool {
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	suffix := strings.TrimPrefix(value, prefix)
	if len(suffix) != 12 {
		return false
	}
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
