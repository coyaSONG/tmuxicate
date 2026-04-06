package protocol

import (
	"strings"
	"testing"
	"time"
)

func TestNewMessageID(t *testing.T) {
	t.Parallel()

	got := NewMessageID(142)
	want := MessageID("msg_000000000142")
	if got != want {
		t.Fatalf("NewMessageID(142) = %q, want %q", got, want)
	}
}

func TestNewThreadID(t *testing.T) {
	t.Parallel()

	got := NewThreadID(19)
	want := ThreadID("thr_000000000019")
	if got != want {
		t.Fatalf("NewThreadID(19) = %q, want %q", got, want)
	}
}

func TestEnvelopeValidateValid(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	env := Envelope{
		Schema:     MessageSchemaV1,
		ID:         NewMessageID(142),
		Seq:        142,
		Session:    "dev",
		Thread:     NewThreadID(19),
		Kind:       KindReviewRequest,
		From:       AgentName("coordinator"),
		To:         []AgentName{AgentName("reviewer")},
		CreatedAt:  now,
		BodyFormat: BodyFormatMD,
		BodySHA256: "8a5d4d7c5bf3a1f4f54abf1b7f70d3f3d95c2f5f7e82f4c0f33a0a2ec8714abc",
		BodyBytes:  913,
		Priority:   PriorityHigh,
		Budget: &Budget{
			MaxTurns: 1,
			MaxLines: 40,
		},
		Attachments: []Attachment{
			{
				Path:      "artifacts/diff.patch",
				MediaType: "text/x-diff",
				SHA256:    "4e2b4f5a6874a1a26d3ce9fdb9f6d8bfa70cb737d8283e28c2a9338c40d0e734",
			},
		},
		Meta: map[string]string{"source": "tmuxicate-send"},
	}

	if err := env.Validate(); err != nil {
		t.Fatalf("Envelope.Validate() unexpected error: %v", err)
	}
}

func TestEnvelopeValidateInvalid(t *testing.T) {
	t.Parallel()

	env := Envelope{
		Schema:     MessageSchemaV1,
		ID:         NewMessageID(1),
		Seq:        1,
		Session:    "dev",
		Thread:     NewThreadID(1),
		Kind:       Kind("bogus"),
		From:       AgentName("coordinator"),
		To:         []AgentName{AgentName("reviewer")},
		CreatedAt:  time.Now().UTC(),
		BodyFormat: BodyFormatMD,
		BodySHA256: "not-a-sha",
		BodyBytes:  10,
	}

	if err := env.Validate(); err == nil {
		t.Fatal("Envelope.Validate() expected error, got nil")
	}
}

func TestReceiptValidateUnread(t *testing.T) {
	t.Parallel()

	r := Receipt{
		Schema:         ReceiptSchemaV1,
		MessageID:      NewMessageID(142),
		Seq:            142,
		Recipient:      AgentName("reviewer"),
		FolderState:    FolderStateUnread,
		Revision:       0,
		NotifyAttempts: 0,
	}

	if err := r.Validate(); err != nil {
		t.Fatalf("Receipt.Validate() unexpected error: %v", err)
	}
}

func TestReceiptValidateDoneRequiresDoneAt(t *testing.T) {
	t.Parallel()

	r := Receipt{
		Schema:         ReceiptSchemaV1,
		MessageID:      NewMessageID(142),
		Seq:            142,
		Recipient:      AgentName("reviewer"),
		FolderState:    FolderStateDone,
		Revision:       2,
		NotifyAttempts: 1,
	}

	if err := r.Validate(); err == nil {
		t.Fatal("Receipt.Validate() expected error for done receipt without done_at")
	}
}

