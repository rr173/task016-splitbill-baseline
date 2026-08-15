package splitbill

import "testing"

// TestCreateGroupMembersIsolation verifies that modifying the original
// members slice after creating a group does not affect the group's state.
func TestCreateGroupMembersIsolation(t *testing.T) {
	store := NewStore()
	members := []string{"alice", "bob", "carol"}
	id, err := store.CreateGroup(members)
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	// Mutate the original slice after group creation
	members[0] = "MUTATED"

	// The group should still have "alice" as first member
	bal, err := store.Balance(id)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if _, ok := bal["alice"]; !ok {
		t.Fatal("group members corrupted: alice missing after external slice mutation")
	}
	if _, ok := bal["MUTATED"]; ok {
		t.Fatal("group members corrupted: external mutation leaked into group state")
	}
}
