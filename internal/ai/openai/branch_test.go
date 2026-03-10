package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"bob/internal/ai"

	openailib "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// ---------------------------------------------------------------------------
// Mock server helpers
// ---------------------------------------------------------------------------

type branchMockServer struct {
	*httptest.Server
	requests []map[string]any
	callNum  atomic.Int64
}

// newBranchMockServer spins up an httptest.Server that handles POST /responses.
// Each call gets a unique resp_mock_N ID so callers can distinguish responses.
func newBranchMockServer(t *testing.T) *branchMockServer {
	t.Helper()
	m := &branchMockServer{}
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		m.requests = append(m.requests, parsed)

		n := m.callNum.Add(1)
		respID := fmt.Sprintf("resp_mock_%d", n)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockRespJSON(respID, `{"result":"ok"}`))
	}))
	t.Cleanup(m.Close)
	return m
}

// setupClient points the package-level OpenAI client at the mock server and
// resets it (and the personality cache) when the test ends.
func setupClient(t *testing.T, server *branchMockServer) {
	t.Helper()
	mu.Lock()
	client = openailib.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
	)
	mu.Unlock()

	t.Cleanup(func() {
		mu.Lock()
		client = openailib.Client{}
		mu.Unlock()

		personalityMutex.Lock()
		lastPersonality = make(map[string]string)
		personalityMutex.Unlock()
	})
}

// mockRespJSON builds the minimal JSON the openai-go SDK needs to parse a
// successful Responses API reply.
func mockRespJSON(id, text string) map[string]any {
	return map[string]any{
		"id":     id,
		"object": "response",
		"model":  "gpt-4o-mini",
		"status": "completed",
		"output": []any{
			map[string]any{
				"id":     "msg_001",
				"type":   "message",
				"role":   "assistant",
				"status": "completed",
				"content": []any{
					map[string]any{
						"type": "output_text",
						"text": text,
					},
				},
			},
		},
		"usage": map[string]any{
			"input_tokens":  10,
			"output_tokens": 5,
			"total_tokens":  15,
		},
	}
}

func simpleSchema() *ai.SchemaBuilder {
	return ai.NewSchema().AddString("result", ai.Required())
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSendMessage_ConvID_UsesConversationField verifies that a normal
// conversation ID (conv_xxx) is sent as the `conversation` field — NOT as
// `previous_response_id`.
func TestSendMessage_ConvID_UsesConversationField(t *testing.T) {
	srv := newBranchMockServer(t)
	setupClient(t, srv)

	convID := "conv_main_test"
	_, err := SendMessage(context.Background(), &convID, "hello", "be helpful", simpleSchema())
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if len(srv.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(srv.requests))
	}
	body := srv.requests[0]

	if _, ok := body["previous_response_id"]; ok {
		t.Error("conv_ ID should NOT set previous_response_id")
	}
	if conv, ok := body["conversation"]; !ok || conv == nil {
		t.Error("conv_ ID should set the conversation field")
	}
}

// TestSendMessage_RespID_UsesPreviousResponseID verifies that a response-chain
// ID (resp_xxx) is sent as `previous_response_id` — NOT as `conversation`.
func TestSendMessage_RespID_UsesPreviousResponseID(t *testing.T) {
	srv := newBranchMockServer(t)
	setupClient(t, srv)

	respID := "resp_origin_001"
	_, err := SendMessage(context.Background(), &respID, "hello", "be helpful", simpleSchema())
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if len(srv.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(srv.requests))
	}
	body := srv.requests[0]

	prevID, ok := body["previous_response_id"]
	if !ok || prevID != respID {
		t.Errorf("expected previous_response_id %q, got %v", respID, prevID)
	}
	if _, ok := body["conversation"]; ok {
		t.Error("resp_ ID should NOT set the conversation field")
	}
}

// TestSendMessage_RespID_AdvancesConvID verifies that when SendMessage is
// called with a resp_xxx ID, the returned ConversationID is the NEW response
// ID (the branch tip), not the one we passed in.
func TestSendMessage_RespID_AdvancesConvID(t *testing.T) {
	srv := newBranchMockServer(t)
	setupClient(t, srv)

	origRespID := "resp_origin_002"
	resp, err := SendMessage(context.Background(), &origRespID, "hello", "be helpful", simpleSchema())
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// The mock returns resp_mock_1 as the ID; ConversationID must be that new ID.
	if resp.ConversationID == origRespID {
		t.Errorf("ConversationID should advance past origRespID on a resp_ chain, but got the same ID back: %q", resp.ConversationID)
	}
	if !strings.HasPrefix(resp.ConversationID, "resp_") {
		t.Errorf("ConversationID should be a resp_ ID, got %q", resp.ConversationID)
	}
	if resp.ConversationID != resp.ResponseID {
		t.Errorf("ConversationID (%q) should equal ResponseID (%q) on a resp_ chain", resp.ConversationID, resp.ResponseID)
	}
}

