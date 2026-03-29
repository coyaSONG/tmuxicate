package adapter

import (
	"context"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

type ReadyState string

const (
	ReadyStateReady   ReadyState = "ready"
	ReadyStateBusy    ReadyState = "busy"
	ReadyStateUnknown ReadyState = "unknown"
	ReadyStateExited  ReadyState = "exited"
)

type Adapter interface {
	Bootstrap(ctx context.Context) error
	Probe(ctx context.Context) (ReadyState, error)
	Notify(ctx context.Context, ref MessageRef) error
	Interrupt(ctx context.Context, reason string) error
}

type MessageRef struct {
	ID      protocol.MessageID
	From    string
	Subject string
}
