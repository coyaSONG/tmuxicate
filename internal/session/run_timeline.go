package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

const (
	timelineImplicitLocalExecutionTarget = "local"
)

type RunTimeline struct {
	RunID  protocol.RunID
	Events []RunTimelineEvent
}

type RunTimelineEvent struct {
	Timestamp       time.Time
	Kind            string
	Owner           protocol.AgentName
	State           string
	TaskClass       protocol.TaskClass
	ExecutionTarget string
	TaskID          protocol.TaskID
	MessageID       protocol.MessageID
	Summary         string
}

type RunTimelineFilter struct {
	Owner           string
	State           string
	TaskClass       protocol.TaskClass
	ExecutionTarget string
}

type timelineTaskInfo struct {
	task   RunGraphTask
	target string
}

type timelineEventSortKey struct {
	precedence int
	taskID     protocol.TaskID
	messageID  protocol.MessageID
	owner      protocol.AgentName
	summary    string
}

func BuildRunTimeline(stateDir string, graph *RunGraph) (*RunTimeline, error) {
	if graph == nil {
		return nil, fmt.Errorf("run graph is required")
	}

	lookup := make(map[protocol.MessageID]timelineTaskInfo, len(graph.Tasks))
	tasksByID := make(map[protocol.TaskID]timelineTaskInfo, len(graph.Tasks))
	for _, task := range graph.Tasks {
		info := timelineTaskInfo{
			task:   task,
			target: taskExecutionTarget(task.Task),
		}
		lookup[task.Task.MessageID] = info
		tasksByID[task.Task.TaskID] = info
	}

	events := make([]RunTimelineEvent, 0, 1+len(graph.Tasks)*4)
	events = append(events, RunTimelineEvent{
		Timestamp:       graph.Run.CreatedAt,
		Kind:            "run.created",
		Owner:           graph.Run.Coordinator,
		ExecutionTarget: timelineImplicitLocalExecutionTarget,
		MessageID:       graph.Run.RootMessageID,
		Summary:         fmt.Sprintf("run %s created", graph.Run.RunID),
	})

	for _, task := range graph.Tasks {
		info := tasksByID[task.Task.TaskID]
		taskState := taskEventState(task.DeclaredState)
		events = append(events, RunTimelineEvent{
			Timestamp:       task.Task.CreatedAt,
			Kind:            "task.created",
			Owner:           task.Task.Owner,
			State:           taskState,
			TaskClass:       task.Task.TaskClass,
			ExecutionTarget: info.target,
			TaskID:          task.Task.TaskID,
			MessageID:       task.Task.MessageID,
			Summary:         fmt.Sprintf("task created for %s", task.Task.Owner),
		})
		if task.Task.RoutingDecision != nil {
			events = append(events, RunTimelineEvent{
				Timestamp:       task.Task.CreatedAt,
				Kind:            "task.routed",
				Owner:           task.Task.Owner,
				State:           taskState,
				TaskClass:       task.Task.TaskClass,
				ExecutionTarget: info.target,
				TaskID:          task.Task.TaskID,
				MessageID:       task.Task.MessageID,
				Summary:         fmt.Sprintf("routed to %s", task.Task.Owner),
			})
		}
		if task.ReviewHandoff != nil {
			reviewInfo, ok := tasksByID[task.ReviewHandoff.ReviewTaskID]
			if task.ReviewHandoff.ReviewTaskID != "" && !ok {
				return nil, coordinatorArtifactMismatch("review handoff references missing review task %s", task.ReviewHandoff.ReviewTaskID)
			}
			reviewOwner := task.ReviewHandoff.Reviewer
			reviewTarget := timelineImplicitLocalExecutionTarget
			reviewTaskClass := protocol.TaskClassReview
			reviewMessageID := task.ReviewHandoff.ReviewMessageID
			if ok {
				reviewOwner = reviewInfo.task.Task.Owner
				reviewTarget = reviewInfo.target
				if reviewInfo.task.Task.TaskClass != "" {
					reviewTaskClass = reviewInfo.task.Task.TaskClass
				}
				reviewMessageID = reviewInfo.task.Task.MessageID
			}
			events = append(events, RunTimelineEvent{
				Timestamp:       task.ReviewHandoff.CreatedAt,
				Kind:            "review.handoff",
				Owner:           reviewOwner,
				State:           reviewState(task.ReviewHandoff.Status),
				TaskClass:       reviewTaskClass,
				ExecutionTarget: reviewTarget,
				TaskID:          task.Task.TaskID,
				MessageID:       reviewMessageID,
				Summary:         fmt.Sprintf("review handoff %s", task.ReviewHandoff.Status),
			})
		}
		if task.BlockerCase != nil && task.BlockerCase.EscalatedAt != nil {
			events = append(events, RunTimelineEvent{
				Timestamp:       *task.BlockerCase.EscalatedAt,
				Kind:            "blocker.escalated",
				Owner:           task.Task.Owner,
				State:           taskEventState(task.BlockerCase.DeclaredState),
				TaskClass:       task.Task.TaskClass,
				ExecutionTarget: info.target,
				TaskID:          task.Task.TaskID,
				MessageID:       task.Task.MessageID,
				Summary:         task.BlockerCase.Reason,
			})
		}
		if task.BlockerCase != nil && task.BlockerCase.ResolvedAt != nil {
			events = append(events, RunTimelineEvent{
				Timestamp:       *task.BlockerCase.ResolvedAt,
				Kind:            "blocker.resolved",
				Owner:           task.Task.Owner,
				State:           "resolved",
				TaskClass:       task.Task.TaskClass,
				ExecutionTarget: info.target,
				TaskID:          task.Task.TaskID,
				MessageID:       task.Task.MessageID,
				Summary:         formatBlockerResolution(task.BlockerCase.Resolution),
			})
		}
		if task.PartialReplan != nil {
			events = append(events, RunTimelineEvent{
				Timestamp:       task.PartialReplan.CreatedAt,
				Kind:            "partial_replan.applied",
				Owner:           task.Task.Owner,
				State:           taskState,
				TaskClass:       task.Task.TaskClass,
				ExecutionTarget: info.target,
				TaskID:          task.Task.TaskID,
				MessageID:       task.Task.MessageID,
				Summary:         task.PartialReplan.Reason,
			})
		}
	}

	stateEvents, err := loadRunTimelineStateEvents(stateDir, lookup)
	if err != nil {
		return nil, err
	}
	events = append(events, stateEvents...)

	sort.Slice(events, func(i, j int) bool {
		if !events[i].Timestamp.Equal(events[j].Timestamp) {
			return events[i].Timestamp.Before(events[j].Timestamp)
		}

		left := timelineSortKey(events[i])
		right := timelineSortKey(events[j])
		if left.precedence != right.precedence {
			return left.precedence < right.precedence
		}
		if left.taskID != right.taskID {
			return left.taskID < right.taskID
		}
		if left.messageID != right.messageID {
			return left.messageID < right.messageID
		}
		if left.owner != right.owner {
			return left.owner < right.owner
		}
		if events[i].Kind != events[j].Kind {
			return events[i].Kind < events[j].Kind
		}
		return left.summary < right.summary
	})

	return &RunTimeline{
		RunID:  graph.Run.RunID,
		Events: events,
	}, nil
}

