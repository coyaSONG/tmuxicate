package mailbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"gopkg.in/yaml.v3"
)

type CoordinatorStore struct {
	stateDir string
}

func NewCoordinatorStore(stateDir string) *CoordinatorStore {
	return &CoordinatorStore{stateDir: SessionDir(stateDir)}
}

func LockRunRoute(stateDir string, runID protocol.RunID) (func() error, error) {
	path := RunRouteLockPath(stateDir, runID)
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return nil, err
	}

	return flockPath(path)
}

func (s *CoordinatorStore) CreateRun(run *protocol.CoordinatorRun) error {
	if run == nil {
		return errors.New("run is required")
	}
	if err := run.Validate(); err != nil {
		return fmt.Errorf("validate run: %w", err)
	}

	path := RunFilePath(s.stateDir, run.RunID)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("run %s already exists", run.RunID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat run: %w", err)
	}

	data, err := yaml.Marshal(run)
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}
	if err := writeFileAtomically(path, data, 0o644); err != nil {
		return fmt.Errorf("write run: %w", err)
	}

	return nil
}

func (s *CoordinatorStore) ReadAdaptiveRoutingPreferences(coordinator protocol.AgentName) (*protocol.AdaptiveRoutingPreferenceSet, error) {
	data, err := os.ReadFile(AdaptiveRoutingPreferencesPath(s.stateDir, coordinator))
	if err != nil {
		return nil, fmt.Errorf("read adaptive routing preferences: %w", err)
	}

	var preferences protocol.AdaptiveRoutingPreferenceSet
	if err := yaml.Unmarshal(data, &preferences); err != nil {
		return nil, fmt.Errorf("unmarshal adaptive routing preferences: %w", err)
	}
	if err := preferences.Validate(); err != nil {
		return nil, fmt.Errorf("validate adaptive routing preferences: %w", err)
	}

	return &preferences, nil
}

func (s *CoordinatorStore) WriteAdaptiveRoutingPreferences(preferences *protocol.AdaptiveRoutingPreferenceSet) error {
	if preferences == nil {
		return errors.New("adaptive routing preferences are required")
	}
	if err := preferences.Validate(); err != nil {
		return fmt.Errorf("validate adaptive routing preferences: %w", err)
	}

	path := AdaptiveRoutingPreferencesPath(s.stateDir, preferences.Coordinator)
	data, err := yaml.Marshal(preferences)
	if err != nil {
		return fmt.Errorf("marshal adaptive routing preferences: %w", err)
	}
	if err := writeFileAtomically(path, data, 0o644); err != nil {
		return fmt.Errorf("write adaptive routing preferences: %w", err)
	}

	return nil
}

func (s *CoordinatorStore) ReadRun(runID protocol.RunID) (*protocol.CoordinatorRun, error) {
	data, err := os.ReadFile(RunFilePath(s.stateDir, runID))
	if err != nil {
		return nil, fmt.Errorf("read run: %w", err)
	}

	var run protocol.CoordinatorRun
	if err := yaml.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("unmarshal run: %w", err)
	}
	if err := run.Validate(); err != nil {
		return nil, fmt.Errorf("validate run: %w", err)
	}

	return &run, nil
}

func (s *CoordinatorStore) CreateTask(task *protocol.ChildTask) error {
	if task == nil {
		return errors.New("task is required")
	}
	if err := task.Validate(); err != nil {
		return fmt.Errorf("validate task: %w", err)
	}

	path := RunTaskPath(s.stateDir, task.ParentRunID, task.TaskID)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("task %s already exists", task.TaskID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat task: %w", err)
	}

	data, err := yaml.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}
	if err := writeFileAtomically(path, data, 0o644); err != nil {
		return fmt.Errorf("write task: %w", err)
	}

	return nil
}

