package mailbox

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
)

type Store struct {
	stateDir string
}

func NewStore(stateDir string) *Store {
	return &Store{stateDir: SessionDir(stateDir)}
}

func (s *Store) CreateMessage(env *protocol.Envelope, body []byte) error {
	if err := env.Validate(); err != nil {
		return fmt.Errorf("validate envelope: %w", err)
	}
	if err := validateBody(env, body); err != nil {
		return err
	}

	if err := ensureDir(MessagesDir(s.stateDir)); err != nil {
		return err
	}
	if err := ensureDir(StagingDir(s.stateDir)); err != nil {
		return err
	}

	stageDir := filepath.Join(StagingDir(s.stateDir), fmt.Sprintf("%s.%d.tmp", env.ID, os.Getpid()))
	finalDir := MessageDir(s.stateDir, env.ID)

	if err := os.RemoveAll(stageDir); err != nil {
		return fmt.Errorf("clean staging dir: %w", err)
	}
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}

	stageEnvelopePath := filepath.Join(stageDir, envelopeFileName)
	stageBodyPath := filepath.Join(stageDir, bodyFileName)

	envelopeBytes, err := yaml.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	if err := writeFileAtomicallyInPlace(stageEnvelopePath, envelopeBytes, 0o644); err != nil {
		_ = os.RemoveAll(stageDir)
		return fmt.Errorf("write envelope: %w", err)
	}
	if err := writeFileAtomicallyInPlace(stageBodyPath, body, 0o644); err != nil {
		_ = os.RemoveAll(stageDir)
		return fmt.Errorf("write body: %w", err)
	}

	if err := syncDir(stageDir); err != nil {
		_ = os.RemoveAll(stageDir)
		return fmt.Errorf("sync staging dir: %w", err)
	}

	if _, err := os.Stat(finalDir); err == nil {
		_ = os.RemoveAll(stageDir)
		return fmt.Errorf("message %s already exists", env.ID)
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = os.RemoveAll(stageDir)
		return fmt.Errorf("stat final dir: %w", err)
	}

	if err := os.Rename(stageDir, finalDir); err != nil {
		_ = os.RemoveAll(stageDir)
		return fmt.Errorf("rename staging dir: %w", err)
	}

	if err := syncDir(MessagesDir(s.stateDir)); err != nil {
		return fmt.Errorf("sync messages dir: %w", err)
	}

	return nil
}

func (s *Store) ReadMessage(msgID protocol.MessageID) (*protocol.Envelope, []byte, error) {
	envelopePath := EnvelopePath(s.stateDir, msgID)
	bodyPath := BodyPath(s.stateDir, msgID)

	envelopeBytes, err := os.ReadFile(envelopePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read envelope: %w", err)
	}

	var env protocol.Envelope
	if err := yaml.Unmarshal(envelopeBytes, &env); err != nil {
		return nil, nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	if err := env.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validate envelope: %w", err)
	}

	body, err := os.ReadFile(bodyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read body: %w", err)
	}
	if err := validateBody(&env, body); err != nil {
		return nil, nil, err
	}

	return &env, body, nil
}

func (s *Store) CreateReceipt(receipt *protocol.Receipt) error {
	if err := receipt.Validate(); err != nil {
		return fmt.Errorf("validate receipt: %w", err)
	}

	agent := string(receipt.Recipient)
	folder := receipt.FolderState
	path := ReceiptPath(s.stateDir, agent, folder, receipt)

	if err := ensureDir(InboxDir(s.stateDir, agent, folder)); err != nil {
		return err
	}

	data, err := yaml.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("marshal receipt: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("receipt already exists at %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat receipt: %w", err)
	}

	if err := writeFileAtomically(path, data, 0o644); err != nil {
		return fmt.Errorf("write receipt: %w", err)
	}

	return nil
}

func (s *Store) ReadReceipt(agent string, msgID protocol.MessageID) (*protocol.Receipt, error) {
	path, _, err := s.findReceiptPath(agent, msgID)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read receipt: %w", err)
	}

	var receipt protocol.Receipt
	if err := yaml.Unmarshal(data, &receipt); err != nil {
		return nil, fmt.Errorf("unmarshal receipt: %w", err)
	}
	if err := receipt.Validate(); err != nil {
		return nil, fmt.Errorf("validate receipt: %w", err)
	}

	return &receipt, nil
}

func (s *Store) UpdateReceipt(agent string, msgID protocol.MessageID, fn func(*protocol.Receipt)) error {
	unlock, err := s.lockReceipt(agent, msgID)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()

	path, folder, err := s.findReceiptPath(agent, msgID)
	if err != nil {
		return err
	}

	receipt, err := s.readReceiptFile(path)
	if err != nil {
		return err
	}
	originalFolder := receipt.FolderState

	fn(receipt)

	if receipt.FolderState != originalFolder {
		return errors.New("receipt folder_state changes must use MoveReceipt")
	}
	if receipt.FolderState != folder {
		return errors.New("receipt folder_state does not match current folder")
	}
	if err := receipt.Validate(); err != nil {
		return fmt.Errorf("validate updated receipt: %w", err)
	}

	return s.writeReceiptAtPath(path, receipt)
}

