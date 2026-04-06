package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

type TaskEvent struct {
	Schema        string               `json:"schema"`
	Timestamp     string               `json:"ts"`
	Agent         string               `json:"agent"`
	Event         string               `json:"event"`
	DeclaredState string               `json:"declared_state"`
	MessageID     protocol.MessageID   `json:"message_id"`
	Thread        protocol.ThreadID    `json:"thread"`
	ReceiptState  protocol.FolderState `json:"receipt_state"`
	WaitingOn     string               `json:"waiting_on,omitempty"`
	BlockedOn     string               `json:"blocked_on,omitempty"`
	Reason        string               `json:"reason,omitempty"`
	Summary       string               `json:"summary,omitempty"`
}

func TaskAccept(stateDir, agent string, msgID protocol.MessageID) error {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return err
	}

	agentName, err := resolveTargetAgent(cfg, agent)
	if err != nil {
		return err
	}

	result, err := ReadMsg(stateDir, agentName, msgID)
	if err != nil {
		return err
	}
	if !isClaimableKind(result.Kind) {
		return fmt.Errorf("message %s is not a task", msgID)
	}

	store := mailbox.NewStore(stateDir)
	if result.RequiresClaim {
		now := time.Now().UTC()
		if err := store.UpdateReceipt(agentName, msgID, func(r *protocol.Receipt) {
			if r.ClaimedBy != nil && *r.ClaimedBy != protocol.AgentName(agentName) {
				return
			}
			claimedBy := protocol.AgentName(agentName)
			r.ClaimedBy = &claimedBy
			r.ClaimedAt = &now
			r.Revision++
		}); err != nil {
			return err
		}

		receipt, err := store.ReadReceipt(agentName, msgID)
		if err != nil {
			return err
		}
		if receipt.ClaimedBy != nil && *receipt.ClaimedBy != protocol.AgentName(agentName) {
			return fmt.Errorf("message %s already claimed by %s", msgID, *receipt.ClaimedBy)
		}
	}

	return appendStateEvent(stateDir, agentName, &TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Agent:         agentName,
		Event:         "task.accept",
		DeclaredState: "busy",
		MessageID:     msgID,
		Thread:        result.Thread,
		ReceiptState:  protocol.FolderStateActive,
	})
}

func TaskWait(stateDir, agent string, msgID protocol.MessageID, on, reason string) error {
	if reason == "" {
		return fmt.Errorf("reason is required")
	}

	result, receipt, agentName, err := loadActiveTask(stateDir, agent, msgID)
	if err != nil {
		return err
	}

	if result.RequiresClaim && receipt.ClaimedBy != nil && *receipt.ClaimedBy != protocol.AgentName(agentName) {
		return fmt.Errorf("message %s claimed by %s", msgID, *receipt.ClaimedBy)
	}

	return appendStateEvent(stateDir, agentName, &TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Agent:         agentName,
		Event:         "task.wait",
		DeclaredState: "awaiting_reply",
		MessageID:     msgID,
		Thread:        result.Thread,
		ReceiptState:  protocol.FolderStateActive,
		WaitingOn:     on,
		Reason:        reason,
	})
}

func TaskBlock(stateDir, agent string, msgID protocol.MessageID, on, reason string) error {
	if reason == "" {
		return fmt.Errorf("reason is required")
	}
	if on == "" {
		on = "human"
	}

	result, receipt, agentName, err := loadActiveTask(stateDir, agent, msgID)
	if err != nil {
		return err
	}

	if result.RequiresClaim && receipt.ClaimedBy != nil && *receipt.ClaimedBy != protocol.AgentName(agentName) {
		return fmt.Errorf("message %s claimed by %s", msgID, *receipt.ClaimedBy)
	}

	return appendStateEvent(stateDir, agentName, &TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Agent:         agentName,
		Event:         "task.block",
		DeclaredState: "blocked",
		MessageID:     msgID,
		Thread:        result.Thread,
		ReceiptState:  protocol.FolderStateActive,
		BlockedOn:     on,
		Reason:        reason,
	})
}