func FilterRunTimeline(timeline *RunTimeline, filter RunTimelineFilter) []RunTimelineEvent {
	if timeline == nil {
		return nil
	}

	owner := strings.TrimSpace(filter.Owner)
	state := strings.TrimSpace(filter.State)
	target := strings.TrimSpace(filter.ExecutionTarget)
	filtered := make([]RunTimelineEvent, 0, len(timeline.Events))
	for _, event := range timeline.Events {
		if owner != "" && string(event.Owner) != owner {
			continue
		}
		if state != "" && event.State != state {
			continue
		}
		if filter.TaskClass != "" && event.TaskClass != filter.TaskClass {
			continue
		}
		if target != "" && event.ExecutionTarget != target {
			continue
		}
		filtered = append(filtered, event)
	}

	return filtered
}

func loadRunTimelineStateEvents(stateDir string, lookup map[protocol.MessageID]timelineTaskInfo) ([]RunTimelineEvent, error) {
	agentRoot := filepath.Join(stateDir, "agents")
	entries, err := os.ReadDir(agentRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read agents dir: %w", err)
	}

	events := make([]RunTimelineEvent, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		agent := entry.Name()
		logPath := filepath.Join(agentRoot, agent, "events", "state.jsonl")
		fileEvents, err := loadAgentTimelineStateEvents(logPath, agent, lookup)
		if err != nil {
			return nil, err
		}
		events = append(events, fileEvents...)
	}

	return events, nil
}

