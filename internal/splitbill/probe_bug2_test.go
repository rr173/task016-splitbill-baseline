package splitbill

import "testing"

// TestAddBillRejectsDuplicateParticipant verifies that a bill with the
// same participant listed twice is rejected to prevent double-counting.
func TestAddBillRejectsDuplicateParticipant(t *testing.T) {
	store := NewStore()
	id, err := store.CreateGroup([]string{"alice", "bob"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	_, err = store.AddBill(id, "alice", 1000, ModeEqual, []ParticipantInput{
		{Name: "alice"},
		{Name: "alice"}, // duplicate
	})
	if err == nil {
		t.Fatal("expected error for duplicate participant in same bill, got nil")
	}
}