func (s *CoordinatorStore) ReadTask(runID protocol.RunID, taskID protocol.TaskID) (*protocol.ChildTask, error) {
	data, err := os.ReadFile(RunTaskPath(s.stateDir, runID, taskID))
	if err != nil {
		return nil, fmt.Errorf("read task: %w", err)
	}

	var task protocol.ChildTask
	if err := yaml.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}

	return &task, nil
}

func (s *CoordinatorStore) CreateReviewHandoff(handoff *protocol.ReviewHandoff) error {
	if handoff == nil {
		return errors.New("review handoff is required")
	}
	if err := handoff.Validate(); err != nil {
		return fmt.Errorf("validate review handoff: %w", err)
	}

	path := RunReviewHandoffPath(s.stateDir, handoff.RunID, handoff.SourceTaskID)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("review handoff for %s already exists", handoff.SourceTaskID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat review handoff: %w", err)
	}

	return s.writeReviewHandoff(path, handoff)
}

func (s *CoordinatorStore) CreateBlockerCase(caseDoc *protocol.BlockerCase) error {
	if caseDoc == nil {
		return errors.New("blocker case is required")
	}
	if err := caseDoc.Validate(); err != nil {
		return fmt.Errorf("validate blocker case: %w", err)
	}

	path := RunBlockerCasePath(s.stateDir, caseDoc.RunID, caseDoc.SourceTaskID)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("blocker case for %s already exists", caseDoc.SourceTaskID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat blocker case: %w", err)
	}

	return s.writeBlockerCase(path, caseDoc)
}

func (s *CoordinatorStore) ReadBlockerCase(runID protocol.RunID, sourceTaskID protocol.TaskID) (*protocol.BlockerCase, error) {
	data, err := os.ReadFile(RunBlockerCasePath(s.stateDir, runID, sourceTaskID))
	if err != nil {
		return nil, fmt.Errorf("read blocker case: %w", err)
	}

	var caseDoc protocol.BlockerCase
	if err := yaml.Unmarshal(data, &caseDoc); err != nil {
		return nil, fmt.Errorf("unmarshal blocker case: %w", err)
	}
	if err := caseDoc.Validate(); err != nil {
		return nil, fmt.Errorf("validate blocker case: %w", err)
	}
	if caseDoc.RunID != runID {
		return nil, fmt.Errorf("validate blocker case: run_id %s does not match path", caseDoc.RunID)
	}
	if caseDoc.SourceTaskID != sourceTaskID {
		return nil, fmt.Errorf("validate blocker case: source_task_id %s does not match path", caseDoc.SourceTaskID)
	}

	return &caseDoc, nil
}

func (s *CoordinatorStore) UpdateBlockerCase(runID protocol.RunID, sourceTaskID protocol.TaskID, updateFn func(*protocol.BlockerCase) error) error {
	if updateFn == nil {
		return errors.New("updateFn is required")
	}

	caseDoc, err := s.ReadBlockerCase(runID, sourceTaskID)
	if err != nil {
		return err
	}

	if err := updateFn(caseDoc); err != nil {
		return err
	}
	if err := caseDoc.Validate(); err != nil {
		return fmt.Errorf("validate updated blocker case: %w", err)
	}
	if caseDoc.RunID != runID {
		return errors.New("updated blocker case run_id must match path")
	}
	if caseDoc.SourceTaskID != sourceTaskID {
		return errors.New("updated blocker case source_task_id must match path")
	}

	return s.writeBlockerCase(RunBlockerCasePath(s.stateDir, runID, sourceTaskID), caseDoc)
}

