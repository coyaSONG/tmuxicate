package mailbox

import (
	"fmt"
	"path/filepath"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

const (
	envelopeFileName   = "envelope.yaml"
	bodyFileName       = "body.md"
	stagingDirName     = ".staging"
	orphanedDirName    = "orphaned"
	coordinatorDirName = "coordinator"
	targetsDirName     = "targets"
	runsDirName        = "runs"
	runFileName        = "run.yaml"
	tasksDirName       = "tasks"
	reviewsDirName     = "reviews"
	blockersDirName    = "blockers"
	replansDirName     = "replans"
	preferencesDirName = "preferences"
	adaptiveDirName    = "adaptive-routing"
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

func TargetLocksDir(stateDir string) string {
	return filepath.Join(LocksDir(stateDir), "targets")
}

func TargetLockPath(stateDir, target string) string {
	return filepath.Join(TargetLocksDir(stateDir), fmt.Sprintf("%s.lock", target))
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

func CoordinatorDir(stateDir string) string {
	return filepath.Join(SessionDir(stateDir), coordinatorDirName)
}

func RunsDir(stateDir string) string {
	return filepath.Join(CoordinatorDir(stateDir), runsDirName)
}

func RunDir(stateDir string, runID protocol.RunID) string {
	return filepath.Join(RunsDir(stateDir), string(runID))
}

func RunFilePath(stateDir string, runID protocol.RunID) string {
	return filepath.Join(RunDir(stateDir, runID), runFileName)
}

func RunTasksDir(stateDir string, runID protocol.RunID) string {
	return filepath.Join(RunDir(stateDir, runID), tasksDirName)
}

func RunTaskPath(stateDir string, runID protocol.RunID, taskID protocol.TaskID) string {
	return filepath.Join(RunTasksDir(stateDir, runID), fmt.Sprintf("%s.yaml", taskID))
}

func RunReviewsDir(stateDir string, runID protocol.RunID) string {
	return filepath.Join(RunDir(stateDir, runID), reviewsDirName)
}

func RunReviewHandoffPath(stateDir string, runID protocol.RunID, sourceTaskID protocol.TaskID) string {
	return filepath.Join(RunReviewsDir(stateDir, runID), fmt.Sprintf("%s.yaml", sourceTaskID))
}

func RunBlockersDir(stateDir string, runID protocol.RunID) string {
	return filepath.Join(RunDir(stateDir, runID), blockersDirName)
}

func RunBlockerCasePath(stateDir string, runID protocol.RunID, sourceTaskID protocol.TaskID) string {
	return filepath.Join(RunBlockersDir(stateDir, runID), fmt.Sprintf("%s.yaml", sourceTaskID))
}

func RunPartialReplansDir(stateDir string, runID protocol.RunID) string {
	return filepath.Join(RunDir(stateDir, runID), replansDirName)
}

func RunPartialReplanPath(stateDir string, runID protocol.RunID, sourceTaskID protocol.TaskID) string {
	return filepath.Join(RunPartialReplansDir(stateDir, runID), fmt.Sprintf("%s.yaml", sourceTaskID))
}

func RunLocksDir(stateDir string, runID protocol.RunID) string {
	return filepath.Join(RunDir(stateDir, runID), "locks")
}

func RunRouteLockPath(stateDir string, runID protocol.RunID) string {
	return filepath.Join(RunLocksDir(stateDir, runID), "route.lock")
}

func AdaptiveRoutingPreferencesDir(stateDir string) string {
	return filepath.Join(CoordinatorDir(stateDir), preferencesDirName, adaptiveDirName)
}

func AdaptiveRoutingPreferencesPath(stateDir string, coordinator protocol.AgentName) string {
	return filepath.Join(AdaptiveRoutingPreferencesDir(stateDir), fmt.Sprintf("%s.yaml", coordinator))
}

func TargetsDir(stateDir string) string {
	return filepath.Join(SessionDir(stateDir), targetsDirName)
}

func TargetDir(stateDir, target string) string {
	return filepath.Join(TargetsDir(stateDir), target)
}

func TargetEventsDir(stateDir, target string) string {
	return filepath.Join(TargetDir(stateDir, target), "events")
}

func TargetStatePath(stateDir, target string) string {
	return filepath.Join(TargetDir(stateDir, target), "health.current.json")
}

func TargetHealthLogPath(stateDir, target string) string {
	return filepath.Join(TargetEventsDir(stateDir, target), "health.jsonl")
}

func TargetDispatchesDir(stateDir, target string) string {
	return filepath.Join(TargetDir(stateDir, target), "dispatches")
}

func TargetDispatchPath(stateDir, target string, msgID protocol.MessageID) string {
	return filepath.Join(TargetDispatchesDir(stateDir, target), fmt.Sprintf("%s.json", msgID))
}

func TargetDispatchLogPath(stateDir, target string) string {
	return filepath.Join(TargetEventsDir(stateDir, target), "dispatch.jsonl")
}
