package session

import (
	"errors"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func BlockerResolve(stateDir string, store *mailbox.Store, runID protocol.RunID, sourceTaskID protocol.TaskID, action protocol.BlockerResolutionAction, owner string, reason string, body []byte) error {
	return errors.New("blocker resolve not implemented")
}
