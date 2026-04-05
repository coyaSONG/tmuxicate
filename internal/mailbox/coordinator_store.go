package mailbox

import (
	"errors"
	"fmt"
	"os"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"gopkg.in/yaml.v3"
)

type CoordinatorStore struct {
	stateDir string
}

func NewCoordinatorStore(stateDir string) *CoordinatorStore {
	return &CoordinatorStore{stateDir: SessionDir(stateDir)}
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
