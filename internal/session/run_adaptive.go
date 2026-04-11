package session

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func BuildAdaptiveRoutingPreferences(cfg *config.ResolvedConfig, stateDir string, coordinator protocol.AgentName) (*protocol.AdaptiveRoutingPreferenceSet, error) {
	if cfg == nil {
		return nil, fmt.Errorf("resolved config is required")
	}
	if strings.TrimSpace(stateDir) == "" {
		return nil, fmt.Errorf("stateDir is required")
	}
	if strings.TrimSpace(string(coordinator)) == "" {
		return nil, fmt.Errorf("coordinator is required")
	}

	preferenceSet := &protocol.AdaptiveRoutingPreferenceSet{
		Coordinator:  coordinator,
		UpdatedAt:    time.Now().UTC(),
		LookbackRuns: cfg.Routing.Adaptive.LookbackRuns,
		Preferences:  nil,
	}
	if !cfg.Routing.Adaptive.Enabled {
		return preferenceSet, nil
	}

	completedRuns, err := loadCompletedAdaptiveRuns(stateDir, coordinator)
	if err != nil {
		return nil, err
	}
	if lookback := cfg.Routing.Adaptive.LookbackRuns; lookback > 0 && len(completedRuns) > lookback {
		completedRuns = completedRuns[:lookback]
	}

	manualPreferences := make(map[string]config.AdaptiveManualPreference, len(cfg.Routing.Adaptive.ManualPreferences))
	for _, manualPreference := range cfg.Routing.Adaptive.ManualPreferences {
		key := adaptivePreferenceKey(manualPreference.TaskClass, manualPreference.Domains, manualPreference.PreferredOwner)
		manualPreferences[key] = manualPreference
	}

	rows := make(map[string]*protocol.AdaptiveRoutingPreference)
	for _, run := range completedRuns {
		graph, err := LoadRunGraph(stateDir, run.RunID)
		if err != nil {
			return nil, fmt.Errorf("load run graph %s: %w", run.RunID, err)
		}

		tasksByID := make(map[protocol.TaskID]RunGraphTask, len(graph.Tasks))
		for _, task := range graph.Tasks {
			tasksByID[task.Task.TaskID] = task
		}

		summary := BuildRunSummary(graph)
		if summary == nil {
			continue
		}
		for _, item := range summary.Items {
			sourceTask, ok := tasksByID[item.SourceTaskID]
			if !ok {
				return nil, fmt.Errorf("run %s summary references missing source task %s", run.RunID, item.SourceTaskID)
			}
			if !hasAdaptiveRoutingSourceTaskMetadata(sourceTask.Task) {
				continue
			}

			owner := item.CurrentOwner
			if owner == "" {
				owner = sourceTask.Task.Owner
			}
			key := adaptivePreferenceKey(sourceTask.Task.TaskClass, sourceTask.Task.NormalizedDomains, owner)
			row := rows[key]
			if row == nil {
				row = &protocol.AdaptiveRoutingPreference{
					PreferenceKey:     key,
					TaskClass:         sourceTask.Task.TaskClass,
					NormalizedDomains: append([]string(nil), sourceTask.Task.NormalizedDomains...),
					PreferredOwner:    owner,
				}
				rows[key] = row
			}

			score, evidence := adaptiveHistoricalSignals(cfg, run.RunID, item, sourceTask.Task.MessageID)
			row.HistoricalScore += score
			row.Evidence = append(row.Evidence, evidence...)
		}
	}

	keys := make([]string, 0, len(rows)+len(manualPreferences))
	for key := range rows {
		keys = append(keys, key)
	}
	for key, manualPreference := range manualPreferences {
		row := rows[key]
		if row == nil {
			row = &protocol.AdaptiveRoutingPreference{
				PreferenceKey:     key,
				TaskClass:         manualPreference.TaskClass,
				NormalizedDomains: append([]string(nil), manualPreference.Domains...),
				PreferredOwner:    manualPreference.PreferredOwner,
			}
			rows[key] = row
			keys = append(keys, key)
		}
		row.ManualWeight += manualPreference.Weight
	}

	slices.Sort(keys)
	keys = slices.Compact(keys)

	preferenceSet.Preferences = make([]protocol.AdaptiveRoutingPreference, 0, len(keys))
	for _, key := range keys {
		row := rows[key]
		sort.SliceStable(row.Evidence, func(i, j int) bool {
			return compareAdaptiveEvidence(row.Evidence[i], row.Evidence[j]) < 0
		})
		row.TotalScore = row.HistoricalScore + row.ManualWeight
		preferenceSet.Preferences = append(preferenceSet.Preferences, *row)
	}

	if err := preferenceSet.Validate(); err != nil {
		return nil, fmt.Errorf("validate adaptive routing preference set: %w", err)
	}

	return preferenceSet, nil
}

type adaptiveCompletedRun struct {
	RunID     protocol.RunID
	CreatedAt time.Time
}

