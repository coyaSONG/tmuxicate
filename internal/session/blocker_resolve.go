package session

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
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

type BlockerResolveOpts struct {
	RunID          protocol.RunID
	SourceTaskID   protocol.TaskID
	Action         protocol.BlockerResolutionAction
	Owner          string
	Reason         string
	Body           []byte
	TaskClass      protocol.TaskClass
	Domains        []string
	Goal           string
	ExpectedOutput string
}

func (o BlockerResolveOpts) Validate() error {
	if o.RunID == "" {
		return fmt.Errorf("run_id is required")
	}
	if o.SourceTaskID == "" {
		return fmt.Errorf("source_task_id is required")
	}
	if err := o.Action.Validate(); err != nil {
		return fmt.Errorf("action: %w", err)
	}

	trimmedReason := strings.TrimSpace(o.Reason)
	if blockerResolutionRequiresReason[o.Action] || o.Action == protocol.BlockerResolutionActionPartialReplan {
		if trimmedReason == "" {
			return fmt.Errorf("reason is required")
		}
	}
	if blockerResolutionRequiresBody[o.Action] {
		if len(o.Body) == 0 || strings.TrimSpace(string(o.Body)) == "" {
			return fmt.Errorf("clarify requires body")
		}
	}
	if o.Action == protocol.BlockerResolutionActionPartialReplan {
		missing := make([]string, 0, 4)
		if strings.TrimSpace(string(o.TaskClass)) == "" {
			missing = append(missing, "task-class")
		}
		if len(o.Domains) == 0 {
			missing = append(missing, "domains")
		}
		if strings.TrimSpace(o.Goal) == "" {
			missing = append(missing, "goal")
		}
		if strings.TrimSpace(o.ExpectedOutput) == "" {
			missing = append(missing, "expected-output")
		}
		if len(missing) > 0 {
			return fmt.Errorf("partial_replan requires %s", strings.Join(missing, ", "))
		}
		if err := o.TaskClass.Validate(); err != nil {
			return fmt.Errorf("task_class: %w", err)
		}
		if o.TaskClass == protocol.TaskClassReview {
			return fmt.Errorf("partial_replan task_class cannot be review")
		}
		if _, err := protocol.NormalizeRouteDomains(o.Domains); err != nil {
			return fmt.Errorf("domains: %w", err)
		}
	}

	return nil
}

