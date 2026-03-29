package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/tmux"
	"gopkg.in/yaml.v3"
)

type StatusReport struct {
	SessionName   string
	State         string
	Uptime        time.Duration
	DaemonHealthy bool
	AgentStatuses []AgentStatus
	FlowStats     FlowStats
	ThreadStats   ThreadStats
}

type AgentStatus struct {
	Name          string
	Alias         string
	PaneID        string
	ObservedState string
	DeclaredState string
	UnreadCount   int
	ActiveCount   int
	LastEvent     *time.Time
}

type FlowStats struct {
	Sent     int
	Acked    int
	Done     int
	Pending  int
	Retrying int
	Failed   int
}

type ThreadStats struct {
	Open     int
	Resolved int
	Closed   int
}

type daemonHeartbeat struct {
	UpdatedAt string `json:"updated_at"`
}

type observedStateFile struct {
	Timestamp     string `json:"ts"`
	ObservedState string `json:"observed_state"`
}

type declaredStateFile struct {
	Timestamp     string `json:"ts"`
	DeclaredState string `json:"declared_state"`
}

type readyFile struct {
	StartedAt string            `json:"started_at"`
	Agents    map[string]string `json:"agents"`
}

type threadAggregate struct {
	HasOpen bool
	HasAny  bool
}

func Status(stateDir string, tmuxClient tmux.Client) (*StatusReport, error) {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return nil, err
	}

	report := &StatusReport{
		SessionName: cfg.Session.Name,
		State:       "stopped",
	}

	ctx := context.Background()
	hasSession, err := tmuxClient.HasSession(ctx, cfg.Session.Name)
	if err != nil {
		return nil, fmt.Errorf("check session: %w", err)
	}
	if hasSession {
		report.State = "running"
	}

	startedAt, paneIDs, _ := readReadyFile(filepath.Join(stateDir, "runtime", "ready.json"))
	if startedAt != nil {
		report.Uptime = time.Since(*startedAt).Round(time.Second)
	}

	report.DaemonHealthy = readHeartbeatHealthy(filepath.Join(stateDir, "runtime", "daemon.heartbeat.json"))

	livePaneIDs := map[string]string{}
	if hasSession {
		panes, err := tmuxClient.ListPanes(ctx, cfg.Session.Name)
		if err != nil {
			return nil, fmt.Errorf("list panes: %w", err)
		}
		for _, pane := range panes {
			agentName, err := tmuxClient.ShowPaneOption(ctx, pane.PaneID, "@tmuxicate-agent")
			if err != nil {
				return nil, fmt.Errorf("show pane option for %s: %w", pane.PaneID, err)
			}
			if strings.TrimSpace(agentName) != "" {
				livePaneIDs[agentName] = pane.PaneID
			}
		}
	}

	threads := map[string]*threadAggregate{}
	messageThreads := map[string]string{}
	if err := scanMessages(stateDir, func(path string) error {
		data, err := os.ReadFile(filepath.Join(path, "envelope.yaml"))
		if err != nil {
			return err
		}
		var env struct {
			ID     string `yaml:"id"`
			Thread string `yaml:"thread"`
		}
		if err := yaml.Unmarshal(data, &env); err != nil {
			return err
		}
		if env.ID == "" || env.Thread == "" {
			return nil
		}
		messageThreads[env.ID] = env.Thread
		if _, ok := threads[env.Thread]; !ok {
			threads[env.Thread] = &threadAggregate{}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan messages: %w", err)
	}
	report.FlowStats.Sent = len(messageThreads)

	for _, agent := range cfg.Agents {
		status := AgentStatus{
			Name:   agent.Name,
			Alias:  agent.Alias,
			PaneID: coalescePaneID(livePaneIDs[agent.Name], paneIDs[agent.Name]),
		}

		observed, observedTS, err := readObservedState(filepath.Join(stateDir, "agents", agent.Name, "events", "observed.current.json"))
		if err != nil {
			return nil, err
		}
		declared, declaredTS, err := readDeclaredState(filepath.Join(stateDir, "agents", agent.Name, "events", "state.current.json"))
		if err != nil {
			return nil, err
		}

		status.ObservedState = observed
		status.DeclaredState = declared
		status.LastEvent = maxTimePtr(observedTS, declaredTS)

		unreadCount, err := countReceiptFiles(filepath.Join(stateDir, "agents", agent.Name, "inbox", "unread"))
		if err != nil {
			return nil, err
		}
		activeCount, err := countReceiptFiles(filepath.Join(stateDir, "agents", agent.Name, "inbox", "active"))
		if err != nil {
			return nil, err
		}
		status.UnreadCount = unreadCount
		status.ActiveCount = activeCount

		if err := scanReceiptsForAgent(stateDir, agent.Name, func(folder string, receiptPath string, receipt receiptSummary) {
			if receipt.AckedAt != "" {
				report.FlowStats.Acked++
			}
			switch folder {
			case "done":
				report.FlowStats.Done++
			case "unread", "active":
				report.FlowStats.Pending++
			case "dead":
				report.FlowStats.Failed++
			}
			if folder == "unread" && receipt.NextRetryAt != "" {
				report.FlowStats.Retrying++
			}

			threadID := messageThreads[receipt.MessageID]
			if threadID == "" {
				return
			}
			agg := threads[threadID]
			if agg == nil {
				agg = &threadAggregate{}
				threads[threadID] = agg
			}
			agg.HasAny = true
			if folder == "unread" || folder == "active" {
				agg.HasOpen = true
			}
		}); err != nil {
			return nil, err
		}

		report.AgentStatuses = append(report.AgentStatuses, status)
	}

	sort.Slice(report.AgentStatuses, func(i, j int) bool {
		return report.AgentStatuses[i].Name < report.AgentStatuses[j].Name
	})

	for _, agg := range threads {
		if agg == nil || !agg.HasAny {
			continue
		}
		if agg.HasOpen {
			report.ThreadStats.Open++
		} else {
			report.ThreadStats.Resolved++
		}
	}

	return report, nil
}

type receiptSummary struct {
	MessageID   string `yaml:"message_id"`
	AckedAt     string `yaml:"acked_at"`
	NextRetryAt string `yaml:"next_retry_at"`
}

func countReceiptFiles(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			count++
		}
	}
	return count, nil
}