func loadCompletedAdaptiveRuns(stateDir string, coordinator protocol.AgentName) ([]adaptiveCompletedRun, error) {
	entries, err := os.ReadDir(mailbox.RunsDir(stateDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read coordinator runs dir: %w", err)
	}

	coordinatorStore := mailbox.NewCoordinatorStore(stateDir)
	messageStore := mailbox.NewStore(stateDir)
	runs := make([]adaptiveCompletedRun, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		run, err := coordinatorStore.ReadRun(protocol.RunID(entry.Name()))
		if err != nil {
			return nil, err
		}
		if run.Coordinator != coordinator {
			continue
		}

		receipt, err := messageStore.ReadReceipt(string(run.Coordinator), run.RootMessageID)
		if err != nil {
			continue
		}
		if receipt.FolderState != protocol.FolderStateDone {
			continue
		}

		runs = append(runs, adaptiveCompletedRun{
			RunID:     run.RunID,
			CreatedAt: run.CreatedAt,
		})
	}

	sort.SliceStable(runs, func(i, j int) bool {
		if runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].RunID > runs[j].RunID
		}
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})

	return runs, nil
}

func adaptiveHistoricalSignals(cfg *config.ResolvedConfig, runID protocol.RunID, item RunSummaryItem, sourceMessageID protocol.MessageID) (int, []protocol.AdaptiveRoutingEvidenceRef) {
	evidence := make([]protocol.AdaptiveRoutingEvidenceRef, 0, 2)
	score := 0

	if item.Status == RunSummaryStatusCompleted {
		score += cfg.Routing.Adaptive.SuccessWeight
		evidence = append(evidence, protocol.AdaptiveRoutingEvidenceRef{
			RunID:        runID,
			SourceTaskID: item.SourceTaskID,
			MessageID:    sourceMessageID,
			Status:       "completed",
			Note:         "completed source task without blocker or review downgrade",
		})
	}
	if item.ReviewOutcome == protocol.ReviewOutcomeApproved {
		score += cfg.Routing.Adaptive.ApprovalWeight
		evidence = append(evidence, protocol.AdaptiveRoutingEvidenceRef{
			RunID:        runID,
			SourceTaskID: item.SourceTaskID,
			MessageID:    sourceMessageID,
			Status:       "approved",
			Note:         "approved review outcome for the source task",
		})
	}
	if item.ReviewOutcome == protocol.ReviewOutcomeChangesRequested {
		score -= cfg.Routing.Adaptive.ChangesRequestedPenalty
		evidence = append(evidence, protocol.AdaptiveRoutingEvidenceRef{
			RunID:        runID,
			SourceTaskID: item.SourceTaskID,
			MessageID:    sourceMessageID,
			Status:       "changes_requested",
			Note:         "changes_requested review outcome for the source task",
		})
	}
	switch item.Status {
	case RunSummaryStatusEscalated:
		score -= cfg.Routing.Adaptive.BlockedPenalty
		evidence = append(evidence, protocol.AdaptiveRoutingEvidenceRef{
			RunID:        runID,
			SourceTaskID: item.SourceTaskID,
			MessageID:    sourceMessageID,
			Status:       "escalated",
			Note:         "source task escalated to a human operator",
		})
	case RunSummaryStatusBlocked:
		score -= cfg.Routing.Adaptive.BlockedPenalty
		evidence = append(evidence, protocol.AdaptiveRoutingEvidenceRef{
			RunID:        runID,
			SourceTaskID: item.SourceTaskID,
			MessageID:    sourceMessageID,
			Status:       "blocked",
			Note:         "source task blocked during execution",
		})
	case RunSummaryStatusWaiting:
		score -= cfg.Routing.Adaptive.WaitPenalty
		evidence = append(evidence, protocol.AdaptiveRoutingEvidenceRef{
			RunID:        runID,
			SourceTaskID: item.SourceTaskID,
			MessageID:    sourceMessageID,
			Status:       "waiting",
			Note:         "source task waiting on a dependency or external event",
		})
	}

	return score, evidence
}

func adaptivePreferenceKey(taskClass protocol.TaskClass, normalizedDomains []string, preferredOwner protocol.AgentName) string {
	return fmt.Sprintf("%s|%s|%s", taskClass, strings.Join(normalizedDomains, ","), preferredOwner)
}

func hasAdaptiveRoutingSourceTaskMetadata(task protocol.ChildTask) bool {
	return task.TaskClass != "" && len(task.NormalizedDomains) > 0 && strings.TrimSpace(string(task.Owner)) != ""
}

func compareAdaptiveEvidence(left, right protocol.AdaptiveRoutingEvidenceRef) int {
	if diff := strings.Compare(string(left.RunID), string(right.RunID)); diff != 0 {
		return diff
	}
	if diff := strings.Compare(string(left.SourceTaskID), string(right.SourceTaskID)); diff != 0 {
		return diff
	}
	if diff := strings.Compare(string(left.MessageID), string(right.MessageID)); diff != 0 {
		return diff
	}
	if diff := adaptiveEvidenceStatusRank(left.Status) - adaptiveEvidenceStatusRank(right.Status); diff != 0 {
		return diff
	}

	return strings.Compare(left.Note, right.Note)
}

func adaptiveEvidenceStatusRank(status string) int {
	switch status {
	case "completed":
		return 1
	case "approved":
		return 2
	case "changes_requested":
		return 3
	case "blocked":
		return 4
	case "waiting":
		return 5
	case "escalated":
		return 6
	default:
		return 99
	}
}
