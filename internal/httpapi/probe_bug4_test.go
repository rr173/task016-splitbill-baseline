package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task016-splitbill/internal/splitbill"
)

// TestSettlementEmptyReturnsArray verifies that when a group has no bills
// (all balances are zero), the settlement endpoint returns an empty JSON
// array [] rather than null.
func TestSettlementEmptyReturnsArray(t *testing.T) {
	store := splitbill.NewStore()
	api := New(store)
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	// Create a group with no bills
	resp, err := http.Post(srv.URL+"/groups", "application/json",
		strings.NewReader(`{"members":["a","b"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		GroupID string `json:"group_id"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// Get settlement - should be empty array
	resp, err = http.Get(srv.URL + "/groups/" + created.GroupID + "/settlement")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var body map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	raw := string(body["transfers"])
	if raw == "null" {
		t.Fatal("settlement with no bills should return [] not null for transfers")
	}
	if raw != "[]" {
		t.Fatalf("expected empty array [], got %s", raw)
	}
}
