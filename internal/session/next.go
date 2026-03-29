package session

import (
	"errors"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

var ErrNoUnreadMessages = errors.New("no unread messages")

func Next(stateDir, agent string) (*ReadResult, error) {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return nil, err
	}

	agentName, err := resolveTargetAgent(cfg, agent)
	if err != nil {
		return nil, err
	}

	entries, err := Inbox(stateDir, agentName, true)
	if err != nil {
		return nil, err
	}

	for i := range entries {
		if entries[i].State == protocol.FolderStateUnread {
			return ReadMsg(stateDir, agentName, entries[i].MessageID)
		}
	}

	return nil, ErrNoUnreadMessages
}