func loadAgentTimelineStateEvents(logPath, agent string, lookup map[protocol.MessageID]timelineTaskInfo) ([]RunTimelineEvent, error) {
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open state log: %w", err)
	}
	defer file.Close()

	events := make([]RunTimelineEvent, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var taskEvent TaskEvent
		if err := json.Unmarshal([]byte(line), &taskEvent); err != nil {
			return nil, fmt.Errorf("decode state event: %w", err)
		}
		if strings.TrimSpace(taskEvent.Event) == "" {
			continue
		}

		info, ok := lookup[taskEvent.MessageID]
		if !ok {
			continue
		}
		if strings.TrimSpace(taskEvent.Agent) != agent {
			return nil, coordinatorArtifactMismatch("state event agent mismatch for message %s", taskEvent.MessageID)
		}
		if protocol.AgentName(taskEvent.Agent) != info.task.Task.Owner {
			return nil, coordinatorArtifactMismatch("state event owner mismatch for message %s", taskEvent.MessageID)
		}
		if taskEvent.MessageID != info.task.Task.MessageID {
			return nil, coordinatorArtifactMismatch("state event message mismatch for task %s", info.task.Task.TaskID)
		}
		if taskEvent.Thread != info.task.Task.ThreadID {
			return nil, coordinatorArtifactMismatch("state event thread mismatch for task %s", info.task.Task.TaskID)
		}

		timestamp, err := time.Parse(time.RFC3339Nano, taskEvent.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse state event timestamp: %w", err)
		}

		switch taskEvent.Event {
		case "task.accept", "task.wait", "task.block", "task.done", "review.respond":
			events = append(events, RunTimelineEvent{
				Timestamp:       timestamp,
				Kind:            taskEvent.Event,
				Owner:           info.task.Task.Owner,
				State:           taskEventState(taskEvent.DeclaredState),
				TaskClass:       taskClassForStateEvent(taskEvent.Event, info.task.taskClassOrDefault()),
				ExecutionTarget: info.target,
				TaskID:          info.task.Task.TaskID,
				MessageID:       taskEvent.MessageID,
				Summary:         timelineStateEventSummary(taskEvent),
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan state log: %w", err)
	}

	return events, nil
}

func taskClassForStateEvent(kind string, defaultClass protocol.TaskClass) protocol.TaskClass {
	if kind == "review.respond" {
		return protocol.TaskClassReview
	}
	return defaultClass
}

func timelineStateEventSummary(event TaskEvent) string {
	switch event.Event {
	case "task.wait":
		return normalizeDisplayValue(event.Reason)
	case "task.block":
		return normalizeDisplayValue(event.Reason)
	case "task.done", "review.respond":
		return normalizeDisplayValue(event.Summary)
	default:
		return normalizeDisplayValue(event.DeclaredState)
	}
}

func taskExecutionTarget(task protocol.ChildTask) string {
	if task.Placement == nil {
		return timelineImplicitLocalExecutionTarget
	}
	if strings.TrimSpace(task.Placement.Target.Name) == "" {
		return timelineImplicitLocalExecutionTarget
	}
	return task.Placement.Target.Name
}

func reviewState(status protocol.ReviewHandoffStatus) string {
	switch status {
	case protocol.ReviewHandoffStatusPending:
		return "pending"
	case protocol.ReviewHandoffStatusResponded:
		return "responded"
	case protocol.ReviewHandoffStatusHandoffFailed:
		return "handoff_failed"
	default:
		return "-"
	}
}

func taskEventState(state string) string {
	value := strings.TrimSpace(state)
	if value == "" {
		return "-"
	}
	return value
}

func timelineSortKey(event RunTimelineEvent) timelineEventSortKey {
	return timelineEventSortKey{
		precedence: timelineEventPrecedence(event.Kind),
		taskID:     event.TaskID,
		messageID:  event.MessageID,
		owner:      event.Owner,
		summary:    event.Summary,
	}
}

func timelineEventPrecedence(kind string) int {
	switch kind {
	case "run.created":
		return 0
	case "task.created":
		return 1
	case "task.routed":
		return 2
	case "review.handoff":
		return 3
	case "blocker.escalated":
		return 4
	case "task.accept":
		return 5
	case "task.wait":
		return 6
	case "task.block":
		return 7
	case "task.done":
		return 8
	case "review.respond":
		return 9
	case "blocker.resolved":
		return 10
	case "partial_replan.applied":
		return 11
	default:
		return 100
	}
}

func (task RunGraphTask) taskClassOrDefault() protocol.TaskClass {
	if task.Task.TaskClass != "" {
		return task.Task.TaskClass
	}
	return protocol.TaskClassImplementation
}
