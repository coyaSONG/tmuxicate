package adapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/tmux"
)

type ClaudeCodeAdapter struct {
	*GenericAdapter
}

func NewClaudeCodeAdapter(client tmux.Client, paneID string) (*ClaudeCodeAdapter, error) {
	g, err := NewGenericAdapter(client, paneID, GenericConfig{
		ReadyRegex:    `(?m)^❯\s*$`,
		QuietPeriod:   1200 * time.Millisecond,
		BootstrapMode: BootstrapModeArg,
	})
	if err != nil {
		return nil, err
	}
	return &ClaudeCodeAdapter{GenericAdapter: g}, nil
}

var _ Adapter = (*ClaudeCodeAdapter)(nil)

func (a *ClaudeCodeAdapter) Notify(ctx context.Context, ref MessageRef) error {
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
	msg += fmt.Sprintf(". Please run `tmuxicate read %s` using the shell tool", ref.ID)
	if strings.TrimSpace(ref.Subject) != "" {
		msg += fmt.Sprintf(" (%s)", ref.Subject)
	}
	msg += ", then reply through tmuxicate."

	return a.tmux.SendKeys(ctx, a.paneID, msg, true)
}
