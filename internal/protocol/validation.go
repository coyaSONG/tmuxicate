package protocol

import (
	"errors"
	"fmt"
	"slices"
	"sort"
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
		if hasExecutionTargetMetadata(snapshot.ExecutionTarget) {
			if err := validateExecutionTarget(&snapshot.ExecutionTarget); err != nil {
				return fmt.Errorf("team_snapshot[%d].execution_target: %w", i, err)
			}
			r.TeamSnapshot[i].ExecutionTarget = snapshot.ExecutionTarget
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
	if t.Placement != nil {
		if err := validateTaskPlacement(t.Placement); err != nil {
			return fmt.Errorf("placement: %w", err)
		}
	}
	if childTaskHasRoutingMetadata(t) {
		if err := t.TaskClass.Validate(); err != nil {
			return fmt.Errorf("task_class: %w", err)
		}

		domains, err := NormalizeRouteDomains(t.Domains)
		if err != nil {
			return fmt.Errorf("domains: %w", err)
		}
		if len(domains) == 0 {
			return errors.New("domains must contain at least one domain for routed tasks")
		}

		normalizedDomains, err := NormalizeRouteDomains(t.NormalizedDomains)
		if err != nil {
			return fmt.Errorf("normalized_domains: %w", err)
		}
		if len(normalizedDomains) == 0 {
			return errors.New("normalized_domains must contain at least one domain for routed tasks")
		}
		if !slices.Equal(normalizedDomains, domains) {
			return errors.New("normalized_domains must match normalized domains")
		}

		expectedDuplicateKey := fmt.Sprintf("%s|%s|%s", t.ParentRunID, t.TaskClass, strings.Join(normalizedDomains, ","))
		if strings.TrimSpace(t.DuplicateKey) == "" {
			return errors.New("duplicate_key is required for routed tasks")
		}
		if t.DuplicateKey != expectedDuplicateKey {
			return fmt.Errorf("duplicate_key must equal %q", expectedDuplicateKey)
		}

		if t.RoutingDecision == nil {
			return errors.New("routing_decision is required for routed tasks")
		}
		if err := t.RoutingDecision.Validate(); err != nil {
			return fmt.Errorf("routing_decision: %w", err)
		}
		if t.RoutingDecision.TieBreak == "owner_override" && strings.TrimSpace(t.OverrideReason) == "" {
			return errors.New("override_reason is required when owner override is used")
		}

		t.Domains = domains
		t.NormalizedDomains = normalizedDomains
	}

	return nil
}

func validateTaskPlacement(placement *TaskPlacement) error {
	if placement == nil {
		return nil
	}
	if err := validateExecutionTarget(&placement.Target); err != nil {
		return fmt.Errorf("target: %w", err)
	}
	placement.Reason = strings.TrimSpace(placement.Reason)
	if placement.Reason == "" {
		return errors.New("reason is required")
	}

	return nil
}

func validateExecutionTarget(target *ExecutionTarget) error {
	if target == nil {
		return errors.New("execution target is required")
	}

	target.Name = strings.TrimSpace(target.Name)
	if target.Name == "" {
		return errors.New("name is required")
	}
	target.Kind = strings.TrimSpace(target.Kind)
	if target.Kind == "" {
		return errors.New("kind is required")
	}
	if !isValidExecutionTargetKind(target.Kind) {
		return fmt.Errorf("kind %q is invalid", target.Kind)
	}
	target.Description = strings.TrimSpace(target.Description)

	capabilities, err := normalizeExecutionTargetCapabilities(target.Capabilities)
	if err != nil {
		return err
	}
	target.Capabilities = capabilities

	return nil
}

func hasExecutionTargetMetadata(target ExecutionTarget) bool {
	return strings.TrimSpace(target.Name) != "" ||
		strings.TrimSpace(target.Kind) != "" ||
		strings.TrimSpace(target.Description) != "" ||
		len(target.Capabilities) > 0 ||
		target.PaneBacked
}

func (h *ReviewHandoff) Validate() error {
	if h == nil {
		return errors.New("review handoff is required")
	}
	if !isGeneratedRunID(h.RunID) {
		return errors.New("run_id must use generated run_ identifier")
	}
	if !isGeneratedTaskID(h.SourceTaskID) {
		return errors.New("source_task_id must use generated task_ identifier")
	}
	if strings.TrimSpace(string(h.SourceMessageID)) == "" {
		return errors.New("source_message_id is required")
	}
	if err := h.Status.Validate(); err != nil {
		return fmt.Errorf("status: %w", err)
	}
	if h.CreatedAt.IsZero() {
		return errors.New("created_at is required")
	}
	if h.RespondedAt != nil && h.RespondedAt.IsZero() {
		return errors.New("responded_at must not be zero")
	}
	if h.ReviewTaskID != "" && !isGeneratedTaskID(h.ReviewTaskID) {
		return errors.New("review_task_id must use generated task_ identifier")
	}
	if h.Reviewer != "" && strings.TrimSpace(string(h.Reviewer)) == "" {
		return errors.New("reviewer must not be blank")
	}

	switch h.Status {
	case ReviewHandoffStatusPending:
		if !isGeneratedTaskID(h.ReviewTaskID) {
			return errors.New("pending handoff requires review_task_id")
		}
		if strings.TrimSpace(string(h.ReviewMessageID)) == "" {
			return errors.New("pending handoff requires review_message_id")
		}
		if strings.TrimSpace(string(h.Reviewer)) == "" {
			return errors.New("pending handoff requires reviewer")
		}
	case ReviewHandoffStatusResponded:
		if !isGeneratedTaskID(h.ReviewTaskID) {
			return errors.New("responded handoff requires review_task_id")
		}
		if strings.TrimSpace(string(h.ReviewMessageID)) == "" {
			return errors.New("responded handoff requires review_message_id")
		}
		if strings.TrimSpace(string(h.Reviewer)) == "" {
			return errors.New("responded handoff requires reviewer")
		}
		if strings.TrimSpace(string(h.ResponseMessageID)) == "" {
			return errors.New("responded handoff requires response_message_id")
		}
		if err := h.Outcome.Validate(); err != nil {
			return fmt.Errorf("outcome: %w", err)
		}
		if h.RespondedAt == nil {
			return errors.New("responded handoff requires responded_at")
		}
	case ReviewHandoffStatusHandoffFailed:
		if strings.TrimSpace(h.FailureSummary) == "" {
			return errors.New("handoff_failed requires failure_summary")
		}
	}

	return nil
}

func (b *BlockerCase) Validate() error {
	if b == nil {
		return errors.New("blocker case is required")
	}
	if !isGeneratedRunID(b.RunID) {
		return errors.New("run_id must use generated run_ identifier")
	}
	if !isGeneratedTaskID(b.SourceTaskID) {
		return errors.New("source_task_id must use generated task_ identifier")
	}
	if strings.TrimSpace(string(b.SourceMessageID)) == "" {
		return errors.New("source_message_id is required")
	}
	if strings.TrimSpace(string(b.SourceOwner)) == "" {
		return errors.New("source_owner is required")
	}
	if b.CurrentTaskID != "" && !isGeneratedTaskID(b.CurrentTaskID) {
		return errors.New("current_task_id must use generated task_ identifier")
	}
	if strings.TrimSpace(string(b.CurrentMessageID)) == "" {
		return errors.New("current_message_id is required")
	}
	if strings.TrimSpace(string(b.CurrentOwner)) == "" {
		return errors.New("current_owner is required")
	}
	if strings.TrimSpace(b.Reason) == "" {
		return errors.New("reason is required")
	}
	if err := b.SelectedAction.Validate(); err != nil {
		return fmt.Errorf("selected_action: %w", err)
	}
	if err := b.Status.Validate(); err != nil {
		return fmt.Errorf("status: %w", err)
	}
	if b.RerouteCount < 0 {
		return errors.New("reroute_count must be >= 0")
	}
	if b.MaxReroutes < 0 {
		return errors.New("max_reroutes must be >= 0")
	}
	if b.CreatedAt.IsZero() {
		return errors.New("created_at is required")
	}
	if b.UpdatedAt.IsZero() {
		return errors.New("updated_at is required")
	}
	if b.EscalatedAt != nil && b.EscalatedAt.IsZero() {
		return errors.New("escalated_at must not be zero")
	}
	if b.ResolvedAt != nil && b.ResolvedAt.IsZero() {
		return errors.New("resolved_at must not be zero")
	}

	switch b.DeclaredState {
	case "wait":
		if err := b.WaitKind.Validate(); err != nil {
			return fmt.Errorf("wait_kind: %w", err)
		}
		if b.BlockKind != "" {
			return errors.New("block_kind must be empty when declared_state=wait")
		}
	case "block":
		if err := b.BlockKind.Validate(); err != nil {
			return fmt.Errorf("block_kind: %w", err)
		}
		if b.WaitKind != "" {
			return errors.New("wait_kind must be empty when declared_state=block")
		}
	default:
		return errors.New(`declared_state must be either "wait" or "block"`)
	}

	if b.RecommendedAction != nil {
		if err := b.RecommendedAction.Validate(); err != nil {
			return fmt.Errorf("recommended_action: %w", err)
		}
	}
	if b.Resolution != nil {
		if err := b.Resolution.Validate(); err != nil {
			return fmt.Errorf("resolution: %w", err)
		}
	}
	for i := range b.Attempts {
		if err := b.Attempts[i].Validate(); err != nil {
			return fmt.Errorf("attempts[%d]: %w", i, err)
		}
	}

	switch b.Status {
	case BlockerStatusEscalated:
		if b.RecommendedAction == nil {
			return errors.New("escalated blocker cases require recommended_action.kind")
		}
		if b.EscalatedAt == nil {
			return errors.New("escalated blocker cases require escalated_at")
		}
	case BlockerStatusResolved:
		if b.Resolution == nil {
			return errors.New("resolved blocker cases require resolution.action")
		}
		if b.ResolvedAt == nil {
			return errors.New("resolved blocker cases require resolved_at")
		}
	}

	return nil
}

func (p *PartialReplan) Validate() error {
	if p == nil {
		return errors.New("partial replan is required")
	}
	if !isGeneratedRunID(p.RunID) {
		return errors.New("run_id must use generated run_ identifier")
	}
	if !isGeneratedTaskID(p.SourceTaskID) {
		return errors.New("source_task_id must use generated task_ identifier")
	}
	if !isGeneratedMessageID(p.SourceMessageID) {
		return errors.New("source_message_id must use generated msg_ identifier")
	}
	if !isGeneratedTaskID(p.BlockerSourceTaskID) {
		return errors.New("blocker_source_task_id must use generated task_ identifier")
	}
	if p.BlockerSourceTaskID != p.SourceTaskID {
		return errors.New("blocker_source_task_id must equal source_task_id")
	}
	if !isGeneratedTaskID(p.SupersededTaskID) {
		return errors.New("superseded_task_id must use generated task_ identifier")
	}
	if !isGeneratedMessageID(p.SupersededMessageID) {
		return errors.New("superseded_message_id must use generated msg_ identifier")
	}
	if strings.TrimSpace(string(p.SupersededOwner)) == "" {
		return errors.New("superseded_owner is required")
	}
	if !isGeneratedTaskID(p.ReplacementTaskID) {
		return errors.New("replacement_task_id must use generated task_ identifier")
	}
	if !isGeneratedMessageID(p.ReplacementMessageID) {
		return errors.New("replacement_message_id must use generated msg_ identifier")
	}
	if p.ReplacementTaskID == p.SourceTaskID {
		return errors.New("replacement_task_id must differ from source_task_id")
	}
	if p.ReplacementTaskID == p.SupersededTaskID {
		return errors.New("replacement_task_id must differ from superseded_task_id")
	}
	if strings.TrimSpace(string(p.ReplacementOwner)) == "" {
		return errors.New("replacement_owner is required")
	}
	if strings.TrimSpace(p.Reason) == "" {
		return errors.New("reason is required")
	}
	if err := p.Status.Validate(); err != nil {
		return fmt.Errorf("status: %w", err)
	}
	if p.CreatedAt.IsZero() {
		return errors.New("created_at is required")
	}
	if p.UpdatedAt.IsZero() {
		return errors.New("updated_at is required")
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

func (o ReviewOutcome) Validate() error {
	switch o {
	case ReviewOutcomeApproved, ReviewOutcomeChangesRequested:
		return nil
	default:
		return fmt.Errorf("must be one of %q or %q", ReviewOutcomeApproved, ReviewOutcomeChangesRequested)
	}
}

func (s ReviewHandoffStatus) Validate() error {
	switch s {
	case ReviewHandoffStatusPending, ReviewHandoffStatusResponded, ReviewHandoffStatusHandoffFailed:
		return nil
	default:
		return fmt.Errorf("must be one of %q, %q, or %q", ReviewHandoffStatusPending, ReviewHandoffStatusResponded, ReviewHandoffStatusHandoffFailed)
	}
}

func (w WaitKind) Validate() error {
	switch w {
	case WaitKindDependencyReply, WaitKindExternalEvent:
		return nil
	default:
		return fmt.Errorf("must be one of %q or %q", WaitKindDependencyReply, WaitKindExternalEvent)
	}
}

func (b BlockKind) Validate() error {
	switch b {
	case BlockKindAgentClarification, BlockKindRerouteNeeded, BlockKindHumanDecision, BlockKindUnsupported:
		return nil
	default:
		return fmt.Errorf(
			"must be one of %q, %q, %q, or %q",
			BlockKindAgentClarification,
			BlockKindRerouteNeeded,
			BlockKindHumanDecision,
			BlockKindUnsupported,
		)
	}
}

func (a BlockerAction) Validate() error {
	switch a {
	case BlockerActionWatch, BlockerActionClarificationRequest, BlockerActionReroute, BlockerActionEscalate:
		return nil
	default:
		return fmt.Errorf(
			"must be one of %q, %q, %q, or %q",
			BlockerActionWatch,
			BlockerActionClarificationRequest,
			BlockerActionReroute,
			BlockerActionEscalate,
		)
	}
}

func (s BlockerStatus) Validate() error {
	switch s {
	case BlockerStatusActive, BlockerStatusEscalated, BlockerStatusResolved:
		return nil
	default:
		return fmt.Errorf("must be one of %q, %q, or %q", BlockerStatusActive, BlockerStatusEscalated, BlockerStatusResolved)
	}
}

func (a BlockerResolutionAction) Validate() error {
	switch a {
	case BlockerResolutionActionManualReroute, BlockerResolutionActionPartialReplan, BlockerResolutionActionClarify, BlockerResolutionActionDismiss:
		return nil
	default:
		return fmt.Errorf(
			"must be one of %q, %q, %q, or %q",
			BlockerResolutionActionManualReroute,
			BlockerResolutionActionPartialReplan,
			BlockerResolutionActionClarify,
			BlockerResolutionActionDismiss,
		)
	}
}

func (s PartialReplanStatus) Validate() error {
	switch s {
	case PartialReplanStatusApplied:
		return nil
	default:
		return fmt.Errorf("must be %q", PartialReplanStatusApplied)
	}
}

func (a *RecommendedAction) Validate() error {
	if a == nil {
		return errors.New("recommended action is required")
	}
	if err := a.Kind.Validate(); err != nil {
		return fmt.Errorf("kind: %w", err)
	}

	return nil
}

func (a *BlockerAttempt) Validate() error {
	if a == nil {
		return errors.New("blocker attempt is required")
	}
	if err := a.Action.Validate(); err != nil {
		return fmt.Errorf("action: %w", err)
	}
	if a.TaskID != "" && !isGeneratedTaskID(a.TaskID) {
		return errors.New("task_id must use generated task_ identifier")
	}
	if a.MessageID != "" && strings.TrimSpace(string(a.MessageID)) == "" {
		return errors.New("message_id must not be blank")
	}
	if a.Owner != "" && strings.TrimSpace(string(a.Owner)) == "" {
		return errors.New("owner must not be blank")
	}
	if a.CreatedAt.IsZero() {
		return errors.New("created_at is required")
	}

	return nil
}

func (r *BlockerResolution) Validate() error {
	if r == nil {
		return errors.New("blocker resolution is required")
	}
	if err := r.Action.Validate(); err != nil {
		return fmt.Errorf("action: %w", err)
	}
	if r.CreatedTaskID != "" && !isGeneratedTaskID(r.CreatedTaskID) {
		return errors.New("created_task_id must use generated task_ identifier")
	}
	if r.CreatedMessageID != "" && strings.TrimSpace(string(r.CreatedMessageID)) == "" {
		return errors.New("created_message_id must not be blank")
	}
	if r.ResolvedBy != "" && strings.TrimSpace(string(r.ResolvedBy)) == "" {
		return errors.New("resolved_by must not be blank")
	}
	if r.CreatedAt.IsZero() {
		return errors.New("created_at is required")
	}

	return nil
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
	if strings.TrimSpace(d.Status) == "" {
		return errors.New("status is required")
	}
	if strings.TrimSpace(string(d.SelectedOwner)) == "" {
		return errors.New("selected_owner is required")
	}
	if strings.TrimSpace(d.TieBreak) == "" {
		return errors.New("tie_break is required")
	}
	if len(d.Candidates) == 0 {
		return errors.New("candidates must contain at least one candidate")
	}
	if strings.TrimSpace(d.DuplicateStatus) == "" {
		return errors.New("duplicate_status is required")
	}
	if d.MatchedTaskID != "" && !isGeneratedTaskID(d.MatchedTaskID) {
		return errors.New("matched_task_id must use generated task_ identifier")
	}
	for i, suggestion := range d.Suggestions {
		if strings.TrimSpace(suggestion) == "" {
			return fmt.Errorf("suggestions[%d] must not be blank", i)
		}
	}
	if d.Adaptive != nil {
		if err := d.Adaptive.Validate(); err != nil {
			return fmt.Errorf("adaptive: %w", err)
		}
	}

	return nil
}

func (a *AdaptiveRoutingExplanation) Validate() error {
	if a == nil {
		return errors.New("adaptive routing explanation is required")
	}
	if a.ManualWeight < 0 {
		return errors.New("manual_weight must be >= 0")
	}
	if a.TotalScore != a.HistoricalScore+a.ManualWeight {
		return errors.New("total_score must equal historical_score + manual_weight")
	}
	if a.Applied {
		if strings.TrimSpace(string(a.BaselineOwner)) == "" {
			return errors.New("baseline_owner is required when applied is true")
		}
		if strings.TrimSpace(a.Reason) == "" {
			return errors.New("reason is required when applied is true")
		}
	}
	if len(a.Evidence) > 0 {
		if !slices.IsSortedFunc(a.Evidence, compareAdaptiveRoutingEvidenceRef) {
			return errors.New("evidence must be sorted by run_id, source_task_id, message_id, status")
		}
	}
	for i := range a.Evidence {
		if err := a.Evidence[i].Validate(); err != nil {
			return fmt.Errorf("evidence[%d]: %w", i, err)
		}
	}

	return nil
}

func (p *AdaptiveRoutingPreferenceSet) Validate() error {
	if p == nil {
		return errors.New("adaptive routing preference set is required")
	}
	if strings.TrimSpace(string(p.Coordinator)) == "" {
		return errors.New("coordinator is required")
	}
	if p.UpdatedAt.IsZero() {
		return errors.New("updated_at is required")
	}
	if p.LookbackRuns < 0 {
		return errors.New("lookback_runs must be >= 0")
	}
	for i := range p.Preferences {
		if err := p.Preferences[i].Validate(); err != nil {
			return fmt.Errorf("preferences[%d]: %w", i, err)
		}
	}

	return nil
}

func (p *AdaptiveRoutingPreference) Validate() error {
	if p == nil {
		return errors.New("adaptive routing preference is required")
	}
	if err := p.TaskClass.Validate(); err != nil {
		return fmt.Errorf("task_class: %w", err)
	}
	domains, err := NormalizeRouteDomains(p.NormalizedDomains)
	if err != nil {
		return fmt.Errorf("normalized_domains: %w", err)
	}
	if len(domains) == 0 {
		return errors.New("normalized_domains must contain at least one domain")
	}
	if !slices.Equal(domains, p.NormalizedDomains) {
		return errors.New("normalized_domains must be sorted and normalized")
	}
	if strings.TrimSpace(string(p.PreferredOwner)) == "" {
		return errors.New("preferred_owner is required")
	}
	if p.ManualWeight < 0 {
		return errors.New("manual_weight must be >= 0")
	}
	if p.TotalScore != p.HistoricalScore+p.ManualWeight {
		return errors.New("total_score must equal historical_score + manual_weight")
	}
	expectedKey := fmt.Sprintf("%s|%s|%s", p.TaskClass, strings.Join(domains, ","), p.PreferredOwner)
	if p.PreferenceKey != expectedKey {
		return fmt.Errorf("preference_key must equal %q", expectedKey)
	}
	if p.HistoricalScore != 0 && len(p.Evidence) == 0 {
		return errors.New("evidence is required when historical_score is non-zero")
	}
	if len(p.Evidence) > 0 && !slices.IsSortedFunc(p.Evidence, compareAdaptiveRoutingEvidenceRef) {
		return errors.New("evidence must be sorted by run_id, source_task_id, message_id, status")
	}
	for i := range p.Evidence {
		if err := p.Evidence[i].Validate(); err != nil {
			return fmt.Errorf("evidence[%d]: %w", i, err)
		}
	}
	p.NormalizedDomains = domains

	return nil
}

func (e *AdaptiveRoutingEvidenceRef) Validate() error {
	if e == nil {
		return errors.New("adaptive routing evidence ref is required")
	}
	if !isGeneratedRunID(e.RunID) {
		return errors.New("run_id must use generated run_ identifier")
	}
	if !isGeneratedTaskID(e.SourceTaskID) {
		return errors.New("source_task_id must use generated task_ identifier")
	}
	if strings.TrimSpace(string(e.MessageID)) == "" {
		return errors.New("message_id is required")
	}
	if strings.TrimSpace(e.Status) == "" {
		return errors.New("status is required")
	}
	if strings.TrimSpace(e.Note) == "" {
		return errors.New("note is required")
	}

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

func compareAdaptiveRoutingEvidenceRef(left, right AdaptiveRoutingEvidenceRef) int {
	if diff := strings.Compare(string(left.RunID), string(right.RunID)); diff != 0 {
		return diff
	}
	if diff := strings.Compare(string(left.SourceTaskID), string(right.SourceTaskID)); diff != 0 {
		return diff
	}
	if diff := strings.Compare(string(left.MessageID), string(right.MessageID)); diff != 0 {
		return diff
	}
	if diff := adaptiveRoutingEvidenceStatusRank(left.Status) - adaptiveRoutingEvidenceStatusRank(right.Status); diff != 0 {
		return diff
	}

	return strings.Compare(left.Note, right.Note)
}

func adaptiveRoutingEvidenceStatusRank(status string) int {
	switch status {
	case "completed":
		return 1
	case "approved":
		return 2
	case "changes_requested":
		return 3
	case "blocked":
		return 4
	case "waiting":
		return 5
	case "escalated":
		return 6
	default:
		return 99
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

func isGeneratedRunID(id RunID) bool {
	return isGeneratedIdentifier(string(id), "run_")
}

func childTaskHasRoutingMetadata(task *ChildTask) bool {
	if task == nil {
		return false
	}

	return task.TaskClass != "" ||
		len(task.Domains) > 0 ||
		len(task.NormalizedDomains) > 0 ||
		strings.TrimSpace(task.DuplicateKey) != "" ||
		task.RoutingDecision != nil ||
		strings.TrimSpace(task.OverrideReason) != ""
}

func isValidExecutionTargetKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "local", "remote", "sandbox":
		return true
	default:
		return false
	}
}

func normalizeExecutionTargetCapabilities(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for i, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, fmt.Errorf("capabilities[%d] must not be blank", i)
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	sort.Strings(normalized)
	return normalized, nil
}

func isGeneratedTaskID(id TaskID) bool {
	return isGeneratedIdentifier(string(id), "task_")
}

func isGeneratedMessageID(id MessageID) bool {
	return isGeneratedIdentifier(string(id), "msg_")
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
