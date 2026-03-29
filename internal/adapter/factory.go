package adapter

import (
	"fmt"

	"github.com/coyaSONG/tmuxicate/internal/tmux"
)

func NewAdapter(adapterType string, client tmux.Client, paneID string) (Adapter, error) {
	switch adapterType {
	case "generic":
		return NewGenericAdapter(client, paneID, GenericConfig{})
	case "claude-code":
		return NewClaudeCodeAdapter(client, paneID)
	case "codex":
		return NewCodexAdapter(client, paneID)
	default:
		return nil, fmt.Errorf("unknown adapter type %q", adapterType)
	}
}
