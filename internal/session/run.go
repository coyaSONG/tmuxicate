package session

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"sort"
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

	allowedOwners, teamSnapshot, err := routingBaseline(cfg, coordinator)
	if err != nil {
		return nil, err
	}
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
	// `## Run References`, and the `tmuxicate run route-task --run` command prefix.
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
	if !owner.Role.IsDeclared() {
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

	if childTaskRequestHasRoutingMetadata(req) {
		unlock, err := mailbox.LockRunRoute(cfg.Session.StateDir, req.ParentRunID)
		if err != nil {
			return nil, err
		}
		defer func() { _ = unlock() }()
	}

	return addChildTaskWithResolvedOwner(cfg, store, coordinatorStore, run, owner, req)
}

func addChildTaskWithResolvedOwner(cfg *config.ResolvedConfig, store *mailbox.Store, coordinatorStore *mailbox.CoordinatorStore, run *protocol.CoordinatorRun, owner *config.AgentConfig, req ChildTaskRequest) (*protocol.ChildTask, error) {
	var routingDecision *protocol.RoutingDecision
	if childTaskRequestHasRoutingMetadata(req) {
		decision := req.RoutingDecision
		decision.Status = strings.TrimSpace(decision.Status)
		decision.SelectedOwner = protocol.AgentName(owner.Name)
		if duplicate, err := findActiveDuplicateTask(cfg.Session.StateDir, req.ParentRunID, req.DuplicateKey); err != nil {
			return nil, err
		} else if duplicate != nil {
			if duplicatePolicyBlocks(cfg, req.TaskClass) {
				return nil, duplicateRouteError(req.DuplicateKey, duplicate.TaskID)
			}

			decision.DuplicateStatus = "fanout"
			decision.MatchedTaskID = duplicate.TaskID
		} else {
			decision.DuplicateStatus = "unique"
			decision.MatchedTaskID = ""
		}

		routingDecision = &decision
	}

	placement, err := selectTaskPlacement(cfg, owner)
	if err != nil {
		return nil, err
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
		TaskID:            protocol.NewTaskID(taskSeq),
		ParentRunID:       req.ParentRunID,
		Owner:             protocol.AgentName(owner.Name),
		Goal:              strings.TrimSpace(req.Goal),
		ExpectedOutput:    strings.TrimSpace(req.ExpectedOutput),
		DependsOn:         slices.Clone(req.DependsOn),
		ReviewRequired:    req.ReviewRequired,
		TaskClass:         req.TaskClass,
		Domains:           slices.Clone(req.Domains),
		NormalizedDomains: slices.Clone(req.NormalizedDomains),
		DuplicateKey:      strings.TrimSpace(req.DuplicateKey),
		RoutingDecision:   routingDecision,
		Placement:         placement,
		OverrideReason:    strings.TrimSpace(req.OverrideReason),
		MessageID:         protocol.NewMessageID(messageSeq),
		ThreadID:          run.RootThreadID,
		CreatedAt:         time.Now().UTC(),
	}

	body := buildChildTaskBody(run, &task)
	if err := coordinatorStore.CreateTask(&task); err != nil {
		return nil, err
	}

	messageKind := protocol.KindTask
	if task.TaskClass == protocol.TaskClassReview {
		messageKind = protocol.KindReviewRequest
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
		Kind:          messageKind,
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

type routeCandidate struct {
	agent *config.AgentConfig
	index int
}

func RouteChildTask(cfg *config.ResolvedConfig, store *mailbox.Store, req protocol.RouteChildTaskRequest) (*protocol.ChildTask, *protocol.RoutingDecision, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("resolved config is required")
	}
	if store == nil {
		return nil, nil, fmt.Errorf("store is required")
	}
	if err := req.Validate(); err != nil {
		return nil, nil, err
	}

	coordinatorStore := mailbox.NewCoordinatorStore(cfg.Session.StateDir)
	run, err := coordinatorStore.ReadRun(req.RunID)
	if err != nil {
		return nil, nil, err
	}

	unlock, err := mailbox.LockRunRoute(cfg.Session.StateDir, req.RunID)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = unlock() }()

	duplicateKey := duplicateKeyForRoute(req.RunID, req.TaskClass, req.Domains)
	existingDuplicate, err := findActiveDuplicateTask(cfg.Session.StateDir, req.RunID, duplicateKey)
	if err != nil {
		return nil, nil, err
	}
	if existingDuplicate != nil && duplicatePolicyBlocks(cfg, req.TaskClass) {
		return nil, nil, duplicateRouteError(duplicateKey, existingDuplicate.TaskID)
	}

	kindCandidates, domainCandidates := routeCandidates(cfg, run.AllowedOwners, req.TaskClass, req.Domains)
	rankCandidates(kindCandidates)
	rankCandidates(domainCandidates)
	if req.OwnerOverride != "" {
		return routeChildTaskOverride(cfg, store, coordinatorStore, req, run, kindCandidates, duplicateKey, existingDuplicate)
	}
	if len(domainCandidates) == 0 {
		rejection := &protocol.RouteRejection{
			TaskClass:          req.TaskClass,
			Domains:            slices.Clone(req.Domains),
			AllowedOwners:      slices.Clone(run.AllowedOwners),
			EligibleCandidates: candidateNames(kindCandidates),
			Suggestions:        routeSuggestions(req.TaskClass, kindCandidates),
		}
		if err := rejection.Validate(); err != nil {
			return nil, nil, fmt.Errorf("validate route rejection: %w", err)
		}

		return nil, nil, rejection
	}

	selected := domainCandidates[0]
	baseline := selected
	var adaptiveExplanation *protocol.AdaptiveRoutingExplanation
	if cfg.Routing.Adaptive.Enabled {
		preferenceSet, err := readAdaptiveRoutingPreferenceSet(cfg.Session.StateDir, run.Coordinator)
		if err != nil {
			return nil, nil, err
		}
		adaptiveExplanation = adaptiveRoutingExplanation(req.TaskClass, req.Domains, baseline, domainCandidates, preferenceSet)
		if adaptiveExplanation != nil && adaptiveExplanation.Applied {
			for _, candidate := range domainCandidates {
				if protocol.AgentName(candidate.agent.Name) == adaptiveExplanation.PreferredOwner {
					selected = candidate
					break
				}
			}
		}
	}
	decision := &protocol.RoutingDecision{
		Status:        "selected",
		SelectedOwner: protocol.AgentName(selected.agent.Name),
		Candidates:    candidateNames(domainCandidates),
		TieBreak:      "route_priority desc, config_order asc",
		Adaptive:      adaptiveExplanation,
	}
	if existingDuplicate != nil {
		decision.DuplicateStatus = "fanout"
		decision.MatchedTaskID = existingDuplicate.TaskID
	} else {
		decision.DuplicateStatus = "unique"
	}
	if err := decision.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validate routing decision: %w", err)
	}

	task, err := addChildTaskWithResolvedOwner(cfg, store, coordinatorStore, run, selected.agent, ChildTaskRequest{
		ParentRunID:       req.RunID,
		Owner:             selected.agent.Name,
		Goal:              req.Goal,
		ExpectedOutput:    req.ExpectedOutput,
		ReviewRequired:    req.ReviewRequired,
		TaskClass:         req.TaskClass,
		Domains:           slices.Clone(req.Domains),
		NormalizedDomains: slices.Clone(req.Domains),
		DuplicateKey:      duplicateKey,
		RoutingDecision:   *decision,
		OverrideReason:    req.OverrideReason,
	})
	if err != nil {
		return nil, nil, err
	}

	return task, decision, nil
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

