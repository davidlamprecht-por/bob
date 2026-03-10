package ai

import "testing"

func TestBranchFromResponse(t *testing.T) {
	opt := BranchFromResponse("resp_abc123")
	b, ok := opt.(BranchOption)
	if !ok {
		t.Fatal("BranchFromResponse should return a BranchOption")
	}
	if b.ResponseID != "resp_abc123" {
		t.Errorf("expected ResponseID %q, got %q", "resp_abc123", b.ResponseID)
	}
}

func TestBranchOption_Apply_IsNoop(t *testing.T) {
	b := BranchOption{ResponseID: "resp_test"}
	// Apply must not panic regardless of argument type
	b.Apply(nil)
	b.Apply("anything")
	b.Apply(42)
}

// Compile-time check: BranchOption satisfies Option.
var _ Option = BranchOption{}
