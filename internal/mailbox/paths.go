package mailbox

import (
	"fmt"
	"path/filepath"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

const (
	envelopeFileName = "envelope.yaml"
	bodyFileName     = "body.md"
	stagingDirName   = ".staging"
	orphanedDirName  = "orphaned"
)

func SessionDir(stateDir string) string {
	return filepath.Clean(stateDir)
}

func StateDir(stateDir string) string {
	return filepath.Join(SessionDir(stateDir), "state")
}

func NextSeqPath(stateDir string) string {
	return filepath.Join(StateDir(stateDir), "next-seq")
}

func MessagesDir(stateDir string) string {
	return filepath.Join(SessionDir(stateDir), "messages")
}

func StagingDir(stateDir string) string {
	return filepath.Join(MessagesDir(stateDir), stagingDirName)
}

func OrphanedMessagesDir(stateDir string) string {
	return filepath.Join(MessagesDir(stateDir), orphanedDirName)
}

func MessageDir(stateDir string, msgID protocol.MessageID) string {
	return filepath.Join(MessagesDir(stateDir), string(msgID))
}

func EnvelopePath(stateDir string, msgID protocol.MessageID) string {
	return filepath.Join(MessageDir(stateDir, msgID), envelopeFileName)
}

func BodyPath(stateDir string, msgID protocol.MessageID) string {
	return filepath.Join(MessageDir(stateDir, msgID), bodyFileName)
}

func AgentDir(stateDir, agent string) string {
	return filepath.Join(SessionDir(stateDir), "agents", agent)
}

func InboxBaseDir(stateDir, agent string) string {
	return filepath.Join(AgentDir(stateDir, agent), "inbox")
}

func InboxDir(stateDir, agent string, folder protocol.FolderState) string {
	return filepath.Join(InboxBaseDir(stateDir, agent), string(folder))
}

func LocksDir(stateDir string) string {
	return filepath.Join(SessionDir(stateDir), "locks")
}

func SequenceLockPath(stateDir string) string {
	return filepath.Join(LocksDir(stateDir), "sequence.lock")
}

func ReceiptLocksDir(stateDir, agent string) string {
	return filepath.Join(LocksDir(stateDir), "receipts", agent)
}

func ReceiptLockPath(stateDir, agent string, msgID protocol.MessageID) string {
	return filepath.Join(ReceiptLocksDir(stateDir, agent), fmt.Sprintf("%s.lock", msgID))
}

func ReceiptFileName(receipt *protocol.Receipt) string {
	return fmt.Sprintf("%010d-%s.yaml", receipt.Seq, receipt.MessageID)
}

func ReceiptPath(stateDir, agent string, folder protocol.FolderState, receipt *protocol.Receipt) string {
	return filepath.Join(InboxDir(stateDir, agent, folder), ReceiptFileName(receipt))
}

func ReceiptGlob(stateDir, agent string, msgID protocol.MessageID) string {
	return filepath.Join(InboxBaseDir(stateDir, agent), "*", fmt.Sprintf("*-%s.yaml", msgID))
}
