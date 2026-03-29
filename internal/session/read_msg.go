package session

import (
	"fmt"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

type ReadResult struct {
	MessageID     protocol.MessageID
	Seq           int64
	Thread        protocol.ThreadID
	From          protocol.AgentName
	To            []protocol.AgentName
	Kind          protocol.Kind
	Priority      protocol.Priority
	Subject       string
	CreatedAt     time.Time
	RequiresClaim bool
	Attachments   []protocol.Attachment
	Body          string
	State         protocol.FolderState
}

func ReadMsg(stateDir, agent string, msgID protocol.MessageID) (*ReadResult, error) {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return nil, err
	}

	agentName, err := resolveTargetAgent(cfg, agent)
	if err != nil {
		return nil, err
	}

	store := mailbox.NewStore(stateDir)
	receipt, err := store.ReadReceipt(agentName, msgID)
	if err != nil {
		return nil, fmt.Errorf("read receipt: %w", err)
	}

	env, body, err := store.ReadMessage(msgID)
	if err != nil {
		return nil, err
	}

	if receipt.FolderState == protocol.FolderStateUnread {
		now := time.Now().UTC()
		if err := store.UpdateReceipt(agentName, msgID, func(r *protocol.Receipt) {
			r.AckedAt = &now
			r.Revision++
		}); err != nil {
			return nil, err
		}
		if err := store.MoveReceipt(agentName, msgID, protocol.FolderStateUnread, protocol.FolderStateActive); err != nil {
			return nil, err
		}
		receipt.FolderState = protocol.FolderStateActive
		receipt.AckedAt = &now
	}

	priority := env.Priority
	if priority == "" {
		priority = protocol.PriorityNormal
	}

	return &ReadResult{
		MessageID:     env.ID,
		Seq:           env.Seq,
		Thread:        env.Thread,
		From:          env.From,
		To:            env.To,
		Kind:          env.Kind,
		Priority:      priority,
		Subject:       env.Subject,
		CreatedAt:     env.CreatedAt,
		RequiresClaim: env.RequiresClaim,
		Attachments:   env.Attachments,
		Body:          string(body),
		State:         receipt.FolderState,
	}, nil
}