func scanMessages(stateDir string, fn func(path string) error) error {
	entries, err := os.ReadDir(filepath.Join(stateDir, "messages"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "orphaned" {
			continue
		}
		if err := fn(filepath.Join(stateDir, "messages", name)); err != nil {
			return err
		}
	}
	return nil
}

func scanReceiptsForAgent(stateDir, agent string, fn func(folder string, receiptPath string, receipt receiptSummary)) error {
	for _, folder := range []string{"unread", "active", "done", "dead"} {
		dir := filepath.Join(stateDir, "agents", agent, "inbox", folder)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var receipt receiptSummary
			if err := yaml.Unmarshal(data, &receipt); err != nil {
				return err
			}
			fn(folder, path, receipt)
		}
	}
	return nil
}

func readHeartbeatHealthy(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var hb daemonHeartbeat
	if err := json.Unmarshal(data, &hb); err != nil {
		return false
	}
	ts, err := time.Parse(time.RFC3339Nano, hb.UpdatedAt)
	if err != nil {
		return false
	}
	return time.Since(ts) <= 15*time.Second
}

func readReadyFile(path string) (*time.Time, map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, map[string]string{}, nil
		}
		return nil, nil, err
	}
	var rf readyFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil, nil, err
	}
	var startedAt *time.Time
	if strings.TrimSpace(rf.StartedAt) != "" {
		ts, err := time.Parse(time.RFC3339Nano, rf.StartedAt)
		if err == nil {
			startedAt = &ts
		}
	}
	if rf.Agents == nil {
		rf.Agents = map[string]string{}
	}
	return startedAt, rf.Agents, nil
}

func readObservedState(path string) (string, *time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "-", nil, nil
		}
		return "", nil, err
	}
	var state observedStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return "", nil, err
	}
	ts, _ := parseTimePtr(state.Timestamp)
	if strings.TrimSpace(state.ObservedState) == "" {
		return "-", ts, nil
	}
	return state.ObservedState, ts, nil
}

func readDeclaredState(path string) (string, *time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "-", nil, nil
		}
		return "", nil, err
	}
	var state declaredStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return "", nil, err
	}
	ts, _ := parseTimePtr(state.Timestamp)
	if strings.TrimSpace(state.DeclaredState) == "" {
		return "-", ts, nil
	}
	return state.DeclaredState, ts, nil
}

func parseTimePtr(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	ts, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, err
	}
	return &ts, nil
}

func maxTimePtr(values ...*time.Time) *time.Time {
	var max *time.Time
	for _, value := range values {
		if value == nil {
			continue
		}
		if max == nil || value.After(*max) {
			v := *value
			max = &v
		}
	}
	return max
}

func coalescePaneID(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "-"
}