func (s *CoordinatorStore) FindBlockerCaseByCurrentTaskID(runID protocol.RunID, currentTaskID protocol.TaskID) (*protocol.BlockerCase, error) {
	if currentTaskID == "" {
		return nil, errors.New("currentTaskID is required")
	}

	entries, err := os.ReadDir(RunBlockersDir(s.stateDir, runID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("blocker case for current task %s: %w", currentTaskID, os.ErrNotExist)
		}
		return nil, fmt.Errorf("read blocker cases: %w", err)
	}

	var matched *protocol.BlockerCase
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		sourceTaskID := protocol.TaskID(entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))])
		caseDoc, err := s.ReadBlockerCase(runID, sourceTaskID)
		if err != nil {
			return nil, err
		}
		if caseDoc.CurrentTaskID != currentTaskID {
			continue
		}
		if matched != nil {
			return nil, fmt.Errorf("multiple blocker cases found for current task %s", currentTaskID)
		}
		matched = caseDoc
	}

	if matched == nil {
		return nil, fmt.Errorf("blocker case for current task %s: %w", currentTaskID, os.ErrNotExist)
	}

	return matched, nil
}

func (s *CoordinatorStore) ReadReviewHandoff(runID protocol.RunID, sourceTaskID protocol.TaskID) (*protocol.ReviewHandoff, error) {
	data, err := os.ReadFile(RunReviewHandoffPath(s.stateDir, runID, sourceTaskID))
	if err != nil {
		return nil, fmt.Errorf("read review handoff: %w", err)
	}

	var handoff protocol.ReviewHandoff
	if err := yaml.Unmarshal(data, &handoff); err != nil {
		return nil, fmt.Errorf("unmarshal review handoff: %w", err)
	}
	if err := handoff.Validate(); err != nil {
		return nil, fmt.Errorf("validate review handoff: %w", err)
	}

	return &handoff, nil
}

func (s *CoordinatorStore) FindReviewHandoffByReviewTaskID(runID protocol.RunID, reviewTaskID protocol.TaskID) (*protocol.ReviewHandoff, error) {
	entries, err := os.ReadDir(RunReviewsDir(s.stateDir, runID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("review handoff for review task %s: %w", reviewTaskID, os.ErrNotExist)
		}
		return nil, fmt.Errorf("read review handoffs: %w", err)
	}

	var matched *protocol.ReviewHandoff
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		sourceTaskID := protocol.TaskID(entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))])
		handoff, err := s.ReadReviewHandoff(runID, sourceTaskID)
		if err != nil {
			return nil, err
		}
		if handoff.ReviewTaskID != reviewTaskID {
			continue
		}
		if matched != nil {
			return nil, fmt.Errorf("multiple review handoffs found for review task %s", reviewTaskID)
		}
		matched = handoff
	}

	if matched == nil {
		return nil, fmt.Errorf("review handoff for review task %s: %w", reviewTaskID, os.ErrNotExist)
	}

	return matched, nil
}

func (s *CoordinatorStore) UpdateReviewHandoff(runID protocol.RunID, sourceTaskID protocol.TaskID, updateFn func(*protocol.ReviewHandoff) error) error {
	if updateFn == nil {
		return errors.New("updateFn is required")
	}

	handoff, err := s.ReadReviewHandoff(runID, sourceTaskID)
	if err != nil {
		return err
	}

	if err := updateFn(handoff); err != nil {
		return err
	}
	if err := handoff.Validate(); err != nil {
		return fmt.Errorf("validate updated review handoff: %w", err)
	}

	return s.writeReviewHandoff(RunReviewHandoffPath(s.stateDir, runID, sourceTaskID), handoff)
}

func (s *CoordinatorStore) writeReviewHandoff(path string, handoff *protocol.ReviewHandoff) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}

	data, err := yaml.Marshal(handoff)
	if err != nil {
		return fmt.Errorf("marshal review handoff: %w", err)
	}
	if err := writeFileAtomically(path, data, 0o644); err != nil {
		return fmt.Errorf("write review handoff: %w", err)
	}

	return nil
}

func (s *CoordinatorStore) writeBlockerCase(path string, caseDoc *protocol.BlockerCase) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}

	data, err := yaml.Marshal(caseDoc)
	if err != nil {
		return fmt.Errorf("marshal blocker case: %w", err)
	}
	if err := writeFileAtomically(path, data, 0o644); err != nil {
		return fmt.Errorf("write blocker case: %w", err)
	}

	return nil
}
