package session

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

type RunRequest struct {
	Goal          string                   `yaml:"goal"`
	Coordinator   string                   `yaml:"coordinator"`
	CreatedBy     string                   `yaml:"created_by"`
	AllowedOwners []protocol.AgentName     `yaml:"allowed_owners,omitempty"`
	TeamSnapshot  []protocol.AgentSnapshot `yaml:"team_snapshot,omitempty"`
}

type ChildTaskRequest struct {
	ParentRunID       protocol.RunID           `yaml:"parent_run_id"`
	Owner             string                   `yaml:"owner"`
	Goal              string                   `yaml:"goal"`
	ExpectedOutput    string                   `yaml:"expected_output"`
	DependsOn         []protocol.TaskID        `yaml:"depends_on,omitempty"`
	ReviewRequired    bool                     `yaml:"review_required"`
	TaskClass         protocol.TaskClass       `yaml:"task_class,omitempty"`
	Domains           []string                 `yaml:"domains,omitempty"`
	NormalizedDomains []string                 `yaml:"normalized_domains,omitempty"`
	DuplicateKey      string                   `yaml:"duplicate_key,omitempty"`
	RoutingDecision   protocol.RoutingDecision `yaml:"routing_decision,omitempty"`
	OverrideReason    string                   `yaml:"override_reason,omitempty"`
	MessageID         protocol.MessageID       `yaml:"message_id,omitempty"`
	ThreadID          protocol.ThreadID        `yaml:"thread_id,omitempty"`
}

type RunRootMessageInput struct {
	Run protocol.CoordinatorRun
}

func (r RunRequest) Validate(cfg *config.ResolvedConfig) error {
	if strings.TrimSpace(r.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	if strings.TrimSpace(r.Coordinator) == "" {
		return fmt.Errorf("coordinator is required")
	}
	if strings.TrimSpace(r.CreatedBy) == "" {
		return fmt.Errorf("created_by is required")
	}
	if cfg == nil {
		return fmt.Errorf("resolved config is required")
	}
	if !matchesAgentNameOrAlias(cfg, r.Coordinator) {
		return fmt.Errorf("coordinator %q does not match any agent name or alias", r.Coordinator)
	}

	return nil
}

func (r ChildTaskRequest) Validate() error {
	task := protocol.ChildTask{
		TaskID:            protocol.NewTaskID(1),
		ParentRunID:       r.ParentRunID,
		Owner:             protocol.AgentName(r.Owner),
		Goal:              r.Goal,
		ExpectedOutput:    r.ExpectedOutput,
		DependsOn:         slices.Clone(r.DependsOn),
		ReviewRequired:    r.ReviewRequired,
		TaskClass:         r.TaskClass,
		Domains:           slices.Clone(r.Domains),
		NormalizedDomains: slices.Clone(r.NormalizedDomains),
		DuplicateKey:      r.DuplicateKey,
		OverrideReason:    r.OverrideReason,
		MessageID:         r.MessageID,
		ThreadID:          r.ThreadID,
		CreatedAt:         time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}
	if childTaskRequestHasRoutingMetadata(r) {
		decision := r.RoutingDecision
		task.RoutingDecision = &decision
	}

	return task.Validate()
}

func BuildRunRootMessageBody(input RunRootMessageInput) (string, error) {
	if err := input.Run.Validate(); err != nil {
		return "", fmt.Errorf("validate run root message input: %w", err)
	}

	var body strings.Builder
	body.WriteString("# Coordinator Run\n\n")
	body.WriteString(fmt.Sprintf("Goal: %s\n", input.Run.Goal))
	body.WriteString(fmt.Sprintf("Coordinator: %s\n\n", input.Run.Coordinator))
	body.WriteString("## Decomposition Instructions\n")
	body.WriteString("Decompose this run into bounded child tasks for allowed owners only.\n")
	body.WriteString("Request a routed owner through the canonical CLI entrypoint instead of guessing owners in pane text:\n")
	body.WriteString(fmt.Sprintf("tmuxicate run route-task --run %s --task-class <class> --domain <domain> --goal \"<goal>\" --expected-output \"<deliverable>\"\n\n", input.Run.RunID))
	body.WriteString("Use `tmuxicate run add-task` only after routing has chosen the owner or when persisting an explicit owner override.\n")
	body.WriteString("Each task must include owner, goal, expected output, dependency IDs, and whether review is required.\n\n")
	body.WriteString("## Run References\n")
	body.WriteString(fmt.Sprintf("run_id: %s\n", input.Run.RunID))
	body.WriteString(fmt.Sprintf("root_message_id: %s\n", input.Run.RootMessageID))
	body.WriteString(fmt.Sprintf("root_thread_id: %s\n", input.Run.RootThreadID))

	return body.String(), nil
}

func matchesAgentNameOrAlias(cfg *config.ResolvedConfig, target string) bool {
	for _, agent := range cfg.Agents {
		if agent.Name == target || agent.Alias == target {
			return true
		}
	}
	return false
}

func childTaskRequestHasRoutingMetadata(req ChildTaskRequest) bool {
	return req.TaskClass != "" ||
		len(req.Domains) > 0 ||
		len(req.NormalizedDomains) > 0 ||
		strings.TrimSpace(req.DuplicateKey) != "" ||
		strings.TrimSpace(req.RoutingDecision.Status) != "" ||
		strings.TrimSpace(string(req.RoutingDecision.SelectedOwner)) != "" ||
		len(req.RoutingDecision.Candidates) > 0 ||
		strings.TrimSpace(req.RoutingDecision.TieBreak) != "" ||
		strings.TrimSpace(req.RoutingDecision.DuplicateStatus) != "" ||
		req.RoutingDecision.MatchedTaskID != "" ||
		len(req.RoutingDecision.Suggestions) > 0 ||
		strings.TrimSpace(req.OverrideReason) != ""
}
