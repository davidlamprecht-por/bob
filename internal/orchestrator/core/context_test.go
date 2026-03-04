package core

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPushWorkflowHistory_TrimsToMax(t *testing.T) {
	ctx := NewConversationContext()
	for i := 0; i < maxWorkflowHistory+2; i++ {
		ctx.PushWorkflowHistory(&WorkflowHistoryEntry{
			WorkflowName: "wf",
			Summary:      "s",
			CompletedAt:  time.Now(),
		})
	}
	if got := len(ctx.GetWorkflowHistory()); got != maxWorkflowHistory {
		t.Errorf("expected history len=%d after overflow, got %d", maxWorkflowHistory, got)
	}
}

func TestPushWorkflowHistory_OrderPreserved(t *testing.T) {
	ctx := NewConversationContext()
	names := []string{"alpha", "beta", "gamma"}
	for _, n := range names {
		ctx.PushWorkflowHistory(&WorkflowHistoryEntry{WorkflowName: n})
	}
	history := ctx.GetWorkflowHistory()
	if len(history) != len(names) {
		t.Fatalf("expected %d entries, got %d", len(names), len(history))
	}
	for i, want := range names {
		if got := history[i].WorkflowName; got != want {
			t.Errorf("history[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestPendingClarification_SetGet(t *testing.T) {
	ctx := NewConversationContext()
	if ctx.GetPendingClarification() {
		t.Error("expected initial pendingClarification=false")
	}
	ctx.SetPendingClarification(true)
	if !ctx.GetPendingClarification() {
		t.Error("expected pendingClarification=true after set")
	}
	ctx.SetPendingClarification(false)
	if ctx.GetPendingClarification() {
		t.Error("expected pendingClarification=false after reset")
	}
}

func TestWorkflowHistoryJSON_RoundTrip(t *testing.T) {
	ctx := NewConversationContext()
	ts := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	ctx.PushWorkflowHistory(&WorkflowHistoryEntry{
		WorkflowName:   "testAI",
		TriggerMessage: "hello",
		Summary:        "user tested the AI",
		CompletedAt:    ts,
	})

	b, err := json.Marshal(ctx.GetWorkflowHistory())
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var out []*WorkflowHistoryEntry
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 entry after round-trip, got %d", len(out))
	}
	e := out[0]
	if e.WorkflowName != "testAI" {
		t.Errorf("WorkflowName: got %q, want %q", e.WorkflowName, "testAI")
	}
	if e.TriggerMessage != "hello" {
		t.Errorf("TriggerMessage: got %q, want %q", e.TriggerMessage, "hello")
	}
	if e.Summary != "user tested the AI" {
		t.Errorf("Summary: got %q, want %q", e.Summary, "user tested the AI")
	}
	if !e.CompletedAt.Equal(ts) {
		t.Errorf("CompletedAt: got %v, want %v", e.CompletedAt, ts)
	}
}
