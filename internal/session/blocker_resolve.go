package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

var blockerResolutionRequiresReason = map[protocol.BlockerResolutionAction]bool{
	"manual_reroute": true,
	"dismiss":        true,
}

var blockerResolutionRequiresBody = map[protocol.BlockerResolutionAction]bool{
	"clarify": true,
}

func BlockerResolve(stateDir string, store *mailbox.Store, runID protocol.RunID, sourceTaskID protocol.TaskID, action protocol.BlockerResolutionAction, owner string, reason string, body []byte) error {
	if store == nil {
		return fmt.Errorf("store is required")
	}
	if err := action.Validate(); err != nil {
		return fmt.Errorf("action: %w", err)
	}

	trimmedReason := strings.TrimSpace(reason)
	if blockerResolutionRequiresReason[action] {
		if trimmedReason == "" {
			return fmt.Errorf("reason is required")
		}
	}
	if blockerResolutionRequiresBody[action] {
		if len(body) == 0 || strings.TrimSpace(string(body)) == "" {
			return fmt.Errorf("clarify requires body")
		}
	}

	coordinatorStore := mailbox.NewCoordinatorStore(stateDir)
	blockerCase, err := coordinatorStore.ReadBlockerCase(runID, sourceTaskID)
	if err != nil {
		return err
	}
	if blockerCase.Status != protocol.BlockerStatusEscalated {
		return fmt.Errorf("blocker case %s is not escalated", sourceTaskID)
	}

	now := time.Now().UTC()
	resolution := &protocol.BlockerResolution{
		Action:     action,
		ResolvedBy: protocol.AgentName("human"),
		Note:       trimmedReason,
		CreatedAt:  now,
	}

	switch action {
	case protocol.BlockerResolutionActionManualReroute:
		cfg, err := loadResolvedConfigFromStateDir(stateDir)
		if err != nil {
			return err
		}
		activeTask := &activeTaskContext{
			cfg:   cfg,
			store: store,
		}
		coordinatorTask := &coordinatorTaskContext{
			runID:            runID,
			coordinatorStore: coordinatorStore,
		}

		sourceTask, err := coordinatorStore.ReadTask(runID, blockerCase.SourceTaskID)
		if err != nil {
			return err
		}
		currentTask, err := coordinatorStore.ReadTask(runID, blockerCase.CurrentTaskID)
		if err != nil {
			return err
		}
		coordinatorTask.sourceTask = sourceTask
		coordinatorTask.currentTask = currentTask

		overrideOwner := protocol.AgentName(strings.TrimSpace(owner))
		reroutedTask, err := rerouteBlockerTask(activeTask, coordinatorTask, overrideOwner, trimmedReason)
		if err != nil {
			return err
		}
		blockerCase.CurrentTaskID = reroutedTask.TaskID
		blockerCase.CurrentMessageID = reroutedTask.MessageID
		blockerCase.CurrentOwner = reroutedTask.Owner
		blockerCase.Attempts = append(blockerCase.Attempts, protocol.BlockerAttempt{
			Action:    protocol.BlockerActionReroute,
			TaskID:    reroutedTask.TaskID,
			MessageID: reroutedTask.MessageID,
			Owner:     reroutedTask.Owner,
			Note:      trimmedReason,
			CreatedAt: now,
		})
		resolution.CreatedTaskID = reroutedTask.TaskID
		resolution.CreatedMessageID = reroutedTask.MessageID
	case protocol.BlockerResolutionActionClarify:
		run, err := coordinatorStore.ReadRun(runID)
		if err != nil {
			return err
		}
		messageID, err := Send(stateDir, store, string(blockerCase.CurrentOwner), string(body), SendOpts{
			Kind:   protocol.KindDecision,
			Thread: run.RootThreadID,
			ReplyTo: func() *protocol.MessageID {
				replyTo := blockerCase.CurrentMessageID
				return &replyTo
			}(),
		})
		if err != nil {
			return err
		}
		resolution.CreatedMessageID = messageID
	case protocol.BlockerResolutionActionDismiss:
	default:
		return fmt.Errorf("unsupported action %q", action)
	}

	return coordinatorStore.UpdateBlockerCase(runID, sourceTaskID, func(existing *protocol.BlockerCase) error {
		existing.Status = protocol.BlockerStatusResolved
		existing.Resolution = resolution
		existing.ResolvedAt = &now
		existing.UpdatedAt = now
		existing.CurrentTaskID = blockerCase.CurrentTaskID
		existing.CurrentMessageID = blockerCase.CurrentMessageID
		existing.CurrentOwner = blockerCase.CurrentOwner
		existing.Attempts = blockerCase.Attempts
		return nil
	})
}
