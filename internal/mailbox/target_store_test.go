package mailbox

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestRecordTargetHeartbeatPersistsStateAndLog(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	target := testExecutionTarget("sandbox")

	got, err := RecordTargetHeartbeat(stateDir, &target, TargetAvailabilityReady, "sandbox ready", "worker", []string{"fast", "sandbox"})
	if err != nil {
		t.Fatalf("RecordTargetHeartbeat() unexpected error: %v", err)
	}
	if got.Name != target.Name {
		t.Fatalf("Name = %q, want %q", got.Name, target.Name)
	}
	if got.Availability != TargetAvailabilityReady {
		t.Fatalf("Availability = %q, want %q", got.Availability, TargetAvailabilityReady)
	}
	if got.Summary != "sandbox ready" {
		t.Fatalf("Summary = %q, want sandbox ready", got.Summary)
	}
	if got.Source != "worker" {
		t.Fatalf("Source = %q, want worker", got.Source)
	}
	if got.LastHeartbeatAt == "" {
		t.Fatal("LastHeartbeatAt should be set")
	}

	stored, err := ReadTargetState(stateDir, target.Name)
	if err != nil {
		t.Fatalf("ReadTargetState() unexpected error: %v", err)
	}
	if stored.Availability != TargetAvailabilityReady {
		t.Fatalf("stored Availability = %q, want %q", stored.Availability, TargetAvailabilityReady)
	}

	lines := readJSONLLines(t, TargetHealthLogPath(stateDir, target.Name))
	if len(lines) != 1 {
		t.Fatalf("health log lines = %d, want 1", len(lines))
	}
	var logged TargetState
	if err := json.Unmarshal([]byte(lines[0]), &logged); err != nil {
		t.Fatalf("decode health log: %v", err)
	}
	if logged.Summary != "sandbox ready" {
		t.Fatalf("logged Summary = %q, want sandbox ready", logged.Summary)
	}
}

