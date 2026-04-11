package config

import (
	"fmt"
	"strings"
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
	Blockers   BlockersConfig   `yaml:"blockers"`
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
	Coordinator          string               `yaml:"coordinator"`
	ExclusiveTaskClasses []protocol.TaskClass `yaml:"exclusive_task_classes"`
	FanoutTaskClasses    []protocol.TaskClass `yaml:"fanout_task_classes"`
	Adaptive             AdaptiveRoutingConfig `yaml:"adaptive"`
}

type AdaptiveRoutingConfig struct {
	Enabled                 bool                       `yaml:"enabled"`
	LookbackRuns            int                        `yaml:"lookback_runs"`
	SuccessWeight           int                        `yaml:"success_weight"`
	ApprovalWeight          int                        `yaml:"approval_weight"`
	ChangesRequestedPenalty int                        `yaml:"changes_requested_penalty"`
	BlockedPenalty          int                        `yaml:"blocked_penalty"`
	WaitPenalty             int                        `yaml:"wait_penalty"`
	ManualPreferences       []AdaptiveManualPreference `yaml:"manual_preferences"`
}

type AdaptiveManualPreference struct {
	TaskClass      protocol.TaskClass `yaml:"task_class"`
	Domains        []string           `yaml:"domains"`
	PreferredOwner protocol.AgentName `yaml:"preferred_owner"`
	Weight         int                `yaml:"weight"`
	Reason         string             `yaml:"reason"`
}

type BlockersConfig struct {
	MaxReroutesDefault     int                        `yaml:"max_reroutes_default"`
	MaxReroutesByTaskClass map[protocol.TaskClass]int `yaml:"max_reroutes_by_task_class"`

	maxReroutesDefaultSet bool `yaml:"-"`
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

type RoleSpec struct {
	Kind        string   `yaml:"kind"`
	Domains     []string `yaml:"domains,omitempty"`
	Description string   `yaml:"description,omitempty"`
}

func (r RoleSpec) IsDeclared() bool {
	return strings.TrimSpace(r.Kind) != ""
}

func (r RoleSpec) String() string {
	kind := strings.TrimSpace(r.Kind)
	if kind == "" {
		return ""
	}
	if len(r.Domains) == 0 {
		return kind
	}

	return fmt.Sprintf("%s [%s]", kind, strings.Join(r.Domains, ", "))
}

type AgentConfig struct {
	Name          string          `yaml:"name"`
	Alias         string          `yaml:"alias"`
	Adapter       string          `yaml:"adapter"`
	Command       string          `yaml:"command"`
	Role          RoleSpec        `yaml:"role"`
	RoutePriority int             `yaml:"route_priority"`
	Pane          PaneConfig      `yaml:"pane"`
	Teammates     []string        `yaml:"teammates"`
	Bootstrap     BootstrapConfig `yaml:"bootstrap"`
	Workdir       string          `yaml:"workdir,omitempty"`
}

type BootstrapConfig struct {
	ExtraInstructions string `yaml:"extra_instructions"`
}

type PaneConfig struct {
	Slot string `yaml:"slot"`
}

func (b *BlockersConfig) UnmarshalYAML(unmarshal func(any) error) error {
	if b == nil {
		return nil
	}

	var raw struct {
		MaxReroutesDefault     *int                       `yaml:"max_reroutes_default"`
		MaxReroutesByTaskClass map[protocol.TaskClass]int `yaml:"max_reroutes_by_task_class"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	b.MaxReroutesDefault = 0
	b.maxReroutesDefaultSet = false
	if raw.MaxReroutesDefault != nil {
		b.MaxReroutesDefault = *raw.MaxReroutesDefault
		b.maxReroutesDefaultSet = true
	}
	if raw.MaxReroutesByTaskClass != nil {
		b.MaxReroutesByTaskClass = make(map[protocol.TaskClass]int, len(raw.MaxReroutesByTaskClass))
		for taskClass, maxReroutes := range raw.MaxReroutesByTaskClass {
			b.MaxReroutesByTaskClass[taskClass] = maxReroutes
		}
	} else {
		b.MaxReroutesByTaskClass = nil
	}

	return nil
}