func (s *Store) MoveReceipt(agent string, msgID protocol.MessageID, from, to protocol.FolderState) error {
	unlock, err := s.lockReceipt(agent, msgID)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()

	path, folder, err := s.findReceiptPath(agent, msgID)
	if err != nil {
		return err
	}
	if folder != from {
		return fmt.Errorf("receipt %s is in %s, not %s", msgID, folder, from)
	}

	receipt, err := s.readReceiptFile(path)
	if err != nil {
		return err
	}
	receipt.FolderState = to
	if err := receipt.Validate(); err != nil {
		return fmt.Errorf("validate moved receipt: %w", err)
	}

	if err := ensureDir(InboxDir(s.stateDir, agent, to)); err != nil {
		return err
	}
	dst := ReceiptPath(s.stateDir, agent, to, receipt)
	if err := s.writeReceiptAtPath(dst, receipt); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove old receipt after move: %w", err)
	}
	if err := syncDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("sync source inbox dir: %w", err)
	}
	if err := syncDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("sync destination inbox dir: %w", err)
	}

	return nil
}

func (s *Store) AllocateSeq() (int64, error) {
	unlock, err := s.lockSequence()
	if err != nil {
		return 0, err
	}
	defer func() { _ = unlock() }()

	if err := ensureDir(StateDir(s.stateDir)); err != nil {
		return 0, err
	}

	path := NextSeqPath(s.stateDir)
	current := int64(0)
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return 0, fmt.Errorf("read next-seq: %w", err)
		}
	} else {
		value := strings.TrimSpace(string(data))
		if value != "" {
			current, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("parse next-seq: %w", err)
			}
		}
	}

	next := current + 1
	payload := []byte(fmt.Sprintf("%d\n", next))
	if err := writeFileAtomically(path, payload, 0o644); err != nil {
		return 0, fmt.Errorf("write next-seq: %w", err)
	}

	return next, nil
}

func (s *Store) findReceiptPath(agent string, msgID protocol.MessageID) (string, protocol.FolderState, error) {
	for _, folder := range []protocol.FolderState{
		protocol.FolderStateUnread,
		protocol.FolderStateActive,
		protocol.FolderStateDone,
		protocol.FolderStateDead,
	} {
		dir := InboxDir(s.stateDir, agent, folder)
		matches, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("*-%s.yaml", msgID)))
		if err != nil {
			return "", "", fmt.Errorf("glob receipt: %w", err)
		}
		switch len(matches) {
		case 0:
			continue
		case 1:
			return matches[0], folder, nil
		default:
			return "", "", fmt.Errorf("multiple receipts found for %s/%s in %s", agent, msgID, dir)
		}
	}

	return "", "", fmt.Errorf("receipt %s for agent %s not found", msgID, agent)
}

func (s *Store) readReceiptFile(path string) (*protocol.Receipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read receipt: %w", err)
	}

	var receipt protocol.Receipt
	if err := yaml.Unmarshal(data, &receipt); err != nil {
		return nil, fmt.Errorf("unmarshal receipt: %w", err)
	}
	if err := receipt.Validate(); err != nil {
		return nil, fmt.Errorf("validate receipt: %w", err)
	}

	return &receipt, nil
}

func (s *Store) writeReceiptAtPath(path string, receipt *protocol.Receipt) error {
	data, err := yaml.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("marshal receipt: %w", err)
	}
	if err := writeFileAtomically(path, data, 0o644); err != nil {
		return fmt.Errorf("write receipt: %w", err)
	}
	return nil
}

func (s *Store) lockSequence() (func() error, error) {
	if err := ensureDir(LocksDir(s.stateDir)); err != nil {
		return nil, err
	}
	return flockPath(SequenceLockPath(s.stateDir))
}

func (s *Store) lockReceipt(agent string, msgID protocol.MessageID) (func() error, error) {
	if err := ensureDir(ReceiptLocksDir(s.stateDir, agent)); err != nil {
		return nil, err
	}
	return flockPath(ReceiptLockPath(s.stateDir, agent, msgID))
}

func validateBody(env *protocol.Envelope, body []byte) error {
	sum := sha256.Sum256(body)
	gotSHA := fmt.Sprintf("%x", sum[:])
	if env.BodySHA256 != gotSHA {
		return fmt.Errorf("body sha256 mismatch: envelope=%s actual=%s", env.BodySHA256, gotSHA)
	}
	if env.BodyBytes != int64(len(body)) {
		return fmt.Errorf("body byte count mismatch: envelope=%d actual=%d", env.BodyBytes, len(body))
	}
	return nil
}

func ensureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", path, err)
	}
	return nil
}

func writeFileAtomically(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := ensureDir(dir); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("sync parent dir: %w", err)
	}

	return nil
}

func writeFileAtomicallyInPlace(path string, data []byte, perm os.FileMode) error {
	return writeFileAtomically(path, data, perm)
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open dir %s: %w", path, err)
	}
	defer f.Close()

	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync dir %s: %w", path, err)
	}

	return nil
}

func flockPath(path string) (func() error, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("flock %s: %w", path, err)
	}

	return func() error {
		defer f.Close()
		if err := unix.Flock(int(f.Fd()), unix.LOCK_UN); err != nil {
			return fmt.Errorf("unlock %s: %w", path, err)
		}
		return nil
	}, nil
}
