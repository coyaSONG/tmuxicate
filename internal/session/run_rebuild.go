package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"gopkg.in/yaml.v3"
)

type RunGraph struct {
	Run   protocol.CoordinatorRun
	Tasks []RunGraphTask
}

type RunGraphTask struct {
	Task          protocol.ChildTask
	ReceiptState  protocol.FolderState
	DeclaredState string
}

type runMessageSummary struct {
	Thread protocol.ThreadID
}

func LoadRunGraph(stateDir string, runID protocol.RunID) (*RunGraph, error) {
	store := mailbox.NewCoordinatorStore(stateDir)
	run, err := store.ReadRun(runID)
	if err != nil {
		return nil, err
	}

	messages, err := loadRunMessages(stateDir)
	if err != nil {
		return nil, fmt.Errorf("scan messages: %w", err)
	}

	rootMessage, ok := messages[run.RootMessageID]
	if !ok {
		return nil, coordinatorArtifactMismatch("run %s references missing root message %s", run.RunID, run.RootMessageID)
	}
	if rootMessage.Thread != run.RootThreadID {
		return nil, coordinatorArtifactMismatch("run %s root thread mismatch for message %s", run.RunID, run.RootMessageID)
	}

	tasks, err := loadRunTasks(stateDir, run.RunID)
	if err != nil {
		return nil, err
	}

	graph := &RunGraph{
		Run:   *run,
		Tasks: make([]RunGraphTask, 0, len(tasks)),
	}
	taskByID := make(map[protocol.TaskID]RunGraphTask, len(tasks))
	for _, task := range tasks {
		if task.ParentRunID != run.RunID {
			return nil, coordinatorArtifactMismatch("task %s belongs to run %s, not %s", task.TaskID, task.ParentRunID, run.RunID)
		}

		message, ok := messages[task.MessageID]
		if !ok {
			return nil, coordinatorArtifactMismatch("task %s references missing message %s", task.TaskID, task.MessageID)
		}
		if message.Thread != task.ThreadID {
			return nil, coordinatorArtifactMismatch("task %s thread mismatch for message %s", task.TaskID, task.MessageID)
		}
		if task.ThreadID != run.RootThreadID {
			return nil, coordinatorArtifactMismatch("task %s thread %s does not match run root thread %s", task.TaskID, task.ThreadID, run.RootThreadID)
		}

		receiptState, err := loadTaskReceiptState(stateDir, string(task.Owner), task.MessageID)
		if err != nil {
			return nil, err
		}

		declaredState, _, err := readDeclaredState(filepath.Join(stateDir, "agents", string(task.Owner), "events", "state.current.json"))
		if err != nil {
			return nil, fmt.Errorf("read declared state for %s: %w", task.Owner, err)
		}

		node := RunGraphTask{
			Task:          task,
			ReceiptState:  receiptState,
			DeclaredState: declaredState,
		}
		graph.Tasks = append(graph.Tasks, node)
		taskByID[task.TaskID] = node
	}

	for _, node := range graph.Tasks {
		for _, dependency := range node.Task.DependsOn {
			if _, ok := taskByID[dependency]; !ok {
				return nil, coordinatorArtifactMismatch("task %s depends on missing task %s", node.Task.TaskID, dependency)
			}
		}
	}

	sort.Slice(graph.Tasks, func(i, j int) bool {
		return graph.Tasks[i].Task.TaskID < graph.Tasks[j].Task.TaskID
	})

	return graph, nil
}

func FormatRunGraph(graph *RunGraph) string {
	if graph == nil {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Run: %s\n", graph.Run.RunID)
	fmt.Fprintf(&builder, "Coordinator: %s\n", graph.Run.Coordinator)
	fmt.Fprintf(&builder, "Goal: %s\n", graph.Run.Goal)
	fmt.Fprintf(&builder, "Root Message: %s\n", graph.Run.RootMessageID)
	for _, task := range graph.Tasks {
		fmt.Fprintf(&builder, "\nTask: %s\n", task.Task.TaskID)
		fmt.Fprintf(&builder, "Owner: %s\n", task.Task.Owner)
		fmt.Fprintf(&builder, "Goal: %s\n", task.Task.Goal)
		fmt.Fprintf(&builder, "Expected Output: %s\n", task.Task.ExpectedOutput)
		fmt.Fprintf(&builder, "Depends On: %s\n", formatDependsOn(task.Task.DependsOn))
		fmt.Fprintf(&builder, "State: %s [%s]\n", normalizeDisplayValue(task.DeclaredState), normalizeDisplayValue(string(task.ReceiptState)))
		fmt.Fprintf(&builder, "Message: %s\n", task.Task.MessageID)
	}

	return builder.String()
}

func loadRunMessages(stateDir string) (map[protocol.MessageID]runMessageSummary, error) {
	messages := make(map[protocol.MessageID]runMessageSummary)
	if err := scanMessages(stateDir, func(path string) error {
		data, err := os.ReadFile(filepath.Join(path, "envelope.yaml"))
		if err != nil {
			return err
		}

		var envelope struct {
			ID     protocol.MessageID `yaml:"id"`
			Thread protocol.ThreadID  `yaml:"thread"`
		}
		if err := yaml.Unmarshal(data, &envelope); err != nil {
			return err
		}
		if envelope.ID == "" {
			return nil
		}

		messages[envelope.ID] = runMessageSummary{Thread: envelope.Thread}
		return nil
	}); err != nil {
		return nil, err
	}

	return messages, nil
}

func loadRunTasks(stateDir string, runID protocol.RunID) ([]protocol.ChildTask, error) {
	entries, err := os.ReadDir(mailbox.RunTasksDir(stateDir, runID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read run tasks dir: %w", err)
	}

	tasks := make([]protocol.ChildTask, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		path := filepath.Join(mailbox.RunTasksDir(stateDir, runID), entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read task yaml: %w", err)
		}

		var task protocol.ChildTask
		if err := yaml.Unmarshal(data, &task); err != nil {
			return nil, fmt.Errorf("unmarshal task yaml: %w", err)
		}
		if err := task.Validate(); err != nil {
			return nil, fmt.Errorf("validate task yaml: %w", err)
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

func loadTaskReceiptState(stateDir, agent string, msgID protocol.MessageID) (protocol.FolderState, error) {
	var (
		found bool
		state protocol.FolderState
	)

	if err := scanReceiptsForAgent(stateDir, agent, func(folder string, _ string, receipt receiptSummary) {
		if receipt.MessageID != string(msgID) {
			return
		}
		found = true
		state = protocol.FolderState(folder)
	}); err != nil {
		return "", fmt.Errorf("scan receipts for %s: %w", agent, err)
	}

	if !found {
		return "", coordinatorArtifactMismatch("task message %s has no receipt for owner %s", msgID, agent)
	}

	return state, nil
}

func coordinatorArtifactMismatch(format string, args ...any) error {
	return fmt.Errorf("coordinator artifact mismatch: "+format, args...)
}

func formatDependsOn(dependsOn []protocol.TaskID) string {
	if len(dependsOn) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(dependsOn))
	for _, dependency := range dependsOn {
		parts = append(parts, string(dependency))
	}

	return strings.Join(parts, ", ")
}

func normalizeDisplayValue(value string) string {
	if strings.TrimSpace(value) == "" || value == "-" {
		return "-"
	}

	return value
}
