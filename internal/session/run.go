package session

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func Run(cfg *config.ResolvedConfig, store *mailbox.Store, req RunRequest) (*protocol.CoordinatorRun, error) {
	if cfg == nil {
		return nil, fmt.Errorf("resolved config is required")
	}
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if err := req.Validate(cfg); err != nil {
		return nil, err
	}

	coordinator, err := resolveAgentConfig(cfg, req.Coordinator)
	if err != nil {
		return nil, err
	}

	allowedOwners, teamSnapshot := routingBaseline(cfg, coordinator)
	if len(allowedOwners) == 0 {
		return nil, fmt.Errorf("allowed_owners must contain at least one teammate with declared role")
	}
	if len(teamSnapshot) == 0 {
		return nil, fmt.Errorf("team_snapshot must include coordinator routing metadata")
	}
	runSeq, err := store.AllocateSeq()
	if err != nil {
		return nil, fmt.Errorf("allocate run sequence: %w", err)
	}
	messageSeq, err := store.AllocateSeq()
	if err != nil {
		return nil, fmt.Errorf("allocate root message sequence: %w", err)
	}

	run := protocol.CoordinatorRun{
		RunID:         protocol.NewRunID(runSeq),
		Goal:          strings.TrimSpace(req.Goal),
		Coordinator:   protocol.AgentName(coordinator.Name),
		CreatedBy:     protocol.AgentName(strings.TrimSpace(req.CreatedBy)),
		CreatedAt:     time.Now().UTC(),
		RootMessageID: protocol.NewMessageID(messageSeq),
		RootThreadID:  protocol.NewThreadID(messageSeq),
		AllowedOwners: allowedOwners,
		TeamSnapshot:  teamSnapshot,
	}

	// Build the canonical root contract with `## Decomposition Instructions`,
	// `## Run References`, and the `tmuxicate run add-task --run` command prefix.
	body, err := BuildRunRootMessageBody(RunRootMessageInput{Run: run})
	if err != nil {
		return nil, err
	}

	coordinatorStore := mailbox.NewCoordinatorStore(cfg.Session.StateDir)
	if err := coordinatorStore.CreateRun(&run); err != nil {
		return nil, err
	}

	if err := createWorkflowMessage(cfg, store, workflowMessageInput{
		Seq:           messageSeq,
		MessageID:     run.RootMessageID,
		ThreadID:      run.RootThreadID,
		From:          run.CreatedBy,
		To:            protocol.AgentName(coordinator.Name),
		Subject:       fmt.Sprintf("Coordinator run %s: %s", run.RunID, summarizeSubject(run.Goal)),
		Body:          body,
		Kind:          protocol.KindTask,
		RequiresClaim: true,
		Meta: map[string]string{
			"run_id":          string(run.RunID),
			"root_message_id": string(run.RootMessageID),
			"root_thread_id":  string(run.RootThreadID),
		},
	}); err != nil {
		return nil, err
	}

	return &run, nil
}

