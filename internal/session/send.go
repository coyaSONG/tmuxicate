package session

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

type SendOpts struct {
	Subject  string
	Kind     protocol.Kind
	Priority protocol.Priority
	Thread   protocol.ThreadID
	ReplyTo  *protocol.MessageID
}

func Send(stateDir string, store *mailbox.Store, to string, body string, opts SendOpts) (protocol.MessageID, error) {
	if store == nil {
		return "", fmt.Errorf("store is required")
	}
	if strings.TrimSpace(to) == "" {
		return "", fmt.Errorf("target is required")
	}
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("body is required")
	}

	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return "", err
	}

	targetName, err := resolveTargetAgent(cfg, to)
	if err != nil {
		return "", err
	}

	sender := os.Getenv("TMUXICATE_AGENT")
	if strings.TrimSpace(sender) == "" {
		sender = "human"
	}

	seq, err := store.AllocateSeq()
	if err != nil {
		return "", err
	}

	msgID := protocol.NewMessageID(seq)
	threadID := opts.Thread
	if threadID == "" {
		threadID = protocol.NewThreadID(seq)
	}

	payload := []byte(body)
	if !strings.HasSuffix(body, "\n") {
		payload = append(payload, '\n')
	}
	sum := sha256.Sum256(payload)

	kind := opts.Kind
	if kind == "" {
		kind = protocol.KindNote
	}
	priority := opts.Priority
	if priority == "" {
		priority = protocol.PriorityNormal
	}

	env := protocol.Envelope{
		Schema:      protocol.MessageSchemaV1,
		ID:          msgID,
		Seq:         seq,
		Session:     cfg.Session.Name,
		Thread:      threadID,
		Kind:        kind,
		From:        protocol.AgentName(sender),
		To:          []protocol.AgentName{protocol.AgentName(targetName)},
		CreatedAt:   time.Now().UTC(),
		BodyFormat:  protocol.BodyFormatMD,
		BodySHA256:  fmt.Sprintf("%x", sum[:]),
		BodyBytes:   int64(len(payload)),
		Subject:     opts.Subject,
		Priority:    priority,
		RequiresAck: true,
		ReplyTo:     opts.ReplyTo,
	}

	if err := store.CreateMessage(env, payload); err != nil {
		return "", err
	}

	receipt := protocol.Receipt{
		Schema:         protocol.ReceiptSchemaV1,
		MessageID:      msgID,
		Seq:            seq,
		Recipient:      protocol.AgentName(targetName),
		FolderState:    protocol.FolderStateUnread,
		Revision:       0,
		NotifyAttempts: 0,
	}
	if err := store.CreateReceipt(receipt); err != nil {
		return "", err
	}

	return msgID, nil
}

func resolveTargetAgent(cfg *config.ResolvedConfig, target string) (string, error) {
	for _, agent := range cfg.Agents {
		if agent.Name == target || agent.Alias == target {
			return agent.Name, nil
		}
	}
	return "", fmt.Errorf("unknown target agent %q", target)
}