func TaskDone(stateDir, agent string, msgID protocol.MessageID, summary string) error {
	result, receipt, agentName, err := loadActiveTask(stateDir, agent, msgID)
	if err != nil {
		return err
	}

	if result.RequiresClaim && receipt.ClaimedBy != nil && *receipt.ClaimedBy != protocol.AgentName(agentName) {
		return fmt.Errorf("message %s claimed by %s", msgID, *receipt.ClaimedBy)
	}

	store := mailbox.NewStore(stateDir)
	now := time.Now().UTC()
	if err := store.UpdateReceipt(agentName, msgID, func(r *protocol.Receipt) {
		r.ClaimedBy = nil
		r.ClaimedAt = nil
		r.DoneAt = &now
		r.Revision++
	}); err != nil {
		return err
	}
	if err := store.MoveReceipt(agentName, msgID, protocol.FolderStateActive, protocol.FolderStateDone); err != nil {
		return err
	}

	if err := appendStateEvent(stateDir, agentName, &TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Agent:         agentName,
		Event:         "task.done",
		DeclaredState: "idle",
		MessageID:     msgID,
		Thread:        result.Thread,
		ReceiptState:  protocol.FolderStateDone,
		Summary:       summary,
	}); err != nil {
		return err
	}

	return createReviewHandoffAfterTaskDone(stateDir, store, msgID)
}

func loadActiveTask(stateDir, agent string, msgID protocol.MessageID) (*ReadResult, *protocol.Receipt, string, error) {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return nil, nil, "", err
	}

	agentName, err := resolveTargetAgent(cfg, agent)
	if err != nil {
		return nil, nil, "", err
	}

	store := mailbox.NewStore(stateDir)
	receipt, err := store.ReadReceipt(agentName, msgID)
	if err != nil {
		return nil, nil, "", err
	}
	if receipt.FolderState != protocol.FolderStateActive {
		return nil, nil, "", fmt.Errorf("task %s must be active", msgID)
	}

	result, err := ReadMsg(stateDir, agentName, msgID)
	if err != nil {
		return nil, nil, "", err
	}

	return result, receipt, agentName, nil
}

func isClaimableKind(kind protocol.Kind) bool {
	switch kind {
	case protocol.KindTask, protocol.KindQuestion, protocol.KindReviewRequest, protocol.KindStatusRequest:
		return true
	default:
		return false
	}
}

func appendStateEvent(stateDir, agent string, event *TaskEvent) error {
	eventsDir := stateEventsDir(stateDir, agent)
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		return fmt.Errorf("create events dir: %w", err)
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal state event: %w", err)
	}

	logPath := filepath.Join(eventsDir, "state.jsonl")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open state log: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append state log: %w", err)
	}

	currentPath := filepath.Join(eventsDir, "state.current.json")
	current, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal current state event: %w", err)
	}
	if err := os.WriteFile(currentPath, append(current, '\n'), 0o644); err != nil {
		return fmt.Errorf("write current state event: %w", err)
	}

	return nil
}

