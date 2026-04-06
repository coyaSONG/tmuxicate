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
