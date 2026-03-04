// Package aimock provides a queue-based mock AI provider for unit tests.
// Tests pre-load canned responses; the mock pops one per SendMessage call.
package aimock

import (
	"context"
	"fmt"
	"testing"

	"bob/internal/ai"
)

// Call records one invocation of SendMessage.
type Call struct {
	UserPrompt  string
	Personality string
	ConvID      *string
}

// MockProvider is a deterministic, queue-based ai.Provider.
// Responses are returned in the order they were enqueued.
type MockProvider struct {
	queue []*ai.Response
	calls []Call
}

// New returns an empty MockProvider ready for use.
func New() *MockProvider {
	return &MockProvider{}
}

// QueueResponse enqueues a canned response built from data.
// The response uses dummy metadata (convID="mock-conv", responseID="mock-resp").
func (m *MockProvider) QueueResponse(data map[string]any) {
	m.queue = append(m.queue, ai.NewResponse(data, "mock-conv", "mock-resp", "mock-model", "stop", 0))
}

// Install wires m as the global AI client and registers a Cleanup to restore nil.
func Install(t *testing.T, m *MockProvider) {
	t.Helper()
	ai.SetGlobalAIClient(ai.NewAIClient(m))
	t.Cleanup(func() { ai.SetGlobalAIClient(nil) })
}

// SendMessage pops the next queued response. Returns an error when the queue is empty.
func (m *MockProvider) SendMessage(
	_ context.Context,
	conversationID *string,
	userPrompt string,
	personality string,
	_ *ai.SchemaBuilder,
	_ ...ai.Option,
) (*ai.Response, error) {
	m.calls = append(m.calls, Call{
		UserPrompt:  userPrompt,
		Personality: personality,
		ConvID:      conversationID,
	})
	if len(m.queue) == 0 {
		return nil, fmt.Errorf("aimock: no more queued responses (call #%d)", len(m.calls))
	}
	resp := m.queue[0]
	m.queue = m.queue[1:]
	return resp, nil
}

// Connect is a no-op — the mock needs no API key.
func (m *MockProvider) Connect(_ string) error { return nil }

// Close is a no-op.
func (m *MockProvider) Close() error { return nil }

// Calls returns a snapshot of all recorded SendMessage invocations.
func (m *MockProvider) Calls() []Call { return m.calls }

// CallCount returns the number of SendMessage calls made so far.
func (m *MockProvider) CallCount() int { return len(m.calls) }