func BlockerResolve(stateDir string, store *mailbox.Store, opts BlockerResolveOpts) error {
	if store == nil {
		return fmt.Errorf("store is required")
	}
	if err := opts.Validate(); err != nil {
		return err
	}

	trimmedReason := strings.TrimSpace(opts.Reason)
	coordinatorStore := mailbox.NewCoordinatorStore(stateDir)
	blockerCase, err := coordinatorStore.ReadBlockerCase(opts.RunID, opts.SourceTaskID)
	if err != nil {
		return err
	}
	if blockerCase.Status != protocol.BlockerStatusEscalated {
		return fmt.Errorf("blocker case %s is not escalated", opts.SourceTaskID)
	}

	now := time.Now().UTC()
	resolution := &protocol.BlockerResolution{
		Action:     opts.Action,
		ResolvedBy: protocol.AgentName("human"),
		Note:       trimmedReason,
		CreatedAt:  now,
	}

	switch opts.Action {
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
			runID:            opts.RunID,
			coordinatorStore: coordinatorStore,
		}

		sourceTask, err := coordinatorStore.ReadTask(opts.RunID, blockerCase.SourceTaskID)
		if err != nil {
			return err
		}
		currentTask, err := coordinatorStore.ReadTask(opts.RunID, blockerCase.CurrentTaskID)
		if err != nil {
			return err
		}
		coordinatorTask.sourceTask = sourceTask
		coordinatorTask.currentTask = currentTask

		overrideOwner := protocol.AgentName(strings.TrimSpace(opts.Owner))
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
		run, err := coordinatorStore.ReadRun(opts.RunID)
		if err != nil {
			return err
		}
		messageID, err := Send(stateDir, store, string(blockerCase.CurrentOwner), string(opts.Body), SendOpts{
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
	case protocol.BlockerResolutionActionPartialReplan:
		if _, err := coordinatorStore.ReadPartialReplan(opts.RunID, opts.SourceTaskID); err == nil {
			return fmt.Errorf("partial replan for %s already exists", opts.SourceTaskID)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		cfg, err := loadResolvedConfigFromStateDir(stateDir)
		if err != nil {
			return err
		}
		sourceTask, err := coordinatorStore.ReadTask(opts.RunID, blockerCase.SourceTaskID)
		if err != nil {
			return err
		}
		currentTask, err := coordinatorStore.ReadTask(opts.RunID, blockerCase.CurrentTaskID)
		if err != nil {
			return err
		}

		replacementTask, err := createPartialReplanTask(stateDir, store, cfg, coordinatorStore, sourceTask, currentTask, opts)
		if err != nil {
			return err
		}

		replan := &protocol.PartialReplan{
			RunID:                opts.RunID,
			SourceTaskID:         sourceTask.TaskID,
			SourceMessageID:      sourceTask.MessageID,
			BlockerSourceTaskID:  blockerCase.SourceTaskID,
			SupersededTaskID:     currentTask.TaskID,
			SupersededMessageID:  currentTask.MessageID,
			SupersededOwner:      currentTask.Owner,
			ReplacementTaskID:    replacementTask.TaskID,
			ReplacementMessageID: replacementTask.MessageID,
			ReplacementOwner:     replacementTask.Owner,
			Reason:               trimmedReason,
			Status:               protocol.PartialReplanStatusApplied,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := coordinatorStore.CreatePartialReplan(replan); err != nil {
			return err
		}

		blockerCase.CurrentTaskID = replacementTask.TaskID
		blockerCase.CurrentMessageID = replacementTask.MessageID
		blockerCase.CurrentOwner = replacementTask.Owner
		resolution.CreatedTaskID = replacementTask.TaskID
		resolution.CreatedMessageID = replacementTask.MessageID
	case protocol.BlockerResolutionActionDismiss:
	default:
		return fmt.Errorf("unsupported action %q", opts.Action)
	}

	return coordinatorStore.UpdateBlockerCase(opts.RunID, opts.SourceTaskID, func(existing *protocol.BlockerCase) error {
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

func createPartialReplanTask(stateDir string, store *mailbox.Store, cfg *config.ResolvedConfig, coordinatorStore *mailbox.CoordinatorStore, sourceTask *protocol.ChildTask, currentTask *protocol.ChildTask, opts BlockerResolveOpts) (*protocol.ChildTask, error) {
	if cfg == nil {
		return nil, fmt.Errorf("resolved config is required")
	}
	if coordinatorStore == nil {
		return nil, fmt.Errorf("coordinator store is required")
	}
	if sourceTask == nil {
		return nil, fmt.Errorf("source task is required")
	}
	if currentTask == nil {
		return nil, fmt.Errorf("current task is required")
	}

	restoreReceipt, err := suspendTaskReceiptForReroute(store, string(currentTask.Owner), currentTask.MessageID)
	if err != nil {
		return nil, err
	}

	req := protocol.RouteChildTaskRequest{
		RunID:          opts.RunID,
		TaskClass:      opts.TaskClass,
		Domains:        opts.Domains,
		Goal:           strings.TrimSpace(opts.Goal),
		ExpectedOutput: strings.TrimSpace(opts.ExpectedOutput),
		ReviewRequired: sourceTask.ReviewRequired,
	}
	if overrideOwner := strings.TrimSpace(opts.Owner); overrideOwner != "" {
		req.OwnerOverride = protocol.AgentName(overrideOwner)
		req.OverrideReason = strings.TrimSpace(opts.Reason)
	}

	replacementTask, _, err := RouteChildTask(cfg, store, req)
	if err != nil {
		if restoreErr := restoreReceipt(); restoreErr != nil {
			return nil, fmt.Errorf("restore superseded receipt after partial replan failure: %w", restoreErr)
		}
		return nil, err
	}

	return replacementTask, nil
}