func routingBaseline(cfg *config.ResolvedConfig, coordinator *config.AgentConfig) ([]protocol.AgentName, []protocol.AgentSnapshot, error) {
	coordinatorTarget, err := resolveExecutionTarget(cfg, coordinator)
	if err != nil {
		return nil, nil, err
	}

	allowedOwners := make([]protocol.AgentName, 0, len(coordinator.Teammates))
	snapshots := []protocol.AgentSnapshot{
		{
			Name:            protocol.AgentName(coordinator.Name),
			Alias:           coordinator.Alias,
			Role:            coordinator.Role.Kind,
			Teammates:       slices.Clone(coordinator.Teammates),
			ExecutionTarget: coordinatorTarget,
		},
	}

	for i := range cfg.Agents {
		agent := &cfg.Agents[i]
		if !containsString(coordinator.Teammates, agent.Name) {
			continue
		}
		if !agent.Role.IsDeclared() {
			continue
		}
		target, err := resolveExecutionTarget(cfg, agent)
		if err != nil {
			return nil, nil, err
		}

		allowedOwners = append(allowedOwners, protocol.AgentName(agent.Name))
		snapshots = append(snapshots, protocol.AgentSnapshot{
			Name:            protocol.AgentName(agent.Name),
			Alias:           agent.Alias,
			Role:            agent.Role.Kind,
			Teammates:       slices.Clone(agent.Teammates),
			ExecutionTarget: target,
		})
	}

	return allowedOwners, snapshots, nil
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

func selectTaskPlacement(cfg *config.ResolvedConfig, owner *config.AgentConfig) (*protocol.TaskPlacement, error) {
	target, err := resolveExecutionTarget(cfg, owner)
	if err != nil {
		return nil, err
	}

	reason := fmt.Sprintf("owner %q uses implicit local pane-backed target", owner.Name)
	if strings.TrimSpace(owner.ExecutionTarget) != "" {
		reason = fmt.Sprintf("owner %q is bound to execution target %q", owner.Name, target.Name)
	}

	return &protocol.TaskPlacement{
		Target: target,
		Reason: reason,
	}, nil
}

func resolveExecutionTarget(cfg *config.ResolvedConfig, agent *config.AgentConfig) (protocol.ExecutionTarget, error) {
	if cfg == nil {
		return protocol.ExecutionTarget{}, fmt.Errorf("resolved config is required")
	}
	if agent == nil {
		return protocol.ExecutionTarget{}, fmt.Errorf("agent config is required")
	}

	targetName := strings.TrimSpace(agent.ExecutionTarget)
	if targetName == "" {
		return protocol.ExecutionTarget{
			Name:         "local",
			Kind:         "local",
			Description:  "Implicit local pane-backed execution target",
			Capabilities: []string{"local", "pane"},
			PaneBacked:   true,
		}, nil
	}

	for _, target := range cfg.ExecutionTargets {
		if target.Name != targetName {
			continue
		}

		return protocol.ExecutionTarget{
			Name:         target.Name,
			Kind:         target.Kind,
			Description:  target.Description,
			Capabilities: slices.Clone(target.Capabilities),
			PaneBacked:   target.PaneBacked,
		}, nil
	}

	return protocol.ExecutionTarget{}, fmt.Errorf("unknown execution target %q for agent %q", targetName, agent.Name)
}

func routeCandidates(cfg *config.ResolvedConfig, allowedOwners []protocol.AgentName, taskClass protocol.TaskClass, domains []string) ([]routeCandidate, []routeCandidate) {
	kindCandidates := make([]routeCandidate, 0, len(allowedOwners))
	domainCandidates := make([]routeCandidate, 0, len(allowedOwners))

	for i := range cfg.Agents {
		agent := &cfg.Agents[i]
		if !containsAgentName(allowedOwners, agent.Name) {
			continue
		}
		if !agent.Role.IsDeclared() {
			continue
		}
		if agent.Role.Kind != string(taskClass) {
			continue
		}

		candidate := routeCandidate{agent: agent, index: i}
		kindCandidates = append(kindCandidates, candidate)
		if roleCoversDomains(agent.Role, domains) {
			domainCandidates = append(domainCandidates, candidate)
		}
	}

	return kindCandidates, domainCandidates
}

func routeChildTaskOverride(cfg *config.ResolvedConfig, store *mailbox.Store, coordinatorStore *mailbox.CoordinatorStore, req protocol.RouteChildTaskRequest, run *protocol.CoordinatorRun, kindCandidates []routeCandidate, duplicateKey string, existingDuplicate *protocol.ChildTask) (*protocol.ChildTask, *protocol.RoutingDecision, error) {
	owner, err := resolveAgentConfig(cfg, string(req.OwnerOverride))
	if err != nil {
		return nil, nil, err
	}
	if !containsAgentName(run.AllowedOwners, owner.Name) {
		return nil, nil, fmt.Errorf("owner override %q is not an allowed owner for run %q", owner.Name, run.RunID)
	}

	candidates := candidateNames(kindCandidates)
	if len(candidates) == 0 {
		candidates = []protocol.AgentName{protocol.AgentName(owner.Name)}
	}
	decision := &protocol.RoutingDecision{
		Status:        "selected",
		SelectedOwner: protocol.AgentName(owner.Name),
		Candidates:    candidates,
		TieBreak:      "owner_override",
	}
	if existingDuplicate != nil {
		decision.DuplicateStatus = "fanout"
		decision.MatchedTaskID = existingDuplicate.TaskID
	} else {
		decision.DuplicateStatus = "unique"
	}
	if err := decision.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validate routing decision: %w", err)
	}

	task, err := addChildTaskWithResolvedOwner(cfg, store, coordinatorStore, run, owner, ChildTaskRequest{
		ParentRunID:       req.RunID,
		Owner:             owner.Name,
		Goal:              req.Goal,
		ExpectedOutput:    req.ExpectedOutput,
		ReviewRequired:    req.ReviewRequired,
		TaskClass:         req.TaskClass,
		Domains:           slices.Clone(req.Domains),
		NormalizedDomains: slices.Clone(req.Domains),
		DuplicateKey:      duplicateKey,
		RoutingDecision:   *decision,
		OverrideReason:    req.OverrideReason,
	})
	if err != nil {
		return nil, nil, err
	}

	return task, decision, nil
}

