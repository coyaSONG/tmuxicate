package adapter

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/tmux"
)

type BootstrapMode string

const (
	BootstrapModeArg   BootstrapMode = "arg"
	BootstrapModePaste BootstrapMode = "paste"
	BootstrapModeNone  BootstrapMode = "none"
)

type GenericConfig struct {
	Command       string
	ReadyRegex    string
	BusyRegex     string
	QuietPeriod   time.Duration
	BootstrapMode BootstrapMode
	BootstrapText string
}

type GenericAdapter struct {
	tmux   tmux.Client
	paneID string
	cfg    GenericConfig

	readyRe *regexp.Regexp
	busyRe  *regexp.Regexp

	mu           sync.Mutex
	lastSnapshot string
	lastChanged  time.Time
}

func NewGenericAdapter(client tmux.Client, paneID string, cfg *GenericConfig) (*GenericAdapter, error) {
	adapter := &GenericAdapter{
		tmux:        client,
		paneID:      paneID,
		cfg:         *cfg,
		lastChanged: time.Now(),
	}

	if cfg.ReadyRegex != "" {
		re, err := regexp.Compile(cfg.ReadyRegex)
		if err != nil {
			return nil, fmt.Errorf("compile ready regex: %w", err)
		}
		adapter.readyRe = re
	}
	if cfg.BusyRegex != "" {
		re, err := regexp.Compile(cfg.BusyRegex)
		if err != nil {
			return nil, fmt.Errorf("compile busy regex: %w", err)
		}
		adapter.busyRe = re
	}

	return adapter, nil
}

var _ Adapter = (*GenericAdapter)(nil)

func (a *GenericAdapter) Bootstrap(ctx context.Context) error {
	switch a.cfg.BootstrapMode {
	case BootstrapModeNone, BootstrapModeArg:
		return nil
	case BootstrapModePaste:
		if strings.TrimSpace(a.cfg.BootstrapText) == "" {
			return nil
		}
		if err := a.tmux.SetBuffer(ctx, a.cfg.BootstrapText); err != nil {
			return fmt.Errorf("set tmux buffer: %w", err)
		}
		if err := a.tmux.PasteBuffer(ctx, a.paneID); err != nil {
			return fmt.Errorf("paste tmux buffer: %w", err)
		}
		if err := a.tmux.SendKeys(ctx, a.paneID, "", true); err != nil {
			return fmt.Errorf("send bootstrap enter: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported bootstrap mode %q", a.cfg.BootstrapMode)
	}
}

func (a *GenericAdapter) Probe(ctx context.Context) (ReadyState, error) {
	snapshot, err := a.tmux.CapturePane(ctx, a.paneID, 40)
	if err != nil {
		return ReadyStateExited, nil
	}

	now := time.Now()

	a.mu.Lock()
	if snapshot != a.lastSnapshot {
		a.lastSnapshot = snapshot
		a.lastChanged = now
	}
	lastChanged := a.lastChanged
	a.mu.Unlock()

	if a.busyRe != nil && a.busyRe.MatchString(snapshot) {
		return ReadyStateBusy, nil
	}

	if a.readyRe != nil && !a.readyRe.MatchString(snapshot) {
		return ReadyStateUnknown, nil
	}

	if a.cfg.QuietPeriod > 0 && now.Sub(lastChanged) < a.cfg.QuietPeriod {
		return ReadyStateUnknown, nil
	}

	return ReadyStateReady, nil
}

func (a *GenericAdapter) Notify(ctx context.Context, ref MessageRef) error {
	state, err := a.Probe(ctx)
	if err != nil {
		return err
	}
	if state != ReadyStateReady {
		return fmt.Errorf("pane %s is not ready: %s", a.paneID, state)
	}

	msg := fmt.Sprintf("[tmuxicate] New message %s", ref.ID)
	if ref.From != "" {
		msg += fmt.Sprintf(" from %s", ref.From)
	}
	msg += fmt.Sprintf(". Please run `tmuxicate read %s`", ref.ID)
	if strings.TrimSpace(ref.Subject) != "" {
		msg += fmt.Sprintf(" (%s)", ref.Subject)
	}
	msg += " and reply via tmuxicate."

	return a.tmux.SendKeys(ctx, a.paneID, msg, true)
}

func (a *GenericAdapter) Interrupt(ctx context.Context, reason string) error {
	msg := "[tmuxicate] Interrupt requested."
	if strings.TrimSpace(reason) != "" {
		msg = fmt.Sprintf("%s %s", msg, strings.TrimSpace(reason))
	}
	return a.tmux.SendKeys(ctx, a.paneID, msg, true)
}