func TestReceiptValidateStateTransitions(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	reviewer := AgentName("reviewer")

	active := Receipt{
		Schema:         ReceiptSchemaV1,
		MessageID:      NewMessageID(143),
		Seq:            143,
		Recipient:      reviewer,
		FolderState:    FolderStateActive,
		Revision:       1,
		NotifyAttempts: 1,
		AckedAt:        &now,
		ClaimedBy:      &reviewer,
		ClaimedAt:      &now,
	}
	if err := active.Validate(); err != nil {
		t.Fatalf("active receipt should validate: %v", err)
	}

	done := active
	done.FolderState = FolderStateDone
	done.DoneAt = &now
	done.Revision = 2
	if err := done.Validate(); err != nil {
		t.Fatalf("done receipt should validate: %v", err)
	}
}

func TestBlockerCaseValidateRequiresStructuredKinds(t *testing.T) {
	t.Parallel()

	t.Run("wait requires wait kind", func(t *testing.T) {
		t.Parallel()

		blocker := validBlockerCase()
		blocker.DeclaredState = "wait"
		blocker.WaitKind = ""
		blocker.BlockKind = ""

		err := blocker.Validate()
		if err == nil {
			t.Fatal("BlockerCase.Validate() expected error, got nil")
		}
		if got := err.Error(); got == "" || !containsAll(got, "wait_kind") {
			t.Fatalf("BlockerCase.Validate() error = %q, want wait_kind requirement", got)
		}
	})

	t.Run("block requires block kind and forbids wait kind", func(t *testing.T) {
		t.Parallel()

		blocker := validBlockerCase()
		blocker.DeclaredState = "block"
		blocker.BlockKind = ""
		blocker.WaitKind = WaitKindDependencyReply

		err := blocker.Validate()
		if err == nil {
			t.Fatal("BlockerCase.Validate() expected error, got nil")
		}
		if got := err.Error(); got == "" || (!containsAll(got, "block_kind") && !containsAll(got, "wait_kind")) {
			t.Fatalf("BlockerCase.Validate() error = %q, want structured kind failure", got)
		}
	})
}

func TestBlockerCaseValidateRequiresRecommendedActionForEscalation(t *testing.T) {
	t.Parallel()

	blocker := validBlockerCase()
	blocker.Status = BlockerStatusEscalated
	blocker.RecommendedAction = nil

	err := blocker.Validate()
	if err == nil {
		t.Fatal("BlockerCase.Validate() expected error, got nil")
	}
	if got := err.Error(); got == "" || !containsAll(got, "recommended_action") {
		t.Fatalf("BlockerCase.Validate() error = %q, want recommended_action failure", got)
	}
}

func TestBlockerResolutionActionValidate(t *testing.T) {
	t.Parallel()

	valid := []BlockerResolutionAction{
		BlockerResolutionActionManualReroute,
		BlockerResolutionActionClarify,
		BlockerResolutionActionDismiss,
	}

	for _, action := range valid {
		action := action
		t.Run(string(action), func(t *testing.T) {
			t.Parallel()

			if err := action.Validate(); err != nil {
				t.Fatalf("BlockerResolutionAction.Validate() unexpected error: %v", err)
			}
		})
	}

	invalid := BlockerResolutionAction("other")
	if err := invalid.Validate(); err == nil {
		t.Fatal("BlockerResolutionAction.Validate() expected error, got nil")
	}
}

func validBlockerCase() *BlockerCase {
	now := time.Now().UTC()

	return &BlockerCase{
		RunID:           NewRunID(1),
		SourceTaskID:    NewTaskID(1),
		SourceMessageID: NewMessageID(1),
		SourceOwner:     AgentName("coordinator"),
		CurrentTaskID:   NewTaskID(2),
		CurrentMessageID: NewMessageID(2),
		CurrentOwner:    AgentName("backend"),
		DeclaredState:   "block",
		BlockKind:       BlockKindRerouteNeeded,
		Reason:          "needs explicit next step",
		SelectedAction:  BlockerActionReroute,
		Status:          BlockerStatusActive,
		RerouteCount:    0,
		MaxReroutes:     1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func containsAll(got string, wantParts ...string) bool {
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			return false
		}
	}

	return true
}
