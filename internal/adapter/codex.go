package adapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/tmux"
)

type CodexAdapter struct {
	*GenericAdapter
}

func NewCodexAdapter(client tmux.Client, paneID string) (*CodexAdapter, error) {
	g, err := NewGenericAdapter(client, paneID, &GenericConfig{
		ReadyRegex:    `(?m)^›(?:\s|$)`,
		QuietPeriod:   1500 * time.Millisecond,
		BootstrapMode: BootstrapModeArg,
	})
	if err != nil {
		return nil, err
	}
	return &CodexAdapter{GenericAdapter: g}, nil
}

var _ Adapter = (*CodexAdapter)(nil)

func (a *CodexAdapter) Notify(ctx context.Context, ref MessageRef) error {
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
	msg += fmt.Sprintf(". Please use the shell tool to run `tmuxicate read %s`", ref.ID)
	if strings.TrimSpace(ref.Subject) != "" {
		msg += fmt.Sprintf(" (%s)", ref.Subject)
	}
	msg += ", then respond via tmuxicate."

	return a.tmux.SendKeys(ctx, a.paneID, msg, true)
}
