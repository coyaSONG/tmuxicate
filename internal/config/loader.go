package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	for _, kind := range c.Routing.ExclusiveTaskKinds {
		if !isValidKind(kind) {
			return fmt.Errorf("invalid routing.exclusive_task_kinds value %q", kind)
		}
	}
	for _, kind := range c.Routing.FanoutTaskKinds {
		if !isValidKind(kind) {
			return fmt.Errorf("invalid routing.fanout_task_kinds value %q", kind)
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
		if strings.TrimSpace(agent.Role) == "" {
			return fmt.Errorf("%s.role is required", prefix)
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

	if c.Routing.ExclusiveTaskKinds != nil {
		clone.Routing.ExclusiveTaskKinds = append([]protocol.Kind(nil), c.Routing.ExclusiveTaskKinds...)
	}
	if c.Routing.FanoutTaskKinds != nil {
		clone.Routing.FanoutTaskKinds = append([]protocol.Kind(nil), c.Routing.FanoutTaskKinds...)
	}
	if c.Agents != nil {
		clone.Agents = make([]AgentConfig, len(c.Agents))
		copy(clone.Agents, c.Agents)
		for i := range c.Agents {
			if c.Agents[i].Teammates != nil {
				clone.Agents[i].Teammates = append([]string(nil), c.Agents[i].Teammates...)
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

func isValidKind(k protocol.Kind) bool {
	switch k {
	case protocol.KindTask, protocol.KindQuestion, protocol.KindReviewRequest, protocol.KindReviewResponse, protocol.KindDecision, protocol.KindStatusRequest, protocol.KindStatusResponse, protocol.KindNote:
		return true
	default:
		return false
	}
}

func boolPtr(v bool) *bool {
	return &v
}
