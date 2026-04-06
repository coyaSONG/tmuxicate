package session

import (
	"strings"
	"testing"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func TestReviewRespondRecordsOutcome(t *testing.T) {
	t.Parallel()

	fixture := seedPendingReviewFixture(t)

	store := mailbox.NewStore(fixture.cfg.Session.StateDir)
	responseID, err := ReviewRespond(
		fixture.cfg.Session.StateDir,
		store,
		"reviewer",
		fixture.handoff.ReviewMessageID,
		protocol.ReviewOutcomeApproved,
		[]byte("approved with no changes\n"),
	)
	if err != nil {
		t.Fatalf("review respond: %v", err)
	}
	if responseID == "" {
		t.Fatalf("response id should not be empty")
	}

	updated, err := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir).ReadReviewHandoff(fixture.run.RunID, fixture.sourceTask.TaskID)
	if err != nil {
		t.Fatalf("read updated review handoff: %v", err)
	}
	if updated.Status != protocol.ReviewHandoffStatusResponded {
		t.Fatalf("handoff status = %q, want %q", updated.Status, protocol.ReviewHandoffStatusResponded)
	}
	if updated.ResponseMessageID == "" {
		t.Fatalf("response message id should not be empty")
	}
	if updated.Outcome != protocol.ReviewOutcomeApproved {
		t.Fatalf("handoff outcome = %q, want %q", updated.Outcome, protocol.ReviewOutcomeApproved)
	}
	if updated.RespondedAt == nil {
		t.Fatalf("responded_at should not be nil")
	}

	reviewReceipt, err := store.ReadReceipt("reviewer", fixture.handoff.ReviewMessageID)
	if err != nil {
		t.Fatalf("read review receipt: %v", err)
	}
	if reviewReceipt.FolderState != protocol.FolderStateDone {
		t.Fatalf("review receipt state = %q, want %q", reviewReceipt.FolderState, protocol.FolderStateDone)
	}
}

func TestReviewRespondRejectsUnlinkedOrWrongOwnerResponse(t *testing.T) {
	t.Parallel()

	t.Run("wrong owner", func(t *testing.T) {
		t.Parallel()

		fixture := seedPendingReviewFixture(t)
		store := mailbox.NewStore(fixture.cfg.Session.StateDir)

		_, err := ReviewRespond(
			fixture.cfg.Session.StateDir,
			store,
			"backend-high",
			fixture.handoff.ReviewMessageID,
			protocol.ReviewOutcomeChangesRequested,
			[]byte("requesting changes\n"),
		)
		if err == nil {
			t.Fatalf("expected wrong owner response to fail")
		}
	})

	t.Run("unlinked review request", func(t *testing.T) {
		t.Parallel()

		fixture := seedReviewHandoffFixture(t)
		if _, err := ReadMsg(fixture.cfg.Session.StateDir, "backend-high", fixture.sourceTask.MessageID); err != nil {
			t.Fatalf("activate source task: %v", err)
		}

		store := mailbox.NewStore(fixture.cfg.Session.StateDir)
		reviewTask, _, err := RouteChildTask(fixture.cfg, store, protocol.RouteChildTaskRequest{
			RunID:          fixture.run.RunID,
			TaskClass:      protocol.TaskClassReview,
			Domains:        []string{"protocol", "session"},
			Goal:           "Unlinked review request for negative coverage",
			ExpectedOutput: "Should be rejected by review respond",
			ReviewRequired: false,
		})
		if err != nil {
			t.Fatalf("route unlinked review task: %v", err)
		}
		if _, err := ReadMsg(fixture.cfg.Session.StateDir, "reviewer", reviewTask.MessageID); err != nil {
			t.Fatalf("activate unlinked review task: %v", err)
		}

		_, err = ReviewRespond(
			fixture.cfg.Session.StateDir,
			store,
			"reviewer",
			reviewTask.MessageID,
			protocol.ReviewOutcomeApproved,
			[]byte("approved\n"),
		)
		if err == nil {
			t.Fatalf("expected unlinked review response to fail")
		}
		if !strings.Contains(err.Error(), "review handoff") {
			t.Fatalf("expected handoff lookup failure, got %v", err)
		}
	})
}

type pendingReviewFixture struct {
	reviewHandoffFixture
	handoff *protocol.ReviewHandoff
}

func seedPendingReviewFixture(t *testing.T) pendingReviewFixture {
	t.Helper()

	fixture := seedReviewHandoffFixture(t)
	if _, err := ReadMsg(fixture.cfg.Session.StateDir, "backend-high", fixture.sourceTask.MessageID); err != nil {
		t.Fatalf("activate source task: %v", err)
	}
	if err := TaskDone(fixture.cfg.Session.StateDir, "backend-high", fixture.sourceTask.MessageID, "implementation complete"); err != nil {
		t.Fatalf("task done: %v", err)
	}

	handoff, err := mailbox.NewCoordinatorStore(fixture.cfg.Session.StateDir).ReadReviewHandoff(fixture.run.RunID, fixture.sourceTask.TaskID)
	if err != nil {
		t.Fatalf("read review handoff: %v", err)
	}
	if _, err := ReadMsg(fixture.cfg.Session.StateDir, "reviewer", handoff.ReviewMessageID); err != nil {
		t.Fatalf("activate review request: %v", err)
	}

	return pendingReviewFixture{
		reviewHandoffFixture: fixture,
		handoff:              handoff,
	}
}
