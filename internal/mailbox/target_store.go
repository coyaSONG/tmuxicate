package mailbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

type TargetAvailability string

const (
	TargetAvailabilityUnknown  TargetAvailability = "unknown"
	TargetAvailabilityReady    TargetAvailability = "ready"
	TargetAvailabilityDegraded TargetAvailability = "degraded"
	TargetAvailabilityOffline  TargetAvailability = "offline"
	TargetAvailabilityDisabled TargetAvailability = "disabled"
)

type TargetDispatchStatus string

const (
	TargetDispatchPending    TargetDispatchStatus = "pending"
	TargetDispatchDispatched TargetDispatchStatus = "dispatched"
	TargetDispatchFailed     TargetDispatchStatus = "failed"
)

type TargetState struct {
	Schema          string             `json:"schema"`
	Name            string             `json:"name"`
	Kind            string             `json:"kind"`
	PaneBacked      bool               `json:"pane_backed"`
	Capabilities    []string           `json:"capabilities,omitempty"`
	Availability    TargetAvailability `json:"availability"`
	Summary         string             `json:"summary,omitempty"`
	Source          string             `json:"source,omitempty"`
	UpdatedAt       string             `json:"updated_at"`
	LastHeartbeatAt string             `json:"last_heartbeat_at,omitempty"`
	LastDispatchAt  string             `json:"last_dispatch_at,omitempty"`
	LastError       string             `json:"last_error,omitempty"`
	DisabledReason  string             `json:"disabled_reason,omitempty"`
}

type TargetDispatchRecord struct {
	Schema       string               `json:"schema"`
	TargetName   string               `json:"target_name"`
	TargetKind   string               `json:"target_kind"`
	Agent        string               `json:"agent"`
	RunID        string               `json:"run_id,omitempty"`
	TaskID       string               `json:"task_id,omitempty"`
	MessageID    string               `json:"message_id"`
	Command      string               `json:"command,omitempty"`
	Status       TargetDispatchStatus `json:"status"`
	Summary      string               `json:"summary,omitempty"`
	Error        string               `json:"error,omitempty"`
	CreatedAt    string               `json:"created_at"`
	UpdatedAt    string               `json:"updated_at"`
	DispatchedAt string               `json:"dispatched_at,omitempty"`
}

func DefaultTargetState(target protocol.ExecutionTarget) *TargetState {
	availability := TargetAvailabilityUnknown
	summary := "target has not reported health yet"
	if target.Kind == "local" && target.PaneBacked {
		availability = TargetAvailabilityReady
		summary = "implicit local pane-backed target"
	}

	return &TargetState{
		Schema:       "tmuxicate/target-health/v1",
		Name:         target.Name,
		Kind:         target.Kind,
		PaneBacked:   target.PaneBacked,
		Capabilities: append([]string(nil), target.Capabilities...),
		Availability: availability,
		Summary:      summary,
		Source:       "default",
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func (a TargetAvailability) BlocksRouting() bool {
	switch a {
	case TargetAvailabilityOffline, TargetAvailabilityDisabled:
		return true
	default:
		return false
	}
}

func ReadTargetState(stateDir, target string) (*TargetState, error) {
	data, err := os.ReadFile(TargetStatePath(stateDir, target))
	if err != nil {
		return nil, err
	}
	var state TargetState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode target state: %w", err)
	}
	return &state, nil
}

func UpsertTargetState(stateDir string, target protocol.ExecutionTarget, mutate func(*TargetState) error) (*TargetState, error) {
	if strings.TrimSpace(target.Name) == "" {
		return nil, errors.New("target name is required")
	}

	state, err := ReadTargetState(stateDir, target.Name)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		state = DefaultTargetState(target)
	}
	if state.Name == "" {
		state.Name = target.Name
	}
	if state.Kind == "" {
		state.Kind = target.Kind
	}
	if len(state.Capabilities) == 0 && len(target.Capabilities) > 0 {
		state.Capabilities = append([]string(nil), target.Capabilities...)
	}
	state.PaneBacked = target.PaneBacked

	if mutate != nil {
		if err := mutate(state); err != nil {
			return nil, err
		}
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := writeTargetState(stateDir, state); err != nil {
		return nil, err
	}
	return state, nil
}

func RecordTargetHeartbeat(stateDir string, target protocol.ExecutionTarget, availability TargetAvailability, summary, source string, capabilities []string) (*TargetState, error) {
	state, err := UpsertTargetState(stateDir, target, func(state *TargetState) error {
		state.Availability = availability
		state.Summary = strings.TrimSpace(summary)
		state.Source = strings.TrimSpace(source)
		state.LastHeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
		if len(capabilities) > 0 {
			state.Capabilities = append([]string(nil), capabilities...)
		}
		if availability != TargetAvailabilityDisabled {
			state.DisabledReason = ""
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return state, appendJSONLine(TargetHealthLogPath(stateDir, target.Name), state)
}

func ReadTargetDispatch(stateDir, target string, msgID protocol.MessageID) (*TargetDispatchRecord, error) {
	data, err := os.ReadFile(TargetDispatchPath(stateDir, target, msgID))
	if err != nil {
		return nil, err
	}
	var record TargetDispatchRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode target dispatch: %w", err)
	}
	return &record, nil
}

func WriteTargetDispatch(stateDir string, record *TargetDispatchRecord) error {
	if record == nil {
		return errors.New("dispatch record is required")
	}
	if strings.TrimSpace(record.TargetName) == "" {
		return errors.New("target_name is required")
	}
	if strings.TrimSpace(record.MessageID) == "" {
		return errors.New("message_id is required")
	}
	if err := os.MkdirAll(TargetDispatchesDir(stateDir, record.TargetName), 0o755); err != nil {
		return fmt.Errorf("create target dispatch dir: %w", err)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal target dispatch: %w", err)
	}
	if err := os.WriteFile(TargetDispatchPath(stateDir, record.TargetName, protocol.MessageID(record.MessageID)), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write target dispatch: %w", err)
	}
	return appendJSONLine(TargetDispatchLogPath(stateDir, record.TargetName), record)
}

func ListTargetDispatches(stateDir, target string) ([]TargetDispatchRecord, error) {
	entries, err := os.ReadDir(TargetDispatchesDir(stateDir, target))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	records := make([]TargetDispatchRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		record, err := ReadTargetDispatch(stateDir, target, protocol.MessageID(strings.TrimSuffix(entry.Name(), ".json")))
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].UpdatedAt != records[j].UpdatedAt {
			return records[i].UpdatedAt > records[j].UpdatedAt
		}
		return records[i].MessageID < records[j].MessageID
	})
	return records, nil
}

func writeTargetState(stateDir string, state *TargetState) error {
	if state == nil {
		return errors.New("target state is required")
	}
	if err := os.MkdirAll(TargetEventsDir(stateDir, state.Name), 0o755); err != nil {
		return fmt.Errorf("create target events dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal target state: %w", err)
	}
	if err := os.WriteFile(TargetStatePath(stateDir, state.Name), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write target state: %w", err)
	}
	return nil
}

func appendJSONLine(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create jsonl dir: %w", err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal jsonl value: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open jsonl path: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append jsonl value: %w", err)
	}
	return nil
}
