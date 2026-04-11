package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"gopkg.in/yaml.v3"
)

type ResolvedConfig struct {
	Config
	ConfigPath string
	ConfigDir  string
}

func Load(path string) (*Config, error) {
	resolved, err := LoadResolved(path)
	if err != nil {
		return nil, err
	}

	return &resolved.Config, nil
}

func LoadResolved(path string) (*ResolvedConfig, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.applyDefaults()

	resolved, err := cfg.Resolve(filepath.Dir(absPath))
	if err != nil {
		return nil, err
	}

	resolved.ConfigPath = absPath
	return resolved, nil
}

func (c *Config) Resolve(workspace string) (*ResolvedConfig, error) {
	if c == nil {
		return nil, errors.New("config is nil")
	}

	cfg := c.clone()
	cfg.applyDefaults()

	baseDir := workspace
	if baseDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		baseDir = cwd
	}

	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}

	cfg.Session.Workspace = resolvePath(absBaseDir, cfg.Session.Workspace)
	cfg.Session.StateDir = resolvePath(absBaseDir, cfg.Session.StateDir)

	if cfg.Transcript.Dir == "" {
		cfg.Transcript.Dir = filepath.Join(cfg.Session.StateDir, "transcripts")
	} else {
		cfg.Transcript.Dir = resolvePath(absBaseDir, cfg.Transcript.Dir)
	}

	if cfg.Defaults.Workdir == "" {
		cfg.Defaults.Workdir = cfg.Session.Workspace
	} else {
		cfg.Defaults.Workdir = resolvePath(absBaseDir, cfg.Defaults.Workdir)
	}

	for i := range cfg.Agents {
		if cfg.Agents[i].Workdir == "" {
			cfg.Agents[i].Workdir = cfg.Defaults.Workdir
		} else {
			cfg.Agents[i].Workdir = resolvePath(absBaseDir, cfg.Agents[i].Workdir)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &ResolvedConfig{
		Config:     cfg,
		ConfigPath: "",
		ConfigDir:  absBaseDir,
	}, nil
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}

	if c.Version != 1 {
		return fmt.Errorf("version must be 1")
	}

	if strings.TrimSpace(c.Session.Name) == "" {
		return errors.New("session.name is required")
	}
	if strings.TrimSpace(c.Session.Workspace) == "" {
		return errors.New("session.workspace is required")
	}
	if strings.TrimSpace(c.Session.StateDir) == "" {
		return errors.New("session.state_dir is required")
	}
	if strings.TrimSpace(c.Session.WindowName) == "" {
		return errors.New("session.window_name is required")
	}
	if !isValidLayout(c.Session.Layout) {
		return fmt.Errorf("invalid session.layout %q", c.Session.Layout)
	}

	if !isValidDeliveryMode(c.Delivery.Mode) {
		return fmt.Errorf("invalid delivery.mode %q", c.Delivery.Mode)
	}
	if c.Delivery.AckTimeout.Std() <= 0 {
		return errors.New("delivery.ack_timeout must be > 0")
	}
	if c.Delivery.RetryInterval.Std() <= 0 {
		return errors.New("delivery.retry_interval must be > 0")
	}
	if c.Delivery.MaxRetries < 0 {
		return errors.New("delivery.max_retries must be >= 0")
	}

	if c.Transcript.Mode != "pipe-pane" {
		return fmt.Errorf("invalid transcript.mode %q", c.Transcript.Mode)
	}
	if strings.TrimSpace(c.Transcript.Dir) == "" {
		return errors.New("transcript.dir is required")
	}

	if strings.TrimSpace(c.Routing.Coordinator) == "" {
		return errors.New("routing.coordinator is required")
	}
	for i := range c.Routing.ExclusiveTaskClasses {
		taskClass := c.Routing.ExclusiveTaskClasses[i]
		if err := taskClass.Validate(); err != nil {
			return fmt.Errorf("invalid routing.exclusive_task_classes value %q: %w", taskClass, err)
		}
	}
	for i := range c.Routing.FanoutTaskClasses {
		taskClass := c.Routing.FanoutTaskClasses[i]
		if err := taskClass.Validate(); err != nil {
			return fmt.Errorf("invalid routing.fanout_task_classes value %q: %w", taskClass, err)
		}
	}
	if c.Routing.Adaptive.LookbackRuns < 0 {
		return errors.New("routing.adaptive.lookback_runs must be >= 0")
	}
	if c.Routing.Adaptive.SuccessWeight < 0 {
		return errors.New("routing.adaptive.success_weight must be >= 0")
	}
	if c.Routing.Adaptive.ApprovalWeight < 0 {
		return errors.New("routing.adaptive.approval_weight must be >= 0")
	}
	if c.Routing.Adaptive.ChangesRequestedPenalty < 0 {
		return errors.New("routing.adaptive.changes_requested_penalty must be >= 0")
	}
	if c.Routing.Adaptive.BlockedPenalty < 0 {
		return errors.New("routing.adaptive.blocked_penalty must be >= 0")
	}
	if c.Routing.Adaptive.WaitPenalty < 0 {
		return errors.New("routing.adaptive.wait_penalty must be >= 0")
	}
	if c.Blockers.MaxReroutesDefault < 0 {
		return errors.New("blockers.max_reroutes_default must be >= 0")
	}
	for taskClass, maxReroutes := range c.Blockers.MaxReroutesByTaskClass {
		if err := taskClass.Validate(); err != nil {
			return fmt.Errorf("blockers.max_reroutes_by_task_class[%q] is invalid: %w", taskClass, err)
		}
		if maxReroutes < 0 {
			return fmt.Errorf("blockers.max_reroutes_by_task_class[%q] must be >= 0", taskClass)
		}
	}

	if strings.TrimSpace(c.Defaults.Workdir) == "" {
		return errors.New("defaults.workdir is required")
	}

	if len(c.Agents) == 0 {
		return errors.New("agents must contain at least one entry")
	}

	knownAgents := make(map[string]struct{}, len(c.Agents))
	knownAliases := make(map[string]struct{}, len(c.Agents))
	knownTargets := make(map[string]struct{}, len(c.ExecutionTargets))
	for i := range c.ExecutionTargets {
		target := &c.ExecutionTargets[i]
		prefix := fmt.Sprintf("execution_targets[%d]", i)

		target.Name = strings.TrimSpace(target.Name)
		if target.Name == "" {
			return fmt.Errorf("%s.name is required", prefix)
		}
		if _, ok := knownTargets[target.Name]; ok {
			return fmt.Errorf("duplicate execution target name %q", target.Name)
		}
		target.Kind = strings.TrimSpace(target.Kind)
		if !isValidExecutionTargetKind(target.Kind) {
			return fmt.Errorf("invalid %s.kind %q", prefix, target.Kind)
		}
		target.Description = strings.TrimSpace(target.Description)

		capabilities, err := normalizeExecutionTargetCapabilities(target.Capabilities)
		if err != nil {
			return fmt.Errorf("%s.capabilities: %w", prefix, err)
		}
		target.Capabilities = capabilities
		knownTargets[target.Name] = struct{}{}
	}
	for i := range c.Agents {
		agent := &c.Agents[i]
		prefix := fmt.Sprintf("agents[%d]", i)

		if strings.TrimSpace(agent.Name) == "" {
			return fmt.Errorf("%s.name is required", prefix)
		}
		if _, ok := knownAgents[agent.Name]; ok {
			return fmt.Errorf("duplicate agent name %q", agent.Name)
		}
		knownAgents[agent.Name] = struct{}{}

		if strings.TrimSpace(agent.Alias) == "" {
			return fmt.Errorf("%s.alias is required", prefix)
		}
		if _, ok := knownAliases[agent.Alias]; ok {
			return fmt.Errorf("duplicate agent alias %q", agent.Alias)
		}
		knownAliases[agent.Alias] = struct{}{}

		if !isValidAdapter(agent.Adapter) {
			return fmt.Errorf("%s.adapter %q is invalid", prefix, agent.Adapter)
		}
		if strings.TrimSpace(agent.Command) == "" {
			return fmt.Errorf("%s.command is required", prefix)
		}
		if !agent.Role.IsDeclared() {
			return fmt.Errorf("%s.role.kind is required", prefix)
		}
		taskClass := protocol.TaskClass(strings.TrimSpace(agent.Role.Kind))
		if err := taskClass.Validate(); err != nil {
			return fmt.Errorf("%s.role.kind %q is invalid: %w", prefix, agent.Role.Kind, err)
		}
		normalizedDomains, err := protocol.NormalizeRouteDomains(agent.Role.Domains)
		if err != nil {
			return fmt.Errorf("%s.role.domains: %w", prefix, err)
		}
		if len(normalizedDomains) == 0 {
			return fmt.Errorf("%s.role.domains must contain at least one domain", prefix)
		}
		agent.Role.Kind = string(taskClass)
		agent.Role.Domains = normalizedDomains
		if agent.RoutePriority < 0 {
			return fmt.Errorf("%s.route_priority must be >= 0", prefix)
		}
		agent.ExecutionTarget = strings.TrimSpace(agent.ExecutionTarget)
		if agent.ExecutionTarget != "" {
			if _, ok := knownTargets[agent.ExecutionTarget]; !ok {
				return fmt.Errorf("%s.execution_target %q references unknown execution target", prefix, agent.ExecutionTarget)
			}
		}
		if strings.TrimSpace(agent.Pane.Slot) == "" {
			return fmt.Errorf("%s.pane.slot is required", prefix)
		}
		if strings.TrimSpace(agent.Workdir) == "" {
			return fmt.Errorf("%s.workdir is required after defaults are applied", prefix)
		}
	}

	if _, ok := knownAgents[c.Routing.Coordinator]; !ok {
		return fmt.Errorf("routing.coordinator %q does not match any agent name", c.Routing.Coordinator)
	}
	for i := range c.Routing.Adaptive.ManualPreferences {
		preference := &c.Routing.Adaptive.ManualPreferences[i]
		prefix := fmt.Sprintf("routing.adaptive.manual_preferences[%d]", i)

		if err := preference.TaskClass.Validate(); err != nil {
			return fmt.Errorf("%s.task_class %q is invalid: %w", prefix, preference.TaskClass, err)
		}
		domains, err := protocol.NormalizeRouteDomains(preference.Domains)
		if err != nil {
			return fmt.Errorf("%s.domains: %w", prefix, err)
		}
		if len(domains) == 0 {
			return fmt.Errorf("%s.domains must contain at least one domain", prefix)
		}
		if strings.TrimSpace(string(preference.PreferredOwner)) == "" {
			return fmt.Errorf("%s.preferred_owner is required", prefix)
		}
		if _, ok := knownAgents[string(preference.PreferredOwner)]; !ok {
			return fmt.Errorf("%s.preferred_owner %q does not match any agent name", prefix, preference.PreferredOwner)
		}
		if preference.Weight < 0 {
			return fmt.Errorf("%s.weight must be >= 0", prefix)
		}
		if strings.TrimSpace(preference.Reason) == "" {
			return fmt.Errorf("%s.reason is required", prefix)
		}

		preference.Domains = domains
	}

	for i := range c.Agents {
		for _, teammate := range c.Agents[i].Teammates {
			if _, ok := knownAgents[teammate]; !ok {
				return fmt.Errorf("agents[%d].teammates references unknown agent %q", i, teammate)
			}
		}
	}

	return nil
}

