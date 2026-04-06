package mailbox

import (
	"os"
	"testing"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestCoordinatorStoreBlockerCaseCRUD(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	store := NewCoordinatorStore(stateDir)
	blocker := testBlockerCase()

	if err := store.CreateBlockerCase(blocker); err != nil {
		t.Fatalf("CreateBlockerCase() unexpected error: %v", err)
	}

	if _, err := os.Stat(RunBlockerCasePath(stateDir, blocker.RunID, blocker.SourceTaskID)); err != nil {
		t.Fatalf("blocker case path missing: %v", err)
	}

	got, err := store.ReadBlockerCase(blocker.RunID, blocker.SourceTaskID)
	if err != nil {
		t.Fatalf("ReadBlockerCase() unexpected error: %v", err)
	}
	if got.SourceTaskID != blocker.SourceTaskID {
		t.Fatalf("SourceTaskID = %q, want %q", got.SourceTaskID, blocker.SourceTaskID)
	}

	if err := store.UpdateBlockerCase(blocker.RunID, blocker.SourceTaskID, func(existing *protocol.BlockerCase) error {
		now := time.Now().UTC()
		existing.Status = protocol.BlockerStatusEscalated
		existing.SelectedAction = protocol.BlockerActionEscalate
		existing.RecommendedAction = &protocol.RecommendedAction{
			Kind: protocol.BlockerResolutionActionManualReroute,
			Note: "reroute to a different owner",
		}
		existing.EscalatedAt = &now
		existing.UpdatedAt = now

		return nil
	}); err != nil {
		t.Fatalf("UpdateBlockerCase() unexpected error: %v", err)
	}

	updated, err := store.ReadBlockerCase(blocker.RunID, blocker.SourceTaskID)
	if err != nil {
		t.Fatalf("ReadBlockerCase() after update unexpected error: %v", err)
	}
	if updated.Status != protocol.BlockerStatusEscalated {
		t.Fatalf("Status = %q, want %q", updated.Status, protocol.BlockerStatusEscalated)
	}
	if updated.RecommendedAction == nil || updated.RecommendedAction.Kind != protocol.BlockerResolutionActionManualReroute {
		t.Fatalf("RecommendedAction = %#v, want manual_reroute", updated.RecommendedAction)
	}
}

func TestCoordinatorStoreFindBlockerCaseByCurrentTaskID(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	store := NewCoordinatorStore(stateDir)
	blocker := testBlockerCase()

	if err := store.CreateBlockerCase(blocker); err != nil {
		t.Fatalf("CreateBlockerCase() unexpected error: %v", err)
	}

	if err := os.MkdirAll(RunBlockersDir(stateDir, blocker.RunID), 0o755); err != nil {
		t.Fatalf("MkdirAll() unexpected error: %v", err)
	}
	if err := os.WriteFile(RunBlockersDir(stateDir, blocker.RunID)+"/README.txt", []byte("ignore me\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() unexpected error: %v", err)
	}

	found, err := store.FindBlockerCaseByCurrentTaskID(blocker.RunID, blocker.CurrentTaskID)
	if err != nil {
		t.Fatalf("FindBlockerCaseByCurrentTaskID() unexpected error: %v", err)
	}
	if found.SourceTaskID != blocker.SourceTaskID {
		t.Fatalf("SourceTaskID = %q, want %q", found.SourceTaskID, blocker.SourceTaskID)
	}

	reroutedTaskID := protocol.NewTaskID(33)
	reroutedMessageID := protocol.NewMessageID(34)
	if err := store.UpdateBlockerCase(blocker.RunID, blocker.SourceTaskID, func(existing *protocol.BlockerCase) error {
		existing.CurrentTaskID = reroutedTaskID
		existing.CurrentMessageID = reroutedMessageID
		existing.CurrentOwner = protocol.AgentName("backend-rerouted")
		existing.RerouteCount = 1
		existing.UpdatedAt = time.Now().UTC()
		return nil
	}); err != nil {
		t.Fatalf("UpdateBlockerCase() unexpected error: %v", err)
	}

	rerouted, err := store.FindBlockerCaseByCurrentTaskID(blocker.RunID, reroutedTaskID)
	if err != nil {
		t.Fatalf("FindBlockerCaseByCurrentTaskID() after reroute unexpected error: %v", err)
	}
	if rerouted.SourceTaskID != blocker.SourceTaskID {
		t.Fatalf("SourceTaskID after reroute = %q, want %q", rerouted.SourceTaskID, blocker.SourceTaskID)
	}
	if _, err := os.Stat(RunBlockerCasePath(stateDir, blocker.RunID, blocker.SourceTaskID)); err != nil {
		t.Fatalf("source-keyed blocker case path missing after reroute: %v", err)
	}
}

func testBlockerCase() *protocol.BlockerCase {
	now := time.Now().UTC()

	return &protocol.BlockerCase{
		RunID:            protocol.NewRunID(1),
		SourceTaskID:     protocol.NewTaskID(1),
		SourceMessageID:  protocol.NewMessageID(1),
		SourceOwner:      protocol.AgentName("coordinator"),
		CurrentTaskID:    protocol.NewTaskID(2),
		CurrentMessageID: protocol.NewMessageID(2),
		CurrentOwner:     protocol.AgentName("backend"),
		DeclaredState:    "block",
		BlockKind:        protocol.BlockKindRerouteNeeded,
		Reason:           "needs reroute",
		SelectedAction:   protocol.BlockerActionReroute,
		Status:           protocol.BlockerStatusActive,
		RerouteCount:     0,
		MaxReroutes:      1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}
