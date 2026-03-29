package session

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func Reply(stateDir string, store *mailbox.Store, agent string, parentID protocol.MessageID, body []byte) (protocol.MessageID, error) {
	if store == nil {
		return "", fmt.Errorf("store is required")
	}
	if strings.TrimSpace(agent) == "" {
		return "", fmt.Errorf("agent is required")
	}
	if len(body) == 0 || strings.TrimSpace(string(body)) == "" {
		return "", fmt.Errorf("body is required")
	}

	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return "", err
	}

	agentName, err := resolveTargetAgent(cfg, agent)
	if err != nil {
		return "", err
	}

	parentEnv, _, err := store.ReadMessage(parentID)
	if err != nil {
		return "", err
	}

	payload := body
	if len(payload) == 0 || payload[len(payload)-1] != '\n' {
		payload = append(payload, '\n')
	}

	seq, err := store.AllocateSeq()
	if err != nil {
		return "", err
	}
	msgID := protocol.NewMessageID(seq)
	sum := sha256.Sum256(payload)

	replyTo := parentEnv.ID
	env := protocol.Envelope{
		Schema:      protocol.MessageSchemaV1,
		ID:          msgID,
		Seq:         seq,
		Session:     parentEnv.Session,
		Thread:      parentEnv.Thread,
		Kind:        replyKind(parentEnv.Kind),
		From:        protocol.AgentName(agentName),
		To:          []protocol.AgentName{parentEnv.From},
		CreatedAt:   time.Now().UTC(),
		BodyFormat:  protocol.BodyFormatMD,
		BodySHA256:  fmt.Sprintf("%x", sum[:]),
		BodyBytes:   int64(len(payload)),
		ReplyTo:     &replyTo,
		Priority:    protocol.PriorityNormal,
		RequiresAck: true,
	}

	if err := store.CreateMessage(env, payload); err != nil {
		return "", err
	}

	receipt := protocol.Receipt{
		Schema:         protocol.ReceiptSchemaV1,
		MessageID:      msgID,
		Seq:            seq,
		Recipient:      parentEnv.From,
		FolderState:    protocol.FolderStateUnread,
		Revision:       0,
		NotifyAttempts: 0,
	}
	if err := store.CreateReceipt(receipt); err != nil {
		return "", err
	}

	return msgID, nil
}

func replyKind(parent protocol.Kind) protocol.Kind {
	switch parent {
	case protocol.KindReviewRequest:
		return protocol.KindReviewResponse
	case protocol.KindStatusRequest:
		return protocol.KindStatusResponse
	default:
		return protocol.KindNote
	}
}