func candidateNames(candidates []routeCandidate) []protocol.AgentName {
	names := make([]protocol.AgentName, 0, len(candidates))
	for _, candidate := range candidates {
		names = append(names, protocol.AgentName(candidate.agent.Name))
	}

	return names
}

func rankCandidates(candidates []routeCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.agent.RoutePriority != right.agent.RoutePriority {
			return left.agent.RoutePriority > right.agent.RoutePriority
		}

		return left.index < right.index
	})
}

func adaptiveRoutingExplanation(taskClass protocol.TaskClass, normalizedDomains []string, baseline routeCandidate, candidates []routeCandidate, preferenceSet *protocol.AdaptiveRoutingPreferenceSet) *protocol.AdaptiveRoutingExplanation {
	if preferenceSet == nil {
		return nil
	}

	var (
		bestCandidate  routeCandidate
		haveBest       bool
		bestPreference *protocol.AdaptiveRoutingPreference
		scoreTie       bool
		matchedCount   int
	)
	for index := range candidates {
		candidate := candidates[index]
		preference := matchAdaptivePreference(preferenceSet, taskClass, normalizedDomains, protocol.AgentName(candidate.agent.Name))
		if preference == nil {
			continue
		}
		matchedCount++
		if !haveBest {
			bestCandidate = candidate
			haveBest = true
			bestPreference = preference
			continue
		}
		switch {
		case preference.TotalScore > bestPreference.TotalScore:
			bestCandidate = candidate
			bestPreference = preference
			scoreTie = false
		case preference.TotalScore == bestPreference.TotalScore:
			scoreTie = true
		}
	}
	if matchedCount == 0 || bestPreference == nil || !haveBest || scoreTie {
		return nil
	}
	if bestCandidate.agent.Name == baseline.agent.Name {
		return nil
	}

	return &protocol.AdaptiveRoutingExplanation{
		Applied:         true,
		BaselineOwner:   protocol.AgentName(baseline.agent.Name),
		PreferredOwner:  protocol.AgentName(bestCandidate.agent.Name),
		HistoricalScore: bestPreference.HistoricalScore,
		ManualWeight:    bestPreference.ManualWeight,
		TotalScore:      bestPreference.TotalScore,
		Reason: fmt.Sprintf(
			"exact preference match on %s|%s favored %s over baseline %s",
			taskClass,
			strings.Join(normalizedDomains, ","),
			bestCandidate.agent.Name,
			baseline.agent.Name,
		),
		Evidence: slices.Clone(bestPreference.Evidence),
	}
}

