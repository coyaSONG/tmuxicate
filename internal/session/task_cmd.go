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

func TaskWait(stateDir, agent string, msgID protocol.MessageID, waitKind protocol.WaitKind, on, reason string) error {
	if reason == "" {
		return fmt.Errorf("reason is required")
	}
	if err := waitKind.Validate(); err != nil {
		return fmt.Errorf("wait kind: %w", err)
	}

	activeTask, err := loadActiveTaskContext(stateDir, agent, msgID)
	if err != nil {
		return err
	}

	if coordinatorTask, err := loadCoordinatorTaskContext(activeTask); err != nil {
		return err
	} else if coordinatorTask != nil {
		if err := recordCoordinatorWait(activeTask, coordinatorTask, waitKind, reason); err != nil {
			return err
		}
	}

	return appendStateEvent(stateDir, activeTask.agentName, &TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Agent:         activeTask.agentName,
		Event:         "task.wait",
		DeclaredState: "awaiting_reply",
		MessageID:     msgID,
		Thread:        activeTask.result.Thread,
		ReceiptState:  protocol.FolderStateActive,
		WaitingOn:     on,
		Reason:        reason,
	})
}

func TaskBlock(stateDir, agent string, msgID protocol.MessageID, blockKind protocol.BlockKind, on, reason string) error {
	if reason == "" {
		return fmt.Errorf("reason is required")
	}
	if err := blockKind.Validate(); err != nil {
		return fmt.Errorf("block kind: %w", err)
	}
	if on == "" {
		on = "human"
	}

	activeTask, err := loadActiveTaskContext(stateDir, agent, msgID)
	if err != nil {
		return err
	}

	if coordinatorTask, err := loadCoordinatorTaskContext(activeTask); err != nil {
		return err
	} else if coordinatorTask != nil {
		if err := recordCoordinatorBlock(activeTask, coordinatorTask, blockKind, reason); err != nil {
			return err
		}
	}

	return appendStateEvent(stateDir, activeTask.agentName, &TaskEvent{
		Schema:        "tmuxicate/state-event/v1",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Agent:         activeTask.agentName,
		Event:         "task.block",
		DeclaredState: "blocked",
		MessageID:     msgID,
		Thread:        activeTask.result.Thread,
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

type activeTaskContext struct {
	cfg       *config.ResolvedConfig
	store     *mailbox.Store
	env       *protocol.Envelope
	result    *ReadResult
	receipt   *protocol.Receipt
	agentName string
}

type coordinatorTaskContext struct {
	runID            protocol.RunID
	currentTask      *protocol.ChildTask
	sourceTask       *protocol.ChildTask
	blockerCase      *protocol.BlockerCase
	coordinatorStore *mailbox.CoordinatorStore
}

var waitBlockerPolicy = map[protocol.WaitKind]protocol.BlockerAction{
	"dependency_reply": protocol.BlockerActionWatch,
	"external_event":   protocol.BlockerActionWatch,
}

var blockBlockerPolicy = map[protocol.BlockKind]protocol.BlockerAction{
	"agent_clarification": protocol.BlockerActionClarificationRequest,
	"human_decision":      protocol.BlockerActionEscalate,
	"unsupported":         protocol.BlockerActionEscalate,
}

func loadActiveTaskContext(stateDir, agent string, msgID protocol.MessageID) (*activeTaskContext, error) {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return nil, err
	}

	agentName, err := resolveTargetAgent(cfg, agent)
	if err != nil {
		return nil, err
	}

	store := mailbox.NewStore(stateDir)
	receipt, err := store.ReadReceipt(agentName, msgID)
	if err != nil {
		return nil, err
	}
	if receipt.FolderState != protocol.FolderStateActive {
		return nil, fmt.Errorf("task %s must be active", msgID)
	}

	result, err := ReadMsg(stateDir, agentName, msgID)
	if err != nil {
		return nil, err
	}
	if result.RequiresClaim && receipt.ClaimedBy != nil && *receipt.ClaimedBy != protocol.AgentName(agentName) {
		return nil, fmt.Errorf("message %s claimed by %s", msgID, *receipt.ClaimedBy)
	}

	env, _, err := store.ReadMessage(msgID)
	if err != nil {
		return nil, err
	}

	return &activeTaskContext{
		cfg:       cfg,
		store:     store,
		env:       env,
		result:    result,
		receipt:   receipt,
		agentName: agentName,
	}, nil
}

func loadCoordinatorTaskContext(activeTask *activeTaskContext) (*coordinatorTaskContext, error) {
	if activeTask == nil || activeTask.env == nil {
		return nil, nil
	}

	runIDValue := strings.TrimSpace(activeTask.env.Meta["parent_run_id"])
	currentTaskIDValue := strings.TrimSpace(activeTask.env.Meta["task_id"])
	if runIDValue == "" || currentTaskIDValue == "" {
		return nil, nil
	}

	runID := protocol.RunID(runIDValue)
	currentTaskID := protocol.TaskID(currentTaskIDValue)
	coordinatorStore := mailbox.NewCoordinatorStore(activeTask.cfg.Session.StateDir)
	currentTask, err := coordinatorStore.ReadTask(runID, currentTaskID)
	if err != nil {
		return nil, err
	}

	blockerCase, err := coordinatorStore.FindBlockerCaseByCurrentTaskID(runID, currentTaskID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if errors.Is(err, os.ErrNotExist) {
		blockerCase = nil
	}

	sourceTaskID := currentTaskID
	if blockerCase != nil {
		sourceTaskID = blockerCase.SourceTaskID
	}
	sourceTask := currentTask
	if sourceTaskID != currentTaskID {
		sourceTask, err = coordinatorStore.ReadTask(runID, sourceTaskID)
		if err != nil {
			return nil, err
		}
	}

	return &coordinatorTaskContext{
		runID:            runID,
		currentTask:      currentTask,
		sourceTask:       sourceTask,
		blockerCase:      blockerCase,
		coordinatorStore: coordinatorStore,
	}, nil
}

func recordCoordinatorWait(activeTask *activeTaskContext, coordinatorTask *coordinatorTaskContext, waitKind protocol.WaitKind, reason string) error {
	return recordCoordinatorBlocker(activeTask, coordinatorTask, "wait", waitKind, "", reason)
}

func recordCoordinatorBlock(activeTask *activeTaskContext, coordinatorTask *coordinatorTaskContext, blockKind protocol.BlockKind, reason string) error {
	return recordCoordinatorBlocker(activeTask, coordinatorTask, "block", "", blockKind, reason)
}

func recordCoordinatorBlocker(activeTask *activeTaskContext, coordinatorTask *coordinatorTaskContext, declaredState string, waitKind protocol.WaitKind, blockKind protocol.BlockKind, reason string) error {
	now := time.Now().UTC()
	caseDoc := prepareBlockerCase(activeTask, coordinatorTask, declaredState, waitKind, blockKind, reason, now)

	switch caseDoc.SelectedAction {
	case protocol.BlockerActionWatch, protocol.BlockerActionClarificationRequest:
		caseDoc.Status = protocol.BlockerStatusActive
		caseDoc.RecommendedAction = nil
		caseDoc.EscalatedAt = nil
	case protocol.BlockerActionReroute:
		reroutedTask, err := rerouteBlockerTask(activeTask, coordinatorTask, "", reason)
		if err != nil {
			return err
		}
		caseDoc.Status = protocol.BlockerStatusActive
		caseDoc.CurrentTaskID = reroutedTask.TaskID
		caseDoc.CurrentMessageID = reroutedTask.MessageID
		caseDoc.CurrentOwner = reroutedTask.Owner
		caseDoc.RerouteCount++
		caseDoc.Attempts = append(caseDoc.Attempts, protocol.BlockerAttempt{
			Action:    protocol.BlockerActionReroute,
			TaskID:    reroutedTask.TaskID,
			MessageID: reroutedTask.MessageID,
			Owner:     reroutedTask.Owner,
			Note:      strings.TrimSpace(reason),
			CreatedAt: now,
		})
	case protocol.BlockerActionEscalate:
		caseDoc.Status = protocol.BlockerStatusEscalated
		caseDoc.RecommendedAction = recommendedBlockerAction(caseDoc.BlockKind, reason, caseDoc.RerouteCount, caseDoc.MaxReroutes)
		caseDoc.EscalatedAt = &now
	default:
		return fmt.Errorf("unsupported blocker action %q", caseDoc.SelectedAction)
	}

	return persistBlockerCase(coordinatorTask.coordinatorStore, caseDoc, coordinatorTask.blockerCase == nil)
}

func prepareBlockerCase(activeTask *activeTaskContext, coordinatorTask *coordinatorTaskContext, declaredState string, waitKind protocol.WaitKind, blockKind protocol.BlockKind, reason string, now time.Time) *protocol.BlockerCase {
	var caseDoc *protocol.BlockerCase
	if coordinatorTask.blockerCase != nil {
		existing := *coordinatorTask.blockerCase
		if existing.Attempts != nil {
			existing.Attempts = append([]protocol.BlockerAttempt(nil), existing.Attempts...)
		}
		caseDoc = &existing
	} else {
		caseDoc = &protocol.BlockerCase{
			RunID:           coordinatorTask.runID,
			SourceTaskID:    coordinatorTask.sourceTask.TaskID,
			SourceMessageID: coordinatorTask.sourceTask.MessageID,
			SourceOwner:     coordinatorTask.sourceTask.Owner,
			CreatedAt:       now,
		}
	}

	caseDoc.CurrentTaskID = coordinatorTask.currentTask.TaskID
	caseDoc.CurrentMessageID = activeTask.env.ID
	caseDoc.CurrentOwner = protocol.AgentName(activeTask.agentName)
	caseDoc.DeclaredState = declaredState
	caseDoc.WaitKind = ""
	caseDoc.BlockKind = ""
	caseDoc.Reason = strings.TrimSpace(reason)
	caseDoc.SelectedAction = selectBlockerAction(waitKind, blockKind, caseDoc.RerouteCount, resolveBlockerCeiling(activeTask.cfg, coordinatorTask.sourceTask))
	caseDoc.Status = protocol.BlockerStatusActive
	caseDoc.MaxReroutes = resolveBlockerCeiling(activeTask.cfg, coordinatorTask.sourceTask)
	caseDoc.RecommendedAction = nil
	caseDoc.Resolution = nil
	caseDoc.UpdatedAt = now
	caseDoc.ResolvedAt = nil
	caseDoc.EscalatedAt = nil
	if declaredState == "wait" {
		caseDoc.WaitKind = waitKind
	} else {
		caseDoc.BlockKind = blockKind
	}

	return caseDoc
}

func selectBlockerAction(waitKind protocol.WaitKind, blockKind protocol.BlockKind, rerouteCount, maxReroutes int) protocol.BlockerAction {
	if waitKind != "" {
		if action, ok := waitBlockerPolicy[waitKind]; ok {
			return action
		}
		return protocol.BlockerActionEscalate
	}

	if blockKind == "reroute_needed" {
		if rerouteCount >= maxReroutes {
			return protocol.BlockerActionEscalate
		}
		return protocol.BlockerActionReroute
	}

	if action, ok := blockBlockerPolicy[blockKind]; ok {
		return action
	}

	switch blockKind {
	default:
		return protocol.BlockerActionEscalate
	}
}

func resolveBlockerCeiling(cfg *config.ResolvedConfig, sourceTask *protocol.ChildTask) int {
	if cfg == nil {
		return 0
	}
	if sourceTask != nil {
		if maxReroutes, ok := cfg.Blockers.MaxReroutesByTaskClass[sourceTask.TaskClass]; ok {
			return maxReroutes
		}
	}
	return cfg.Blockers.MaxReroutesDefault
}

func recommendedBlockerAction(blockKind protocol.BlockKind, reason string, rerouteCount, maxReroutes int) *protocol.RecommendedAction {
	note := strings.TrimSpace(reason)
	if blockKind == protocol.BlockKindRerouteNeeded {
		if note == "" {
			note = fmt.Sprintf("reroute ceiling reached at %d/%d attempts", rerouteCount, maxReroutes)
		}
		return &protocol.RecommendedAction{
			Kind: protocol.BlockerResolutionActionManualReroute,
			Note: note,
		}
	}

	if note == "" {
		note = "operator clarification required"
	}
	return &protocol.RecommendedAction{
		Kind: protocol.BlockerResolutionActionClarify,
		Note: note,
	}
}

func rerouteBlockerTask(activeTask *activeTaskContext, coordinatorTask *coordinatorTaskContext, overrideOwner protocol.AgentName, reason string) (*protocol.ChildTask, error) {
	sourceTask := coordinatorTask.sourceTask
	if sourceTask == nil {
		return nil, errors.New("source task is required for reroute")
	}
	if err := sourceTask.TaskClass.Validate(); err != nil {
		return nil, fmt.Errorf("source task task_class: %w", err)
	}

	domains := append([]string(nil), sourceTask.NormalizedDomains...)
	if len(domains) == 0 {
		domains = append([]string(nil), sourceTask.Domains...)
	}
	if len(domains) == 0 {
		return nil, fmt.Errorf("source task %s missing routing domains", sourceTask.TaskID)
	}

	restoreReceipt, err := suspendTaskReceiptForReroute(activeTask.store, string(coordinatorTask.currentTask.Owner), coordinatorTask.currentTask.MessageID)
	if err != nil {
		return nil, err
	}

	req := protocol.RouteChildTaskRequest{
		RunID:          coordinatorTask.runID,
		TaskClass:      sourceTask.TaskClass,
		Domains:        domains,
		Goal:           sourceTask.Goal,
		ExpectedOutput: sourceTask.ExpectedOutput,
		ReviewRequired: sourceTask.ReviewRequired,
	}

	if overrideOwner == "" {
		overrideOwner = selectNextRerouteOwner(sourceTask, coordinatorTask.currentTask.Owner)
	}
	if overrideOwner != "" {
		req.OwnerOverride = overrideOwner
		req.OverrideReason = rerouteOverrideReason(reason, sourceTask.TaskID, coordinatorTask.currentTask.Owner)
	}

	reroutedTask, _, err := RouteChildTask(activeTask.cfg, activeTask.store, req)
	if err != nil {
		if restoreErr := restoreReceipt(); restoreErr != nil {
			return nil, fmt.Errorf("restore current receipt after reroute failure: %w", restoreErr)
		}
		return nil, err
	}

	return reroutedTask, nil
}

func suspendTaskReceiptForReroute(store *mailbox.Store, owner string, msgID protocol.MessageID) (func() error, error) {
	if store == nil {
		return nil, errors.New("store is required")
	}
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(string(msgID)) == "" {
		return func() error { return nil }, nil
	}

	receipt, err := store.ReadReceipt(owner, msgID)
	if err != nil {
		return nil, err
	}
	if receipt.FolderState == protocol.FolderStateDone || receipt.FolderState == protocol.FolderStateDead {
		return func() error { return nil }, nil
	}

	originalState := receipt.FolderState
	var claimedBy *protocol.AgentName
	if receipt.ClaimedBy != nil {
		value := *receipt.ClaimedBy
		claimedBy = &value
	}
	var claimedAt *time.Time
	if receipt.ClaimedAt != nil {
		value := *receipt.ClaimedAt
		claimedAt = &value
	}
	var doneAt *time.Time
	if receipt.DoneAt != nil {
		value := *receipt.DoneAt
		doneAt = &value
	}

	if err := store.UpdateReceipt(owner, msgID, func(r *protocol.Receipt) {
		r.ClaimedBy = nil
		r.ClaimedAt = nil
		r.DoneAt = nil
		r.Revision++
	}); err != nil {
		return nil, err
	}
	if err := store.MoveReceipt(owner, msgID, originalState, protocol.FolderStateDead); err != nil {
		return nil, err
	}

	return func() error {
		if err := store.MoveReceipt(owner, msgID, protocol.FolderStateDead, originalState); err != nil {
			return err
		}
		return store.UpdateReceipt(owner, msgID, func(r *protocol.Receipt) {
			r.ClaimedBy = claimedBy
			r.ClaimedAt = claimedAt
			r.DoneAt = doneAt
			r.Revision++
		})
	}, nil
}

func selectNextRerouteOwner(sourceTask *protocol.ChildTask, currentOwner protocol.AgentName) protocol.AgentName {
	if sourceTask == nil || sourceTask.RoutingDecision == nil {
		return ""
	}

	for _, candidate := range sourceTask.RoutingDecision.Candidates {
		if candidate == "" || candidate == currentOwner {
			continue
		}
		return candidate
	}

	return ""
}

func rerouteOverrideReason(reason string, sourceTaskID protocol.TaskID, currentOwner protocol.AgentName) string {
	note := strings.TrimSpace(reason)
	if note != "" {
		return note
	}
	return fmt.Sprintf("reroute %s away from %s", sourceTaskID, currentOwner)
}

func persistBlockerCase(coordinatorStore *mailbox.CoordinatorStore, caseDoc *protocol.BlockerCase, create bool) error {
	if coordinatorStore == nil {
		return errors.New("coordinator store is required")
	}
	if caseDoc == nil {
		return errors.New("blocker case is required")
	}

	if create {
		return coordinatorStore.CreateBlockerCase(caseDoc)
	}

	return coordinatorStore.UpdateBlockerCase(caseDoc.RunID, caseDoc.SourceTaskID, func(existing *protocol.BlockerCase) error {
		*existing = *caseDoc
		return nil
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