func AddChildTask(cfg *config.ResolvedConfig, store *mailbox.Store, req ChildTaskRequest) (*protocol.ChildTask, error) {
	if cfg == nil {
		return nil, fmt.Errorf("resolved config is required")
	}
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	coordinatorStore := mailbox.NewCoordinatorStore(cfg.Session.StateDir)
	run, err := coordinatorStore.ReadRun(req.ParentRunID)
	if err != nil {
		return nil, err
	}

	owner, err := resolveAgentConfig(cfg, req.Owner)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(owner.Role) == "" {
		return nil, fmt.Errorf("owner %q must have declared role metadata", req.Owner)
	}

	coordinator, err := findAgentByName(cfg, string(run.Coordinator))
	if err != nil {
		return nil, err
	}
	if !containsString(coordinator.Teammates, owner.Name) {
		return nil, fmt.Errorf("owner %q is not an allowed owner for coordinator %q", owner.Name, coordinator.Name)
	}
	if !containsAgentName(run.AllowedOwners, owner.Name) {
		return nil, fmt.Errorf("owner %q is not an allowed owner for run %q", owner.Name, run.RunID)
	}

	taskSeq, err := store.AllocateSeq()
	if err != nil {
		return nil, fmt.Errorf("allocate task sequence: %w", err)
	}
	messageSeq, err := store.AllocateSeq()
	if err != nil {
		return nil, fmt.Errorf("allocate task message sequence: %w", err)
	}

	task := protocol.ChildTask{
		TaskID:         protocol.NewTaskID(taskSeq),
		ParentRunID:    req.ParentRunID,
		Owner:          protocol.AgentName(owner.Name),
		Goal:           strings.TrimSpace(req.Goal),
		ExpectedOutput: strings.TrimSpace(req.ExpectedOutput),
		DependsOn:      slices.Clone(req.DependsOn),
		ReviewRequired: req.ReviewRequired,
		MessageID:      protocol.NewMessageID(messageSeq),
		ThreadID:       run.RootThreadID,
		CreatedAt:      time.Now().UTC(),
	}

	body := buildChildTaskBody(run, &task)
	if err := coordinatorStore.CreateTask(&task); err != nil {
		return nil, err
	}

	if err := createWorkflowMessage(cfg, store, workflowMessageInput{
		Seq:           messageSeq,
		MessageID:     task.MessageID,
		ThreadID:      task.ThreadID,
		ReplyTo:       &run.RootMessageID,
		From:          run.Coordinator,
		To:            task.Owner,
		Subject:       fmt.Sprintf("Task %s: %s", task.TaskID, summarizeSubject(task.Goal)),
		Body:          body,
		Kind:          protocol.KindTask,
		RequiresClaim: true,
		Meta: map[string]string{
			"run_id":          string(run.RunID),
			"task_id":         string(task.TaskID),
			"parent_run_id":   string(task.ParentRunID),
			"expected_output": task.ExpectedOutput,
		},
	}); err != nil {
		return nil, err
	}

	return &task, nil
}

type workflowMessageInput struct {
	Seq           int64
	MessageID     protocol.MessageID
	ThreadID      protocol.ThreadID
	ReplyTo       *protocol.MessageID
	From          protocol.AgentName
	To            protocol.AgentName
	Subject       string
	Body          string
	Kind          protocol.Kind
	RequiresClaim bool
	Meta          map[string]string
}

func createWorkflowMessage(cfg *config.ResolvedConfig, store *mailbox.Store, input workflowMessageInput) error {
	payload := []byte(input.Body)
	if !strings.HasSuffix(input.Body, "\n") {
		payload = append(payload, '\n')
	}
	sum := sha256.Sum256(payload)

	env := protocol.Envelope{
		Schema:        protocol.MessageSchemaV1,
		ID:            input.MessageID,
		Seq:           input.Seq,
		Session:       cfg.Session.Name,
		Thread:        input.ThreadID,
		Kind:          input.Kind,
		From:          input.From,
		To:            []protocol.AgentName{input.To},
		CreatedAt:     time.Now().UTC(),
		BodyFormat:    protocol.BodyFormatMD,
		BodySHA256:    fmt.Sprintf("%x", sum[:]),
		BodyBytes:     int64(len(payload)),
		ReplyTo:       input.ReplyTo,
		Subject:       input.Subject,
		Priority:      protocol.PriorityNormal,
		RequiresAck:   true,
		RequiresClaim: input.RequiresClaim,
		Meta:          input.Meta,
	}
	if err := store.CreateMessage(&env, payload); err != nil {
		return fmt.Errorf("create message: %w", err)
	}

	receipt := protocol.Receipt{
		Schema:         protocol.ReceiptSchemaV1,
		MessageID:      input.MessageID,
		Seq:            input.Seq,
		Recipient:      input.To,
		FolderState:    protocol.FolderStateUnread,
		Revision:       0,
		NotifyAttempts: 0,
	}
	if err := store.CreateReceipt(&receipt); err != nil {
		return fmt.Errorf("create receipt: %w", err)
	}

	return nil
}

