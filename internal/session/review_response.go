package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func ReviewRespond(stateDir string, store *mailbox.Store, agent string, reviewMessageID protocol.MessageID, outcome protocol.ReviewOutcome, body []byte) (protocol.MessageID, error) {
	if store == nil {
		return "", fmt.Errorf("store is required")
	}
	if err := outcome.Validate(); err != nil {
		return "", fmt.Errorf("outcome: %w", err)
	}
	if len(body) == 0 || strings.TrimSpace(string(body)) == "" {
		return "", fmt.Errorf("body is required")
	}

	result, receipt, agentName, err := loadActiveTask(stateDir, agent, reviewMessageID)
	if err != nil {
		return "", err
	}
	if result.RequiresClaim && receipt.ClaimedBy != nil && *receipt.ClaimedBy != protocol.AgentName(agentName) {
		return "", fmt.Errorf("message %s claimed by %s", reviewMessageID, *receipt.ClaimedBy)
	}

	env, _, err := store.ReadMessage(reviewMessageID)
	if err != nil {
		return "", err
	}
	if env.Kind != protocol.KindReviewRequest {
		return "", fmt.Errorf("message %s is not a review request", reviewMessageID)
	}

	runIDValue := strings.TrimSpace(env.Meta["parent_run_id"])
	reviewTaskIDValue := strings.TrimSpace(env.Meta["task_id"])
	if runIDValue == "" || reviewTaskIDValue == "" {
		return "", fmt.Errorf("review request %s missing parent_run_id or task_id metadata", reviewMessageID)
	}

	runID := protocol.RunID(runIDValue)
	reviewTaskID := protocol.TaskID(reviewTaskIDValue)
	coordinatorStore := mailbox.NewCoordinatorStore(stateDir)
	handoff, err := coordinatorStore.FindReviewHandoffByReviewTaskID(runID, reviewTaskID)
	if err != nil {
		return "", err
	}
	if handoff.Status != protocol.ReviewHandoffStatusPending {
		return "", fmt.Errorf("review handoff for %s is not pending", reviewTaskID)
	}
	if handoff.ReviewTaskID != reviewTaskID {
		return "", fmt.Errorf("review handoff task mismatch: got %s want %s", handoff.ReviewTaskID, reviewTaskID)
	}
	if handoff.ReviewMessageID != reviewMessageID {
		return "", fmt.Errorf("review handoff message mismatch: got %s want %s", handoff.ReviewMessageID, reviewMessageID)
	}
	if handoff.Reviewer != protocol.AgentName(agentName) {
		return "", fmt.Errorf("review handoff reviewer mismatch: got %s want %s", handoff.Reviewer, agentName)
	}

	responseMessageID, err := Reply(stateDir, store, agentName, reviewMessageID, body)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	if err := store.UpdateReceipt(agentName, reviewMessageID, func(r *protocol.Receipt) {
		r.ClaimedBy = nil
		r.ClaimedAt = nil
		r.DoneAt = &now
		r.Revision++
	}); err != nil {
		return "", err
	}
	if err := store.MoveReceipt(agentName, reviewMessageID, protocol.FolderStateActive, protocol.FolderStateDone); err != nil {
		return "", err
	}

	if err := appendStateEvent(stateDir, agentName, &TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     now.Format(time.RFC3339Nano),
		Agent:         agentName,
		Event:         "review.respond",
		DeclaredState: "idle",
		MessageID:     reviewMessageID,
		Thread:        result.Thread,
		ReceiptState:  protocol.FolderStateDone,
		Summary:       string(outcome),
	}); err != nil {
		return "", err
	}

	if err := coordinatorStore.UpdateReviewHandoff(runID, handoff.SourceTaskID, func(existing *protocol.ReviewHandoff) error {
		existing.ResponseMessageID = responseMessageID
		existing.Outcome = outcome
		existing.RespondedAt = &now
		existing.Status = protocol.ReviewHandoffStatusResponded
		return nil
	}); err != nil {
		return "", err
	}

	return responseMessageID, nil
}