// TestSendBranchedMessage_SendsPreviousResponseID verifies sendBranchedMessage
// sends previous_response_id and returns resp.ID as ConversationID.
func TestSendBranchedMessage_SendsPreviousResponseID(t *testing.T) {
	srv := newBranchMockServer(t)
	setupClient(t, srv)

	previousRespID := "resp_thread_tip"
	resp, err := sendBranchedMessage(context.Background(), previousRespID, "branch query", "be concise", simpleSchema())
	if err != nil {
		t.Fatalf("sendBranchedMessage failed: %v", err)
	}

	if len(srv.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(srv.requests))
	}
	body := srv.requests[0]

	prevID, ok := body["previous_response_id"]
	if !ok || prevID != previousRespID {
		t.Errorf("expected previous_response_id %q, got %v", previousRespID, prevID)
	}

	// ConversationID must be the new resp ID, not the input previousRespID.
	if resp.ConversationID == previousRespID {
		t.Error("ConversationID should be the new branch resp ID, not the input previousRespID")
	}
	if !strings.HasPrefix(resp.ConversationID, "resp_") {
		t.Errorf("ConversationID should be a resp_ ID, got %q", resp.ConversationID)
	}
}

// TestProviderSendMessage_RoutesBranchOption verifies that provider.SendMessage
// with BranchFromResponse routes through sendBranchedMessage (uses
// previous_response_id) rather than the normal conversation path.
func TestProviderSendMessage_RoutesBranchOption(t *testing.T) {
	srv := newBranchMockServer(t)
	setupClient(t, srv)

	p := &provider{}
	originRespID := "resp_for_provider_test"
	_, err := p.SendMessage(
		context.Background(),
		nil, // conversationID — ignored when BranchOption is present
		"test prompt",
		"be helpful",
		simpleSchema(),
		ai.BranchFromResponse(originRespID),
	)
	if err != nil {
		t.Fatalf("provider.SendMessage failed: %v", err)
	}

	if len(srv.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(srv.requests))
	}
	body := srv.requests[0]

	prevID, ok := body["previous_response_id"]
	if !ok || prevID != originRespID {
		t.Errorf("expected previous_response_id %q, got %v", originRespID, prevID)
	}
	if _, ok := body["conversation"]; ok {
		t.Error("BranchOption should not set the conversation field")
	}
}

// TestBranchIsolation tests two key isolation properties:
//
// 1. Two branch calls from the same resp_origin each send `previous_response_id`
//    = resp_origin but receive independent new IDs back — their chains diverge
//    immediately and share nothing going forward.
//
// 2. Branch calls do NOT touch the personality cache entry for the original
//    conversation ID, so the parent conversation is completely unaffected.
func TestBranchIsolation(t *testing.T) {
	srv := newBranchMockServer(t)
	setupClient(t, srv)

	const (
		origConvID   = "conv_main_isolated"
		origRespTip  = "resp_thread_tip_isolated"
		personality  = "be very helpful"
	)

	// Simulate the original conv having a cached personality so we can verify
	// it is not disturbed by branch calls.
	personalityMutex.Lock()
	lastPersonality[origConvID] = personality
	personalityMutex.Unlock()

	// Branch A
	respA, err := sendBranchedMessage(context.Background(), origRespTip, "query A", personality, simpleSchema())
	if err != nil {
		t.Fatalf("branch A failed: %v", err)
	}

	// Branch B (same origin)
	respB, err := sendBranchedMessage(context.Background(), origRespTip, "query B", personality, simpleSchema())
	if err != nil {
		t.Fatalf("branch B failed: %v", err)
	}

	// Both branches used the same previous_response_id.
	for i, req := range srv.requests {
		prevID, ok := req["previous_response_id"]
		if !ok || prevID != origRespTip {
			t.Errorf("request %d: expected previous_response_id %q, got %v", i, origRespTip, prevID)
		}
	}

	// Each branch got its own unique response ID — the chains are independent.
	if respA.ConversationID == respB.ConversationID {
		t.Errorf("branches should receive independent IDs, but both got %q", respA.ConversationID)
	}

	// Neither branch ID is the origin ID.
	if respA.ConversationID == origRespTip || respB.ConversationID == origRespTip {
		t.Error("branch ConversationID should advance past the origin resp ID")
	}

	// The original conversation's personality cache entry is untouched.
	personalityMutex.RLock()
	cached := lastPersonality[origConvID]
	personalityMutex.RUnlock()
	if cached != personality {
		t.Errorf("branch calls must not alter original conv's personality cache; got %q, want %q", cached, personality)
	}

	// The origin resp tip itself must not have been added to the personality cache
	// (resp_ IDs always set Instructions unconditionally; shouldIncludePersonality
	// is never called for them).
	personalityMutex.RLock()
	_, inCache := lastPersonality[origRespTip]
	personalityMutex.RUnlock()
	if inCache {
		t.Error("resp_ IDs should not be stored in the personality cache")
	}
}