func routingBaseline(cfg *config.ResolvedConfig, coordinator *config.AgentConfig) ([]protocol.AgentName, []protocol.AgentSnapshot) {
	allowedOwners := make([]protocol.AgentName, 0, len(coordinator.Teammates))
	snapshots := []protocol.AgentSnapshot{
		{
			Name:      protocol.AgentName(coordinator.Name),
			Alias:     coordinator.Alias,
			Role:      coordinator.Role,
			Teammates: slices.Clone(coordinator.Teammates),
		},
	}

	for i := range cfg.Agents {
		agent := &cfg.Agents[i]
		if !containsString(coordinator.Teammates, agent.Name) {
			continue
		}
		if strings.TrimSpace(agent.Role) == "" {
			continue
		}

		allowedOwners = append(allowedOwners, protocol.AgentName(agent.Name))
		snapshots = append(snapshots, protocol.AgentSnapshot{
			Name:      protocol.AgentName(agent.Name),
			Alias:     agent.Alias,
			Role:      agent.Role,
			Teammates: slices.Clone(agent.Teammates),
		})
	}

	return allowedOwners, snapshots
}

func resolveAgentConfig(cfg *config.ResolvedConfig, target string) (*config.AgentConfig, error) {
	for i := range cfg.Agents {
		if cfg.Agents[i].Name == target || cfg.Agents[i].Alias == target {
			return &cfg.Agents[i], nil
		}
	}

	return nil, fmt.Errorf("unknown target agent %q", target)
}

func findAgentByName(cfg *config.ResolvedConfig, name string) (*config.AgentConfig, error) {
	for i := range cfg.Agents {
		if cfg.Agents[i].Name == name {
			return &cfg.Agents[i], nil
		}
	}

	return nil, fmt.Errorf("unknown agent %q", name)
}

func containsString(values []string, want string) bool {
	return slices.Contains(values, want)
}

func containsAgentName(values []protocol.AgentName, want string) bool {
	return slices.Contains(values, protocol.AgentName(want))
}

func buildChildTaskBody(run *protocol.CoordinatorRun, task *protocol.ChildTask) string {
	var body strings.Builder
	body.WriteString("# Task\n\n")
	body.WriteString("Use mailbox commands for replies and task state updates. Do not rely on raw pane text.\n\n")
	body.WriteString("## Goal\n")
	body.WriteString(task.Goal)
	body.WriteString("\n\n")
	body.WriteString("## Expected Output\n")
	body.WriteString(task.ExpectedOutput)
	body.WriteString("\n\n")
	body.WriteString("## Dependencies\n")
	if len(task.DependsOn) == 0 {
		body.WriteString("- none\n\n")
	} else {
		for _, dep := range task.DependsOn {
			body.WriteString("- ")
			body.WriteString(string(dep))
			body.WriteByte('\n')
		}
		body.WriteByte('\n')
	}
	body.WriteString("Reply with `tmuxicate reply <message-id>` and use `tmuxicate task` subcommands for state changes instead of raw pane text.\n\n")
	body.WriteString("## Run References\n")
	body.WriteString(fmt.Sprintf("run_id: %s\n", run.RunID))
	body.WriteString(fmt.Sprintf("task_id: %s\n", task.TaskID))
	body.WriteString(fmt.Sprintf("parent_run_id: %s\n", task.ParentRunID))
	body.WriteString(fmt.Sprintf("review_required: %t\n", task.ReviewRequired))
	body.WriteString(fmt.Sprintf("root_message_id: %s\n", run.RootMessageID))
	body.WriteString(fmt.Sprintf("thread_id: %s\n", task.ThreadID))

	return body.String()
}

func summarizeSubject(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= 72 {
		return trimmed
	}

	return trimmed[:69] + "..."
}
