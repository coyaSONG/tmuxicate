package mailbox

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestCreateMessage_AtomicVisibility(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	env, body := testEnvelope(142)

	if err := store.CreateMessage(env, body); err != nil {
		t.Fatalf("CreateMessage() unexpected error: %v", err)
	}

	msgDir := MessageDir(store.stateDir, env.ID)
	if _, err := os.Stat(msgDir); err != nil {
		t.Fatalf("message dir missing: %v", err)
	}
	if _, err := os.Stat(EnvelopePath(store.stateDir, env.ID)); err != nil {
		t.Fatalf("envelope missing: %v", err)
	}
	if _, err := os.Stat(BodyPath(store.stateDir, env.ID)); err != nil {
		t.Fatalf("body missing: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(StagingDir(store.stateDir), "*"))
	if err != nil {
		t.Fatalf("Glob() unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("staging dir not empty after commit: %v", matches)
	}
}

func TestCreateMessage_VerifySHA256(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	env, body := testEnvelope(143)

	if err := store.CreateMessage(env, body); err != nil {
		t.Fatalf("CreateMessage() unexpected error: %v", err)
	}

	if err := os.WriteFile(BodyPath(store.stateDir, env.ID), []byte("tampered\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() unexpected error: %v", err)
	}

	if _, _, err := store.ReadMessage(env.ID); err == nil {
		t.Fatal("ReadMessage() expected checksum error, got nil")
	}
}

func TestAllocateSeq_Unique(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())

	first, err := store.AllocateSeq()
	if err != nil {
		t.Fatalf("AllocateSeq() unexpected error: %v", err)
	}
	second, err := store.AllocateSeq()
	if err != nil {
		t.Fatalf("AllocateSeq() unexpected error: %v", err)
	}

	if first != 1 || second != 2 {
		t.Fatalf("AllocateSeq() values = %d, %d; want 1, 2", first, second)
	}
}

func TestCreateReceipt_And_MoveReceipt(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	receipt := testReceipt(142, protocol.FolderStateUnread)

	if err := store.CreateReceipt(receipt); err != nil {
		t.Fatalf("CreateReceipt() unexpected error: %v", err)
	}

	if err := store.MoveReceipt(string(receipt.Recipient), receipt.MessageID, protocol.FolderStateUnread, protocol.FolderStateActive); err != nil {
		t.Fatalf("MoveReceipt() unexpected error: %v", err)
	}

	got, err := store.ReadReceipt(string(receipt.Recipient), receipt.MessageID)
	if err != nil {
		t.Fatalf("ReadReceipt() unexpected error: %v", err)
	}
	if got.FolderState != protocol.FolderStateActive {
		t.Fatalf("Receipt folder_state = %q, want %q", got.FolderState, protocol.FolderStateActive)
	}
}

func TestUpdateReceipt_RequiresLock(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	receipt := testReceipt(144, protocol.FolderStateActive)
	now := time.Now().UTC()
	receipt.AckedAt = &now

	if err := store.CreateReceipt(receipt); err != nil {
		t.Fatalf("CreateReceipt() unexpected error: %v", err)
	}

	const updates = 25
	var wg sync.WaitGroup
	wg.Add(updates)

	for range updates {
		go func() {
			defer wg.Done()
			err := store.UpdateReceipt(string(receipt.Recipient), receipt.MessageID, func(r *protocol.Receipt) {
				r.Revision++
				r.NotifyAttempts++
			})
			if err != nil {
				t.Errorf("UpdateReceipt() unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	got, err := store.ReadReceipt(string(receipt.Recipient), receipt.MessageID)
	if err != nil {
		t.Fatalf("ReadReceipt() unexpected error: %v", err)
	}
	if got.Revision != int64(updates) {
		t.Fatalf("Revision = %d, want %d", got.Revision, updates)
	}
	if got.NotifyAttempts != updates {
		t.Fatalf("NotifyAttempts = %d, want %d", got.NotifyAttempts, updates)
	}
}

func TestConcurrentAllocateSeq(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	const workers = 20

	results := make(chan int64, workers)
	errs := make(chan error, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			seq, err := store.AllocateSeq()
			if err != nil {
				errs <- err
				return
			}
			results <- seq
		}()
	}

	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("AllocateSeq() unexpected error: %v", err)
		}
	}

	var values []int
	for seq := range results {
		values = append(values, int(seq))
	}
	sort.Ints(values)
	if len(values) != workers {
		t.Fatalf("got %d seq values, want %d", len(values), workers)
	}
	for i, seq := range values {
		want := i + 1
		if seq != want {
			t.Fatalf("values[%d] = %d, want %d; all values=%v", i, seq, want, values)
		}
	}
}

func testEnvelope(seq int64) (protocol.Envelope, []byte) {
	body := []byte(fmt.Sprintf("# Message %d\n\nHello.\n", seq))
	sum := sha256.Sum256(body)
	now := time.Now().UTC()

	return protocol.Envelope{
		Schema:     protocol.MessageSchemaV1,
		ID:         protocol.NewMessageID(seq),
		Seq:        seq,
		Session:    "dev",
		Thread:     protocol.NewThreadID(19),
		Kind:       protocol.KindTask,
		From:       protocol.AgentName("coordinator"),
		To:         []protocol.AgentName{protocol.AgentName("backend")},
		CreatedAt:  now,
		BodyFormat: protocol.BodyFormatMD,
		BodySHA256: fmt.Sprintf("%x", sum[:]),
		BodyBytes:  int64(len(body)),
		Priority:   protocol.PriorityNormal,
	}, body
}

func testReceipt(seq int64, folder protocol.FolderState) protocol.Receipt {
	return protocol.Receipt{
		Schema:         protocol.ReceiptSchemaV1,
		MessageID:      protocol.NewMessageID(seq),
		Seq:            seq,
		Recipient:      protocol.AgentName("backend"),
		FolderState:    folder,
		Revision:       0,
		NotifyAttempts: 0,
	}
}

func TestReadMessageRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())
	env, body := testEnvelope(145)
	if err := store.CreateMessage(env, body); err != nil {
		t.Fatalf("CreateMessage() unexpected error: %v", err)
	}

	gotEnv, gotBody, err := store.ReadMessage(env.ID)
	if err != nil {
		t.Fatalf("ReadMessage() unexpected error: %v", err)
	}
	if gotEnv.ID != env.ID {
		t.Fatalf("Envelope ID = %q, want %q", gotEnv.ID, env.ID)
	}
	if !bytes.Equal(gotBody, body) {
		t.Fatalf("Body mismatch: got %q want %q", gotBody, body)
	}
}
