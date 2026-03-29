package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

type InboxEntry struct {
	MessageID protocol.MessageID
	Seq       int64
	Priority  protocol.Priority
	State     protocol.FolderState
	Kind      protocol.Kind
	From      protocol.AgentName
	Thread    protocol.ThreadID
	Age       time.Duration
	Subject   string
}

func Inbox(stateDir, agent string, unreadOnly bool) ([]InboxEntry, error) {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return nil, err
	}

	agentName, err := resolveTargetAgent(cfg, agent)
	if err != nil {
		return nil, err
	}

	store := mailbox.NewStore(stateDir)
	folders := []protocol.FolderState{protocol.FolderStateUnread, protocol.FolderStateActive}
	if !unreadOnly {
		folders = append(folders, protocol.FolderStateDone)
	}

	var entries []InboxEntry
	for _, folder := range folders {
		dir := mailbox.InboxDir(stateDir, agentName, folder)
		files, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read inbox folder %s: %w", dir, err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			msgID := protocol.MessageID(extractMessageID(file.Name()))
			receipt, err := store.ReadReceipt(agentName, msgID)
			if err != nil {
				return nil, err
			}
			env, _, err := store.ReadMessage(msgID)
			if err != nil {
				return nil, err
			}

			priority := env.Priority
			if priority == "" {
				priority = protocol.PriorityNormal
			}

			entries = append(entries, InboxEntry{
				MessageID: msgID,
				Seq:       env.Seq,
				Priority:  priority,
				State:     receipt.FolderState,
				Kind:      env.Kind,
				From:      env.From,
				Thread:    env.Thread,
				Age:       time.Since(env.CreatedAt).Round(time.Second),
				Subject:   env.Subject,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		left := priorityRank(entries[i].Priority)
		right := priorityRank(entries[j].Priority)
		if left != right {
			return left > right
		}
		if entries[i].Seq != entries[j].Seq {
			return entries[i].Seq < entries[j].Seq
		}
		return entries[i].MessageID < entries[j].MessageID
	})

	return entries, nil
}

func priorityRank(priority protocol.Priority) int {
	switch priority {
	case protocol.PriorityUrgent:
		return 4
	case protocol.PriorityHigh:
		return 3
	case protocol.PriorityNormal, "":
		return 2
	case protocol.PriorityLow:
		return 1
	default:
		return 0
	}
}

func stateEventsDir(stateDir, agent string) string {
	return filepath.Join(mailbox.AgentDir(stateDir, agent), "events")
}
