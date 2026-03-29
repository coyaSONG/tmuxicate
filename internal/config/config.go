package config

import (
	"fmt"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

type Duration time.Duration

func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var raw string
	if err := unmarshal(&raw); err == nil {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", raw, err)
		}
		*d = Duration(parsed)
		return nil
	}

	var nanos int64
	if err := unmarshal(&nanos); err == nil {
		*d = Duration(time.Duration(nanos))
		return nil
	}

	return fmt.Errorf("invalid duration value")
}

func (d Duration) Std() time.Duration {
	return time.Duration(d)
}

func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

type Config struct {
	Version    int              `yaml:"version"`
	Session    SessionConfig    `yaml:"session"`
	Delivery   DeliveryConfig   `yaml:"delivery"`
	Transcript TranscriptConfig `yaml:"transcript"`
	Routing    RoutingConfig    `yaml:"routing"`
	Defaults   DefaultsConfig   `yaml:"defaults"`
	Agents     []AgentConfig    `yaml:"agents"`
}

type SessionConfig struct {
	Name       string `yaml:"name"`
	Workspace  string `yaml:"workspace"`
	StateDir   string `yaml:"state_dir"`
	WindowName string `yaml:"window_name"`
	Layout     string `yaml:"layout"`
	Attach     *bool  `yaml:"attach,omitempty"`
}

type DeliveryConfig struct {
	Mode                    string   `yaml:"mode"`
	AckTimeout              Duration `yaml:"ack_timeout"`
	RetryInterval           Duration `yaml:"retry_interval"`
	MaxRetries              int      `yaml:"max_retries"`
	SafeNotifyOnlyWhenReady *bool    `yaml:"safe_notify_only_when_ready,omitempty"`
	AutoNotify              *bool    `yaml:"auto_notify,omitempty"`
}

type TranscriptConfig struct {
	Mode string `yaml:"mode"`
	Dir  string `yaml:"dir"`
}

type RoutingConfig struct {
	Coordinator        string          `yaml:"coordinator"`
	ExclusiveTaskKinds []protocol.Kind `yaml:"exclusive_task_kinds"`
	FanoutTaskKinds    []protocol.Kind `yaml:"fanout_task_kinds"`
}

type DefaultsConfig struct {
	Workdir           string            `yaml:"workdir"`
	Env               map[string]string `yaml:"env"`
	BootstrapTemplate string            `yaml:"bootstrap_template"`
	Notify            NotifyConfig      `yaml:"notify"`
}

type NotifyConfig struct {
	Enabled *bool `yaml:"enabled,omitempty"`
}

type AgentConfig struct {
	Name      string          `yaml:"name"`
	Alias     string          `yaml:"alias"`
	Adapter   string          `yaml:"adapter"`
	Command   string          `yaml:"command"`
	Role      string          `yaml:"role"`
	Pane      PaneConfig      `yaml:"pane"`
	Teammates []string        `yaml:"teammates"`
	Bootstrap BootstrapConfig `yaml:"bootstrap"`
	Workdir   string          `yaml:"workdir,omitempty"`
}

type BootstrapConfig struct {
	ExtraInstructions string `yaml:"extra_instructions"`
}

type PaneConfig struct {
	Slot string `yaml:"slot"`
}
