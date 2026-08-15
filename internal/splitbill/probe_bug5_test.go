package splitbill

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestBalanceConcurrentSafety verifies that calling Balance concurrently
// with AddBill does not produce inconsistent results or panic.
// This test must be run with -race flag: go test -race -run TestBalanceConcurrentSafety
func TestBalanceConcurrentSafety(t *testing.T) {
	store := NewStore()
	id, err := store.CreateGroup([]string{"alice", "bob"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	// Pre-add a bill so Balance has data to read
	_, err = store.AddBill(id, "alice", 1000, ModeEqual, []ParticipantInput{
		{Name: "alice"}, {Name: "bob"},
	})
	if err != nil {
		t.Fatalf("AddBill: %v", err)
	}

	var wg sync.WaitGroup
	var panicked atomic.Int64

	// Run Balance and AddBill concurrently many times
	for i := 0; i < 200; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Add(1)
				}
			}()
			bal, err := store.Balance(id)
			if err != nil {
				return
			}
			// Check consistency: alice + bob net should sum to 0
			sum := bal["alice"] + bal["bob"]
			if sum != 0 {
				panicked.Add(1)
			}
		}()
		go func() {
			defer wg.Done()
			store.AddBill(id, "bob", 100, ModeEqual, []ParticipantInput{
				{Name: "alice"}, {Name: "bob"},
			})
		}()
	}
	wg.Wait()

	if panicked.Load() > 0 {
		t.Fatalf("concurrent access caused %d panics or inconsistencies", panicked.Load())
	}
}
