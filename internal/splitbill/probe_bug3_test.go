package splitbill

import "testing"

// TestSettleTieBreakAscending verifies that when two debtors have equal
// absolute debt, the one with the lexicographically smaller name is
// selected first for settlement.
func TestSettleTieBreakAscending(t *testing.T) {
	// a is owed 600, b and c each owe 300 (tied).
	// With ascending tie-break, b (smaller name) should settle first.
	balances := map[string]int64{
		"a": 600,
		"b": -300,
		"c": -300,
	}
	transfers := Settle(balances)
	if len(transfers) != 2 {
		t.Fatalf("expected 2 transfers, got %d: %+v", len(transfers), transfers)
	}
	if transfers[0].From != "b" {
		t.Fatalf("first transfer should be from b (ascending tie-break), got from=%q", transfers[0].From)
	}
	if transfers[1].From != "c" {
		t.Fatalf("second transfer should be from c, got from=%q", transfers[1].From)
	}
}