func roleCoversDomains(role config.RoleSpec, domains []string) bool {
	roleDomains, err := protocol.NormalizeRouteDomains(role.Domains)
	if err != nil {
		return false
	}

	available := make(map[string]struct{}, len(roleDomains))
	for _, domain := range roleDomains {
		available[domain] = struct{}{}
	}
	for _, domain := range domains {
		if _, ok := available[domain]; !ok {
			return false
		}
	}

	return true
}

func routeSuggestions(taskClass protocol.TaskClass, candidates []routeCandidate) []string {
	if len(candidates) == 0 {
		return []string{
			fmt.Sprintf("Choose a different task_class or add an allowed owner with role.kind %q.", taskClass),
		}
	}

	return []string{
		"Choose domains that are covered by one of the eligible_candidates.",
		"Add the missing domain to an allowed owner's role.domains and retry.",
	}
}

func duplicatePolicyBlocks(cfg *config.ResolvedConfig, taskClass protocol.TaskClass) bool {
	if duplicatePolicyAllowsFanout(cfg, taskClass) {
		return false
	}
	// routing.exclusive_task_classes stays as an explicit duplicate-block allowlist in config.
	if slices.Contains(cfg.Routing.ExclusiveTaskClasses, taskClass) {
		return true
	}

	return true
}

