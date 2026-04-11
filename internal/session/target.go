package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

type TargetStatus struct {
	Name              string
	Kind              string
	PaneBacked        bool
	Availability      mailbox.TargetAvailability
	Summary           string
	Source            string
	LastHeartbeat     *time.Time
	LastDispatch      *time.Time
	PendingDispatches int
	FailedDispatches  int
	LastError         string
	DisabledReason    string
}

func ListTargetStatuses(stateDir string) ([]TargetStatus, error) {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return nil, err
	}

	targets, err := configuredExecutionTargets(cfg)
	if err != nil {
		return nil, err
	}

	statuses := make([]TargetStatus, 0, len(targets))
	for _, target := range targets {
		report, err := loadTargetStatusReport(stateDir, cfg, target)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, report)
	}

	slices.SortFunc(statuses, func(a, b TargetStatus) int {
		return strings.Compare(a.Name, b.Name)
	})
	return statuses, nil
}

func TargetHeartbeat(stateDir, targetName string, availability mailbox.TargetAvailability, summary string, capabilities []string) (*TargetStatus, error) {
	cfg, target, targetCfg, err := resolveNamedExecutionTarget(stateDir, targetName)
	if err != nil {
		return nil, err
	}
	recorded, err := mailbox.RecordTargetHeartbeat(stateDir, target, availability, summary, "heartbeat", capabilities)
	if err != nil {
		return nil, err
	}
	report, err := buildTargetStatus(cfg, targetCfg, recorded)
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func DisableTarget(stateDir, targetName, reason string) (*TargetStatus, error) {
	cfg, target, targetCfg, err := resolveNamedExecutionTarget(stateDir, targetName)
	if err != nil {
		return nil, err
	}
	recorded, err := mailbox.UpsertTargetState(stateDir, target, func(state *mailbox.TargetState) error {
		state.Availability = mailbox.TargetAvailabilityDisabled
		state.Summary = "target disabled by operator"
		state.Source = "operator"
		state.DisabledReason = strings.TrimSpace(reason)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, err := mailbox.RecordTargetHeartbeat(stateDir, target, recorded.Availability, recorded.Summary, recorded.Source, nil); err != nil {
		return nil, err
	}
	report, err := buildTargetStatus(cfg, targetCfg, recorded)
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func EnableTarget(stateDir, targetName, reason string) (*TargetStatus, int, error) {
	cfg, target, targetCfg, err := resolveNamedExecutionTarget(stateDir, targetName)
	if err != nil {
		return nil, 0, err
	}
	recorded, err := mailbox.UpsertTargetState(stateDir, target, func(state *mailbox.TargetState) error {
		state.Availability = mailbox.TargetAvailabilityReady
		state.Summary = "target enabled by operator"
		state.Source = "operator"
		state.DisabledReason = ""
		if strings.TrimSpace(reason) != "" {
			state.Summary = fmt.Sprintf("target enabled by operator: %s", strings.TrimSpace(reason))
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	if _, err := mailbox.RecordTargetHeartbeat(stateDir, target, recorded.Availability, recorded.Summary, recorded.Source, nil); err != nil {
		return nil, 0, err
	}
	redispatched, err := dispatchPendingForTarget(stateDir, cfg, targetCfg)
	if err != nil {
		return nil, 0, err
	}
	report, err := buildTargetStatus(cfg, targetCfg, recorded)
	return &report, redispatched, err
}

func loadTargetStatusReport(stateDir string, cfg *config.ResolvedConfig, target config.ExecutionTargetConfig) (TargetStatus, error) {
	recorded, err := mailbox.ReadTargetState(stateDir, target.Name)
	if err != nil && !os.IsNotExist(err) {
		return TargetStatus{}, err
	}
	return buildTargetStatus(cfg, target, recorded)
}

func buildTargetStatus(cfg *config.ResolvedConfig, targetCfg config.ExecutionTargetConfig, recorded *mailbox.TargetState) (TargetStatus, error) {
	target := protocol.ExecutionTarget{
		Name:         targetCfg.Name,
		Kind:         targetCfg.Kind,
		Description:  targetCfg.Description,
		Capabilities: append([]string(nil), targetCfg.Capabilities...),
		PaneBacked:   targetCfg.PaneBacked,
	}
	if recorded == nil {
		recorded = mailbox.DefaultTargetState(target)
	}
	availability, summary := effectiveTargetAvailability(targetCfg, recorded)
	dispatches, err := mailbox.ListTargetDispatches(cfg.Session.StateDir, targetCfg.Name)
	if err != nil {
		return TargetStatus{}, err
	}
	report := TargetStatus{
		Name:           targetCfg.Name,
		Kind:           targetCfg.Kind,
		PaneBacked:     targetCfg.PaneBacked,
		Availability:   availability,
		Summary:        summary,
		Source:         recorded.Source,
		LastError:      recorded.LastError,
		DisabledReason: recorded.DisabledReason,
	}
	if ts, _ := parseTimePtr(recorded.LastHeartbeatAt); ts != nil {
		report.LastHeartbeat = ts
	}
	if ts, _ := parseTimePtr(recorded.LastDispatchAt); ts != nil {
		report.LastDispatch = ts
	}
	for _, dispatch := range dispatches {
		switch dispatch.Status {
		case mailbox.TargetDispatchPending:
			report.PendingDispatches++
		case mailbox.TargetDispatchFailed:
			report.FailedDispatches++
		}
	}
	return report, nil
}

func effectiveTargetAvailability(targetCfg config.ExecutionTargetConfig, recorded *mailbox.TargetState) (mailbox.TargetAvailability, string) {
	if recorded == nil {
		return mailbox.TargetAvailabilityUnknown, "target has not reported health yet"
	}
	if recorded.Availability == mailbox.TargetAvailabilityDisabled {
		summary := recorded.Summary
		if strings.TrimSpace(recorded.DisabledReason) != "" {
			summary = fmt.Sprintf("%s (%s)", normalizeDisplayValue(summary), recorded.DisabledReason)
		}
		return recorded.Availability, strings.TrimSpace(summary)
	}
	if timeout := targetCfg.Health.HeartbeatTimeout.Std(); timeout > 0 && strings.TrimSpace(recorded.LastHeartbeatAt) != "" {
		if ts, err := time.Parse(time.RFC3339Nano, recorded.LastHeartbeatAt); err == nil && time.Since(ts) > timeout {
			return mailbox.TargetAvailabilityOffline, fmt.Sprintf("heartbeat expired after %s", timeout.Round(time.Second))
		}
	}
	return recorded.Availability, normalizeDisplayValue(recorded.Summary)
}

func configuredExecutionTargets(cfg *config.ResolvedConfig) ([]config.ExecutionTargetConfig, error) {
	targets := make([]config.ExecutionTargetConfig, 0, len(cfg.ExecutionTargets)+1)
	includeImplicitLocal := false
	for _, agent := range cfg.Agents {
		target, err := resolveExecutionTarget(cfg, &agent)
		if err != nil {
			return nil, err
		}
		if target.Name == "local" && target.Kind == "local" && target.PaneBacked {
			includeImplicitLocal = true
		}
	}
	if includeImplicitLocal {
		targets = append(targets, config.ExecutionTargetConfig{
			Name:         "local",
			Kind:         "local",
			Description:  "Implicit local pane-backed execution target",
			Capabilities: []string{"local", "pane"},
			PaneBacked:   true,
		})
	}
	targets = append(targets, cfg.ExecutionTargets...)
	return targets, nil
}

func resolveNamedExecutionTarget(stateDir, targetName string) (*config.ResolvedConfig, protocol.ExecutionTarget, config.ExecutionTargetConfig, error) {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return nil, protocol.ExecutionTarget{}, config.ExecutionTargetConfig{}, err
	}
	targets, err := configuredExecutionTargets(cfg)
	if err != nil {
		return nil, protocol.ExecutionTarget{}, config.ExecutionTargetConfig{}, err
	}
	for _, targetCfg := range targets {
		if targetCfg.Name != targetName {
			continue
		}
		target := protocol.ExecutionTarget{
			Name:         targetCfg.Name,
			Kind:         targetCfg.Kind,
			Description:  targetCfg.Description,
			Capabilities: append([]string(nil), targetCfg.Capabilities...),
			PaneBacked:   targetCfg.PaneBacked,
		}
		return cfg, target, targetCfg, nil
	}
	return nil, protocol.ExecutionTarget{}, config.ExecutionTargetConfig{}, fmt.Errorf("unknown execution target %q", targetName)
}

func dispatchPendingForTarget(stateDir string, cfg *config.ResolvedConfig, targetCfg config.ExecutionTargetConfig) (int, error) {
	count := 0
	for i := range cfg.Agents {
		agent := &cfg.Agents[i]
		if strings.TrimSpace(agent.ExecutionTarget) != targetCfg.Name {
			continue
		}
		dir := mailbox.InboxDir(stateDir, agent.Name, protocol.FolderStateUnread)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return count, err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}
			msgID := protocol.MessageID(extractMessageID(entry.Name()))
			record, err := mailbox.ReadTargetDispatch(stateDir, targetCfg.Name, msgID)
			if err == nil && record.Status == mailbox.TargetDispatchDispatched {
				continue
			}
			if err != nil && !os.IsNotExist(err) {
				return count, err
			}

			store := mailbox.NewStore(stateDir)
			env, _, err := store.ReadMessage(msgID)
			if err != nil {
				return count, err
			}
			runID := protocol.RunID(strings.TrimSpace(env.Meta["parent_run_id"]))
			taskID := protocol.TaskID(strings.TrimSpace(env.Meta["task_id"]))
			if runID == "" || taskID == "" {
				continue
			}
			task, err := mailbox.NewCoordinatorStore(stateDir).ReadTask(runID, taskID)
			if err != nil {
				return count, err
			}
			if err := dispatchNonLocalTask(cfg, targetCfg, agent, task); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

func dispatchNonLocalTask(cfg *config.ResolvedConfig, targetCfg config.ExecutionTargetConfig, owner *config.AgentConfig, task *protocol.ChildTask) error {
	if cfg == nil || owner == nil || task == nil {
		return fmt.Errorf("config, owner, and task are required")
	}
	target := protocol.ExecutionTarget{
		Name:         targetCfg.Name,
		Kind:         targetCfg.Kind,
		Description:  targetCfg.Description,
		Capabilities: append([]string(nil), targetCfg.Capabilities...),
		PaneBacked:   targetCfg.PaneBacked,
	}
	if target.PaneBacked {
		return nil
	}

	now := time.Now().UTC()
	record := &mailbox.TargetDispatchRecord{
		Schema:     "tmuxicate/target-dispatch/v1",
		TargetName: target.Name,
		TargetKind: target.Kind,
		Agent:      owner.Name,
		RunID:      string(task.ParentRunID),
		TaskID:     string(task.TaskID),
		MessageID:  string(task.MessageID),
		Command:    targetCfg.Dispatch.Command,
		Status:     mailbox.TargetDispatchPending,
		Summary:    "dispatch pending",
		CreatedAt:  now.Format(time.RFC3339Nano),
		UpdatedAt:  now.Format(time.RFC3339Nano),
	}
	if existing, err := mailbox.ReadTargetDispatch(cfg.Session.StateDir, target.Name, task.MessageID); err == nil {
		record.CreatedAt = existing.CreatedAt
	}

	command := strings.TrimSpace(targetCfg.Dispatch.Command)
	if command == "" {
		record.Summary = "dispatch command not configured; manual dispatch required"
		return mailbox.WriteTargetDispatch(cfg.Session.StateDir, record)
	}

	cmd := exec.CommandContext(context.Background(), "bash", "-lc", command)
	cmd.Dir = dispatchWorkingDir(cfg, targetCfg, owner)
	cmd.Env = append(os.Environ(), dispatchEnv(cfg, targetCfg, owner, task)...)
	output, err := cmd.CombinedOutput()
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err != nil {
		record.Status = mailbox.TargetDispatchFailed
		record.Error = strings.TrimSpace(string(output))
		if record.Error == "" {
			record.Error = err.Error()
		}
		record.Summary = "dispatch command failed"
		if writeErr := mailbox.WriteTargetDispatch(cfg.Session.StateDir, record); writeErr != nil {
			return writeErr
		}
		_, stateErr := mailbox.UpsertTargetState(cfg.Session.StateDir, target, func(state *mailbox.TargetState) error {
			state.Availability = mailbox.TargetAvailabilityDegraded
			state.Summary = "last dispatch failed"
			state.Source = "dispatch"
			state.LastDispatchAt = record.UpdatedAt
			state.LastError = record.Error
			return nil
		})
		if stateErr != nil {
			return stateErr
		}
		return nil
	}

	record.Status = mailbox.TargetDispatchDispatched
	record.DispatchedAt = record.UpdatedAt
	record.Summary = "dispatch command completed"
	if text := strings.TrimSpace(string(output)); text != "" {
		record.Summary = text
	}
	if err := mailbox.WriteTargetDispatch(cfg.Session.StateDir, record); err != nil {
		return err
	}
	_, err = mailbox.UpsertTargetState(cfg.Session.StateDir, target, func(state *mailbox.TargetState) error {
		state.Availability = mailbox.TargetAvailabilityReady
		state.Summary = "last dispatch succeeded"
		state.Source = "dispatch"
		state.LastDispatchAt = record.DispatchedAt
		state.LastError = ""
		return nil
	})
	return err
}

func targetAvailabilityForRouting(stateDir string, targetCfg config.ExecutionTargetConfig) (mailbox.TargetAvailability, string, error) {
	target := protocol.ExecutionTarget{
		Name:         targetCfg.Name,
		Kind:         targetCfg.Kind,
		Description:  targetCfg.Description,
		Capabilities: append([]string(nil), targetCfg.Capabilities...),
		PaneBacked:   targetCfg.PaneBacked,
	}
	state, err := mailbox.ReadTargetState(stateDir, targetCfg.Name)
	if err != nil {
		if !os.IsNotExist(err) {
			return mailbox.TargetAvailabilityUnknown, "", err
		}
		state = mailbox.DefaultTargetState(target)
	}
	availability, summary := effectiveTargetAvailability(targetCfg, state)
	return availability, summary, nil
}

func dispatchWorkingDir(cfg *config.ResolvedConfig, targetCfg config.ExecutionTargetConfig, owner *config.AgentConfig) string {
	if strings.TrimSpace(targetCfg.Dispatch.Workdir) != "" {
		return targetCfg.Dispatch.Workdir
	}
	if strings.TrimSpace(owner.Workdir) != "" {
		return owner.Workdir
	}
	return cfg.Session.Workspace
}

func dispatchEnv(cfg *config.ResolvedConfig, targetCfg config.ExecutionTargetConfig, owner *config.AgentConfig, task *protocol.ChildTask) []string {
	values := make([]string, 0, len(targetCfg.Dispatch.Env)+11)
	for key, value := range targetCfg.Dispatch.Env {
		values = append(values, fmt.Sprintf("%s=%s", key, value))
	}
	values = append(values,
		fmt.Sprintf("TMUXICATE_SESSION=%s", cfg.Session.Name),
		fmt.Sprintf("TMUXICATE_STATE_DIR=%s", cfg.Session.StateDir),
		fmt.Sprintf("TMUXICATE_AGENT=%s", owner.Name),
		fmt.Sprintf("TMUXICATE_ALIAS=%s", owner.Alias),
		fmt.Sprintf("TMUXICATE_TARGET=%s", targetCfg.Name),
		fmt.Sprintf("TMUXICATE_TARGET_KIND=%s", targetCfg.Kind),
		fmt.Sprintf("TMUXICATE_TARGET_CAPABILITIES=%s", strings.Join(targetCfg.Capabilities, ",")),
		fmt.Sprintf("TMUXICATE_RUN_ID=%s", task.ParentRunID),
		fmt.Sprintf("TMUXICATE_TASK_ID=%s", task.TaskID),
		fmt.Sprintf("TMUXICATE_MESSAGE_ID=%s", task.MessageID),
		fmt.Sprintf("TMUXICATE_RUN_SCRIPT=%s", filepath.Join(mailbox.AgentDir(cfg.Session.StateDir, owner.Name), "adapter", "run.sh")),
	)
	return values
}