func (c *Config) applyDefaults() {
	if c.Version == 0 {
		c.Version = 1
	}

	if c.Session.WindowName == "" {
		c.Session.WindowName = "agents"
	}
	if c.Session.Layout == "" {
		c.Session.Layout = "triad"
	}
	if c.Session.Attach == nil {
		c.Session.Attach = boolPtr(true)
	}

	if c.Delivery.Mode == "" {
		c.Delivery.Mode = "notify_then_read"
	}
	if c.Delivery.AckTimeout.Std() == 0 {
		c.Delivery.AckTimeout = Duration(2 * 60 * 1e9)
	}
	if c.Delivery.RetryInterval.Std() == 0 {
		c.Delivery.RetryInterval = Duration(30 * 1e9)
	}
	if c.Delivery.MaxRetries == 0 {
		c.Delivery.MaxRetries = 3
	}
	if c.Delivery.SafeNotifyOnlyWhenReady == nil {
		c.Delivery.SafeNotifyOnlyWhenReady = boolPtr(true)
	}
	if c.Delivery.AutoNotify == nil {
		c.Delivery.AutoNotify = boolPtr(true)
	}
	if !c.Blockers.maxReroutesDefaultSet && c.Blockers.MaxReroutesDefault == 0 {
		c.Blockers.MaxReroutesDefault = 1
	}

	if c.Transcript.Mode == "" {
		c.Transcript.Mode = "pipe-pane"
	}

	if c.Defaults.Workdir == "" {
		c.Defaults.Workdir = c.Session.Workspace
	}
	if c.Defaults.Env == nil {
		c.Defaults.Env = map[string]string{}
	}
	if c.Defaults.BootstrapTemplate == "" {
		c.Defaults.BootstrapTemplate = "default"
	}
	if c.Defaults.Notify.Enabled == nil {
		c.Defaults.Notify.Enabled = boolPtr(true)
	}

	for i := range c.Agents {
		if c.Agents[i].Workdir == "" {
			c.Agents[i].Workdir = c.Defaults.Workdir
		}
	}

	if c.Transcript.Dir == "" && c.Session.StateDir != "" {
		c.Transcript.Dir = filepath.Join(c.Session.StateDir, "transcripts")
	}
}