func duplicatePolicyAllowsFanout(cfg *config.ResolvedConfig, taskClass protocol.TaskClass) bool {
	// routing.fanout_task_classes is the only config path that permits duplicate fanout.
	return slices.Contains(cfg.Routing.FanoutTaskClasses, taskClass)
}

func duplicateKeyForRoute(runID protocol.RunID, taskClass protocol.TaskClass, normalizedDomains []string) string {
	return fmt.Sprintf("%s|%s|%s", runID, taskClass, strings.Join(normalizedDomains, ","))
}

func findActiveDuplicateTask(stateDir string, runID protocol.RunID, duplicateKey string) (*protocol.ChildTask, error) {
	if strings.TrimSpace(duplicateKey) == "" {
		return nil, nil
	}

	tasks, err := loadRunTasks(stateDir, runID)
	if err != nil {
		return nil, err
	}

	for _, task := range tasks {
		if task.DuplicateKey != duplicateKey {
			continue
		}

		receiptState, err := loadTaskReceiptState(stateDir, string(task.Owner), task.MessageID)
		if err != nil {
			return nil, err
		}
		if receiptState == protocol.FolderStateDone || receiptState == protocol.FolderStateDead {
			continue
		}

		matched := task
		return &matched, nil
	}

	return nil, nil
}

func duplicateRouteError(duplicateKey string, taskID protocol.TaskID) error {
	return fmt.Errorf("duplicate_key %q matched_task_id %s", duplicateKey, taskID)
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
