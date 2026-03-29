package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

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

	return appendStateEvent(stateDir, agentName, TaskEvent{
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

	return appendStateEvent(stateDir, agentName, TaskEvent{
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

	return appendStateEvent(stateDir, agentName, TaskEvent{
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

	return appendStateEvent(stateDir, agentName, TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Agent:         agentName,
		Event:         "task.done",
		DeclaredState: "idle",
		MessageID:     msgID,
		Thread:        result.Thread,
		ReceiptState:  protocol.FolderStateDone,
		Summary:       summary,
	})
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

func appendStateEvent(stateDir, agent string, event TaskEvent) error {
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