func (c *Config) clone() Config {
	clone := *c

	if c.Defaults.Env != nil {
		clone.Defaults.Env = make(map[string]string, len(c.Defaults.Env))
		for k, v := range c.Defaults.Env {
			clone.Defaults.Env[k] = v
		}
	}

	if c.Routing.ExclusiveTaskClasses != nil {
		clone.Routing.ExclusiveTaskClasses = append([]protocol.TaskClass(nil), c.Routing.ExclusiveTaskClasses...)
	}
	if c.Routing.FanoutTaskClasses != nil {
		clone.Routing.FanoutTaskClasses = append([]protocol.TaskClass(nil), c.Routing.FanoutTaskClasses...)
	}
	if c.Routing.Adaptive.ManualPreferences != nil {
		clone.Routing.Adaptive.ManualPreferences = make([]AdaptiveManualPreference, len(c.Routing.Adaptive.ManualPreferences))
		copy(clone.Routing.Adaptive.ManualPreferences, c.Routing.Adaptive.ManualPreferences)
		for i := range c.Routing.Adaptive.ManualPreferences {
			if c.Routing.Adaptive.ManualPreferences[i].Domains != nil {
				clone.Routing.Adaptive.ManualPreferences[i].Domains = append([]string(nil), c.Routing.Adaptive.ManualPreferences[i].Domains...)
			}
		}
	}
	if c.Blockers.MaxReroutesByTaskClass != nil {
		clone.Blockers.MaxReroutesByTaskClass = make(map[protocol.TaskClass]int, len(c.Blockers.MaxReroutesByTaskClass))
		for taskClass, maxReroutes := range c.Blockers.MaxReroutesByTaskClass {
			clone.Blockers.MaxReroutesByTaskClass[taskClass] = maxReroutes
		}
	}
	if c.ExecutionTargets != nil {
		clone.ExecutionTargets = make([]ExecutionTargetConfig, len(c.ExecutionTargets))
		copy(clone.ExecutionTargets, c.ExecutionTargets)
		for i := range c.ExecutionTargets {
			if c.ExecutionTargets[i].Capabilities != nil {
				clone.ExecutionTargets[i].Capabilities = append([]string(nil), c.ExecutionTargets[i].Capabilities...)
			}
		}
	}
	if c.Agents != nil {
		clone.Agents = make([]AgentConfig, len(c.Agents))
		copy(clone.Agents, c.Agents)
		for i := range c.Agents {
			if c.Agents[i].Teammates != nil {
				clone.Agents[i].Teammates = append([]string(nil), c.Agents[i].Teammates...)
			}
			if c.Agents[i].Role.Domains != nil {
				clone.Agents[i].Role.Domains = append([]string(nil), c.Agents[i].Role.Domains...)
			}
		}
	}

	return clone
}

func resolvePath(base, p string) string {
	if strings.TrimSpace(p) == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(base, p))
}

func isValidAdapter(s string) bool {
	switch s {
	case "generic", "codex", "claude-code":
		return true
	default:
		return false
	}
}

func isValidDeliveryMode(s string) bool {
	switch s {
	case "notify_then_read", "manual":
		return true
	default:
		return false
	}
}

func isValidLayout(s string) bool {
	switch s {
	case "triad", "tiled", "main-vertical", "main-horizontal", "even-horizontal", "even-vertical":
		return true
	default:
		return false
	}
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

func boolPtr(v bool) *bool {
	return &v
}