func createReviewHandoffAfterTaskDone(stateDir string, store *mailbox.Store, msgID protocol.MessageID) error {
	env, _, err := store.ReadMessage(msgID)
	if err != nil {
		return fmt.Errorf("read completed task message: %w", err)
	}

	runIDValue := strings.TrimSpace(env.Meta["parent_run_id"])
	taskIDValue := strings.TrimSpace(env.Meta["task_id"])
	if runIDValue == "" || taskIDValue == "" {
		return nil
	}

	runID := protocol.RunID(runIDValue)
	sourceTaskID := protocol.TaskID(taskIDValue)
	coordinatorStore := mailbox.NewCoordinatorStore(stateDir)
	sourceTask, err := coordinatorStore.ReadTask(runID, sourceTaskID)
	if err != nil {
		return upsertFailedReviewHandoff(coordinatorStore, &protocol.ReviewHandoff{
			RunID:           runID,
			SourceTaskID:    sourceTaskID,
			SourceMessageID: env.ID,
			Status:          protocol.ReviewHandoffStatusHandoffFailed,
			FailureSummary:  fmt.Sprintf("source task %s could not be loaded for review routing: %v", sourceTaskID, err),
		})
	}

	if sourceTask.TaskClass != protocol.TaskClassImplementation || !sourceTask.ReviewRequired {
		return nil
	}

	if _, err := coordinatorStore.ReadReviewHandoff(runID, sourceTaskID); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if len(sourceTask.NormalizedDomains) == 0 {
		return upsertFailedReviewHandoff(coordinatorStore, &protocol.ReviewHandoff{
			RunID:           runID,
			SourceTaskID:    sourceTaskID,
			SourceMessageID: env.ID,
			Status:          protocol.ReviewHandoffStatusHandoffFailed,
			FailureSummary:  fmt.Sprintf("source task %s missing normalized_domains for review routing", sourceTaskID),
		})
	}

	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return upsertFailedReviewHandoff(coordinatorStore, &protocol.ReviewHandoff{
			RunID:           runID,
			SourceTaskID:    sourceTaskID,
			SourceMessageID: env.ID,
			Status:          protocol.ReviewHandoffStatusHandoffFailed,
			FailureSummary:  fmt.Sprintf("load resolved config for review routing: %v", err),
		})
	}

	reviewTask, err := routeReviewTask(cfg, store, sourceTask)
	if err != nil {
		return upsertFailedReviewHandoff(coordinatorStore, &protocol.ReviewHandoff{
			RunID:           runID,
			SourceTaskID:    sourceTaskID,
			SourceMessageID: env.ID,
			Status:          protocol.ReviewHandoffStatusHandoffFailed,
			FailureSummary:  fmt.Sprintf("route review handoff for %s: %v", sourceTaskID, err),
		})
	}

	handoff := &protocol.ReviewHandoff{
		RunID:           runID,
		SourceTaskID:    sourceTaskID,
		SourceMessageID: env.ID,
		ReviewTaskID:    reviewTask.TaskID,
		ReviewMessageID: reviewTask.MessageID,
		Reviewer:        reviewTask.Owner,
		Status:          protocol.ReviewHandoffStatusPending,
		CreatedAt:       time.Now().UTC(),
	}
	if err := coordinatorStore.CreateReviewHandoff(handoff); err != nil {
		return upsertFailedReviewHandoff(coordinatorStore, &protocol.ReviewHandoff{
			RunID:           runID,
			SourceTaskID:    sourceTaskID,
			SourceMessageID: env.ID,
			ReviewTaskID:    reviewTask.TaskID,
			ReviewMessageID: reviewTask.MessageID,
			Reviewer:        reviewTask.Owner,
			Status:          protocol.ReviewHandoffStatusHandoffFailed,
			FailureSummary:  fmt.Sprintf("persist review handoff for %s: %v", sourceTaskID, err),
		})
	}

	return nil
}

func routeReviewTask(cfg *config.ResolvedConfig, store *mailbox.Store, sourceTask *protocol.ChildTask) (*protocol.ChildTask, error) {
	reviewTask, _, err := RouteChildTask(cfg, store, protocol.RouteChildTaskRequest{
		RunID:          sourceTask.ParentRunID,
		TaskClass:      protocol.TaskClassReview,
		Domains:        append([]string(nil), sourceTask.NormalizedDomains...),
		Goal:           fmt.Sprintf("Review implementation task %s: %s", sourceTask.TaskID, sourceTask.Goal),
		ExpectedOutput: fmt.Sprintf("Submit approved or changes_requested for %s via tmuxicate review respond", sourceTask.TaskID),
		ReviewRequired: false,
	})
	if err != nil {
		return nil, err
	}

	return reviewTask, nil
}

func upsertFailedReviewHandoff(coordinatorStore *mailbox.CoordinatorStore, handoff *protocol.ReviewHandoff) error {
	if coordinatorStore == nil {
		return fmt.Errorf("coordinator store is required")
	}
	if handoff == nil {
		return fmt.Errorf("review handoff is required")
	}

	now := time.Now().UTC()
	if handoff.CreatedAt.IsZero() {
		handoff.CreatedAt = now
	}

	if _, err := coordinatorStore.ReadReviewHandoff(handoff.RunID, handoff.SourceTaskID); err == nil {
		return coordinatorStore.UpdateReviewHandoff(handoff.RunID, handoff.SourceTaskID, func(existing *protocol.ReviewHandoff) error {
			if existing.SourceMessageID == "" {
				existing.SourceMessageID = handoff.SourceMessageID
			}
			if handoff.ReviewTaskID != "" {
				existing.ReviewTaskID = handoff.ReviewTaskID
			}
			if handoff.ReviewMessageID != "" {
				existing.ReviewMessageID = handoff.ReviewMessageID
			}
			if handoff.Reviewer != "" {
				existing.Reviewer = handoff.Reviewer
			}
			existing.Status = protocol.ReviewHandoffStatusHandoffFailed
			existing.FailureSummary = handoff.FailureSummary
			return nil
		})
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	handoff.Status = protocol.ReviewHandoffStatusHandoffFailed
	return coordinatorStore.CreateReviewHandoff(handoff)
}