func TestWriteTargetDispatchRoundTripAndOrdering(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	target := "sandbox"
	older := testTargetDispatch(target, 101, time.Now().UTC())
	newer := testTargetDispatch(target, 102, time.Now().UTC().Add(time.Second))

	if err := WriteTargetDispatch(stateDir, older); err != nil {
		t.Fatalf("WriteTargetDispatch(older) unexpected error: %v", err)
	}
	if err := WriteTargetDispatch(stateDir, newer); err != nil {
		t.Fatalf("WriteTargetDispatch(newer) unexpected error: %v", err)
	}

	got, err := ReadTargetDispatch(stateDir, target, protocol.MessageID(newer.MessageID))
	if err != nil {
		t.Fatalf("ReadTargetDispatch() unexpected error: %v", err)
	}
	if got.MessageID != newer.MessageID {
		t.Fatalf("MessageID = %q, want %q", got.MessageID, newer.MessageID)
	}

	records, err := ListTargetDispatches(stateDir, target)
	if err != nil {
		t.Fatalf("ListTargetDispatches() unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	if records[0].MessageID != newer.MessageID || records[1].MessageID != older.MessageID {
		t.Fatalf("dispatch order = [%s, %s], want [%s, %s]", records[0].MessageID, records[1].MessageID, newer.MessageID, older.MessageID)
	}
}

func TestRecordTargetHeartbeatConcurrentWritesRemainReadable(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	target := testExecutionTarget("remote")
	const workers = 25

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	wg.Add(workers)
	for i := range workers {
		go func(i int) {
			defer wg.Done()
			_, err := RecordTargetHeartbeat(stateDir, &target, TargetAvailabilityDegraded, fmt.Sprintf("heartbeat-%02d", i), "worker", nil)
			if err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("RecordTargetHeartbeat() concurrent error: %v", err)
		}
	}

	stored, err := ReadTargetState(stateDir, target.Name)
	if err != nil {
		t.Fatalf("ReadTargetState() unexpected error: %v", err)
	}
	if stored.Availability != TargetAvailabilityDegraded {
		t.Fatalf("Availability = %q, want %q", stored.Availability, TargetAvailabilityDegraded)
	}
	if !strings.HasPrefix(stored.Summary, "heartbeat-") {
		t.Fatalf("Summary = %q, want heartbeat prefix", stored.Summary)
	}

	lines := readJSONLLines(t, TargetHealthLogPath(stateDir, target.Name))
	if len(lines) != workers {
		t.Fatalf("health log lines = %d, want %d", len(lines), workers)
	}
	for i, line := range lines {
		var logged TargetState
		if err := json.Unmarshal([]byte(line), &logged); err != nil {
			t.Fatalf("decode health log line %d: %v", i, err)
		}
	}
}

func TestWriteTargetDispatchConcurrentWritesRemainReadable(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	target := "sandbox"
	const workers = 20

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	wg.Add(workers)
	for i := range workers {
		go func(i int) {
			defer wg.Done()
			record := testTargetDispatch(target, int64(200+i), time.Now().UTC().Add(time.Duration(i)*time.Millisecond))
			if err := WriteTargetDispatch(stateDir, record); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("WriteTargetDispatch() concurrent error: %v", err)
		}
	}

	records, err := ListTargetDispatches(stateDir, target)
	if err != nil {
		t.Fatalf("ListTargetDispatches() unexpected error: %v", err)
	}
	if len(records) != workers {
		t.Fatalf("records = %d, want %d", len(records), workers)
	}
	for _, record := range records {
		if _, err := ReadTargetDispatch(stateDir, target, protocol.MessageID(record.MessageID)); err != nil {
			t.Fatalf("ReadTargetDispatch(%s) unexpected error: %v", record.MessageID, err)
		}
	}

	lines := readJSONLLines(t, TargetDispatchLogPath(stateDir, target))
	if len(lines) != workers {
		t.Fatalf("dispatch log lines = %d, want %d", len(lines), workers)
	}
}

func TestUpsertTargetStateRequiresTarget(t *testing.T) {
	t.Parallel()

	if _, err := UpsertTargetState(t.TempDir(), nil, nil); err == nil {
		t.Fatal("UpsertTargetState() expected nil target error, got nil")
	}
}

func TestUpsertTargetStateAllowsNestedTargetLockPath(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	target := testExecutionTarget("remote/linux")

	if _, err := UpsertTargetState(stateDir, &target, nil); err != nil {
		t.Fatalf("UpsertTargetState() unexpected error: %v", err)
	}
	if _, err := ReadTargetState(stateDir, target.Name); err != nil {
		t.Fatalf("ReadTargetState() unexpected error: %v", err)
	}
	if _, err := os.Stat(TargetLockPath(stateDir, target.Name)); err != nil {
		t.Fatalf("target lock path missing: %v", err)
	}
}

func TestDefaultTargetStateForLocalPaneBackedTarget(t *testing.T) {
	t.Parallel()

	target := protocol.ExecutionTarget{Name: "local", Kind: "local", PaneBacked: true, Capabilities: []string{"local", "pane"}}
	state := DefaultTargetState(&target)

	if state.Availability != TargetAvailabilityReady {
		t.Fatalf("Availability = %q, want %q", state.Availability, TargetAvailabilityReady)
	}
	if state.Summary != "implicit local pane-backed target" {
		t.Fatalf("Summary = %q, want implicit local pane-backed target", state.Summary)
	}
	if len(state.Capabilities) != len(target.Capabilities) {
		t.Fatalf("Capabilities = %v, want %v", state.Capabilities, target.Capabilities)
	}
}

func TestWriteTargetDispatchValidation(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	tests := []struct {
		name   string
		record *TargetDispatchRecord
	}{
		{name: "nil", record: nil},
		{name: "blank target", record: &TargetDispatchRecord{MessageID: string(protocol.NewMessageID(1))}},
		{name: "blank message", record: &TargetDispatchRecord{TargetName: "sandbox"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := WriteTargetDispatch(stateDir, tt.record); err == nil {
				t.Fatal("WriteTargetDispatch() expected error, got nil")
			}
		})
	}
}

func testExecutionTarget(name string) protocol.ExecutionTarget {
	return protocol.ExecutionTarget{
		Name:         name,
		Kind:         "sandbox",
		Description:  "test sandbox target",
		Capabilities: []string{"sandbox"},
		PaneBacked:   false,
	}
}

func testTargetDispatch(target string, seq int64, updated time.Time) *TargetDispatchRecord {
	return &TargetDispatchRecord{
		Schema:       "tmuxicate/target-dispatch/v1",
		TargetName:   target,
		TargetKind:   "sandbox",
		Agent:        "worker",
		RunID:        "run_test",
		TaskID:       fmt.Sprintf("task_%d", seq),
		MessageID:    string(protocol.NewMessageID(seq)),
		Command:      "echo dispatch",
		Status:       TargetDispatchDispatched,
		Summary:      "dispatch command completed",
		CreatedAt:    updated.Add(-time.Second).Format(time.RFC3339Nano),
		UpdatedAt:    updated.Format(time.RFC3339Nano),
		DispatchedAt: updated.Format(time.RFC3339Nano),
	}
}

func readJSONLLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) unexpected error: %v", path, err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
