# GO-002: Mock OpenAI Layer & Slack-to-OpenAI Pass-through - Implementation Plan

## Overview

Create two minimal cmd-driven scripts to validate OpenAI SDK integration and Slack-to-OpenAI data flow. Both scripts are standalone tests with complete implementation already specified in the ticket.

## Current State Analysis

- New Go project structure exists
- Dependencies need to be installed: `go-openai`, `slack-go/slack`, `godotenv`
- Two cmd subdirectories need to be created: `cmd/openai-test/` and `cmd/slack-openai-pass-through/`
- Environment configuration needed (`.env.dist` and `.env`)

## Desired End State

Two working cmd scripts:
1. `go run cmd/openai-test/main.go` - Sends hardcoded prompt to OpenAI, displays response
2. `go run cmd/slack-openai-pass-through/main.go` - Receives Slack DMs, passes to OpenAI, responds back

**Verification**: Both scripts run successfully with proper environment variables and produce expected output/behavior.

## What We're NOT Doing

- No database integration
- No session management
- No conversation history
- No threading support
- No channel mentions (DMs only)
- No complex error recovery
- No production deployment

## Implementation Approach

Both scripts are completely independent with zero shared code. Implement them in parallel for efficiency, then test each independently.

## Execution Strategy

### Parallel Execution with Sequential Prerequisites

**Sequential Prerequisites**:
- Phase 1: Project setup (dependencies, directory structure, environment config)

**Parallel Phase Group**:

**Group A** (can run in parallel after Phase 1):
- Phase 2a: Implement OpenAI standalone test script
  - Modifies: `cmd/openai-test/main.go` (new file)
  - Tests: OpenAI SDK integration only

- Phase 2b: Implement Slack-to-OpenAI pass-through script
  - Modifies: `cmd/slack-openai-pass-through/main.go` (new file)
  - Tests: Slack + OpenAI integration

**Rationale for parallelization**: These are completely independent test scripts in different directories with no shared code, dependencies between them, or integration points. Each can be implemented and tested standalone. Parallel execution significantly reduces implementation time.

**Sequential Completion**:
- Phase 3: Manual end-to-end testing and documentation verification

---

## Phase 1: Project Setup

**Execution**: Sequential (prerequisite for all other phases)

### Overview
Install dependencies, create directory structure, and configure environment variables.

### Changes Required:

#### 1. Install Go Dependencies
**Commands**:
```bash
go get github.com/sashabaranov/go-openai
go get github.com/slack-go/slack
go get github.com/joho/godotenv
```

#### 2. Create Directory Structure
**Directories to create**:
```bash
mkdir -p cmd/openai-test
mkdir -p cmd/slack-openai-pass-through
```

#### 3. Environment Configuration
**File**: `.env.dist` (template)
**Create new file**:
```bash
# OpenAI API Key (required for both scripts)
OPENAI_API_KEY=sk-your-key-here

# Slack Bot Token (required for Script 2)
SLACK_BOT_TOKEN=xoxb-your-token-here

# Slack App Token (required for Script 2)
SLACK_APP_TOKEN=xapp-your-token-here
```

**File**: `.env` (actual secrets - gitignored)
**Action**: User must copy `.env.dist` to `.env` and fill in real values

### Success Criteria:

#### Automated Verification:
- [x] Dependencies installed: `go mod tidy` runs without errors
- [x] Directories exist: `ls cmd/openai-test cmd/slack-openai-pass-through`
- [x] `.env.dist` file exists with template variables

#### Manual Verification:
- [ ] User has created `.env` with real API keys
- [ ] `OPENAI_API_KEY` is valid and set in `.env`
- [ ] `SLACK_BOT_TOKEN` and `SLACK_APP_TOKEN` are valid and set in `.env`

**Implementation Note**: After completing this phase and automated verification passes, ensure the user has configured `.env` with real credentials before proceeding to Phase 2.

---

## Phase 2a: OpenAI Standalone Test Script

**Execution**: Can run in parallel with Phase 2b

### Context for Subagent Execution

**Background**: Testing OpenAI SDK integration standalone before building the full Slack integration.

**Current State**:
- Go project initialized
- Dependencies installed (Phase 1)
- OpenAI SDK: `github.com/sashabaranov/go-openai`

**This Phase's Scope**:
- **What**: Create standalone script that tests OpenAI Chat Completions API
- **Where**: `cmd/openai-test/main.go` (new file)
- **How**: Use OpenAI SDK to send hardcoded prompt, display response and token usage

**Dependencies**:
- **Requires completion of**: Phase 1 (dependencies and directories must exist)
- **Must not conflict with**: Phase 2b (works on different file)
- **Integrates with**: None - completely standalone test script

**Files Modified by This Phase**:
- `cmd/openai-test/main.go` (create new)

**Files Modified by Other Parallel Phases** (avoid these):
- Phase 2b modifies: `cmd/slack-openai-pass-through/main.go` (no conflict)

### Overview
Create a simple Go program that tests OpenAI API with a hardcoded prompt, validates the SDK works, and displays the response with token usage.

### Changes Required:

#### 1. OpenAI Test Script
**File**: `cmd/openai-test/main.go` (create new)
**Implementation**: Full code provided in ticket lines 36-89

Key elements:
- Read `OPENAI_API_KEY` from environment
- Create OpenAI client
- Send chat completion request with:
  - Model: `GPT4oMini`
  - System message: "You are a helpful assistant."
  - User message: "Say hello in a creative way!"
  - Temperature: 0.7, MaxTokens: 100
- Display response and token usage
- Log each step clearly

### Success Criteria:

#### Automated Verification:
- [x] Script compiles: `go build -o /tmp/openai-test cmd/openai-test/main.go`
- [x] No compilation errors
- [x] Script exits with error if `OPENAI_API_KEY` is not set

#### Manual Verification:
- [ ] Run: `go run cmd/openai-test/main.go` with valid API key
- [ ] Output shows: "Sending request to OpenAI..."
- [ ] Output shows: Creative hello message from GPT
- [ ] Output shows: Token usage count
- [ ] No errors or warnings in output

**Implementation Note**: This phase can be implemented independently. After automated verification passes, test manually with real OpenAI API key.

---

## Phase 2b: Slack-to-OpenAI Pass-through Script

**Execution**: Can run in parallel with Phase 2a

### Context for Subagent Execution

**Background**: Testing the full data flow: Slack DM → OpenAI → Slack response.

**Current State**:
- Go project initialized
- Dependencies installed (Phase 1)
- Slack SDK: `github.com/slack-go/slack` with socketmode
- OpenAI SDK: `github.com/sashabaranov/go-openai`

**This Phase's Scope**:
- **What**: Create script that connects Slack to OpenAI for DM pass-through
- **Where**: `cmd/slack-openai-pass-through/main.go` (new file)
- **How**:
  - Use Slack Socket Mode to receive DM events
  - Pass user message to OpenAI Chat Completions
  - Send AI response back to Slack DM
  - Log each step of the flow

**Dependencies**:
- **Requires completion of**: Phase 1 (dependencies and directories must exist)
- **Must not conflict with**: Phase 2a (works on different file)
- **Integrates with**: None - completely standalone test script

**Files Modified by This Phase**:
- `cmd/slack-openai-pass-through/main.go` (create new)

**Files Modified by Other Parallel Phases** (avoid these):
- Phase 2a modifies: `cmd/openai-test/main.go` (no conflict)

### Overview
Create a Socket Mode bot that receives Slack DM messages, sends them to OpenAI, and posts AI responses back to Slack. Keep it minimal: no threading, no channels, DMs only.

### Changes Required:

#### 1. Slack-to-OpenAI Pass-through Script
**File**: `cmd/slack-openai-pass-through/main.go` (create new)
**Implementation**: Full code provided in ticket lines 106-232

Key elements:
- Initialize both OpenAI and Slack clients
- Required env vars: `OPENAI_API_KEY`, `SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN`
- Socket Mode event loop
- Handle only `MessageEvent` with `ChannelType == "im"` (DMs)
- Filter out bot messages (`BotID == ""`)
- Send message to OpenAI with system prompt: "You are Bob, a helpful assistant. Be concise and friendly."
- Post AI response back to same Slack channel
- Clear logging: 📨 Received → 🤔 Asking OpenAI → 🤖 Response → ✅ Sent
- Error handling at each layer

### Success Criteria:

#### Automated Verification:
- [x] Script compiles: `go build -o /tmp/slack-pass-through cmd/slack-openai-pass-through/main.go`
- [x] No compilation errors
- [x] Script exits with error if any required env var is missing

#### Manual Verification:
- [ ] Run: `go run cmd/slack-openai-pass-through/main.go` with valid tokens
- [ ] Bot connects successfully (logs show "Slack-to-OpenAI pass-through bot starting...")
- [ ] Send DM to bot in Slack
- [ ] Logs show: 📨 Received message text
- [ ] Logs show: 🤔 Asking OpenAI...
- [ ] Logs show: 🤖 OpenAI responded with text
- [ ] Logs show: ✅ Response sent to Slack
- [ ] Bot responds in Slack DM with AI-generated message
- [ ] Test edge cases: long message, quick successive messages, invalid API key handling

**Implementation Note**: This phase can be implemented independently. After automated verification passes, test manually with real Slack workspace and OpenAI API key.

---

## Phase 3: Integration Verification and Documentation

**Execution**: Sequential (after both Phase 2a and 2b complete)

### Overview
Verify both scripts work as expected and documentation is accurate.

### Verification Tasks:

#### 1. Script 1 Testing
- Run `go run cmd/openai-test/main.go`
- Verify OpenAI responds with creative greeting
- Verify token usage is displayed
- Test error case: run without `OPENAI_API_KEY` set

#### 2. Script 2 Testing
- Run `go run cmd/slack-openai-pass-through/main.go`
- Send various test messages:
  - Simple question: "What's 2+2?"
  - Complex query: "Explain quantum computing in simple terms"
  - Edge case: Very long message (500+ words)
- Verify all responses come back correctly
- Verify logs show complete flow for each message
- Test error cases: invalid tokens, network issues

#### 3. Documentation Verification
- Verify README or ticket has clear setup instructions
- Verify `.env.dist` has all required variables
- Verify environment variable names match code

### Success Criteria:

#### Automated Verification:
- [x] Both scripts compile without errors
- [x] `go mod tidy` shows no missing dependencies
- [x] Both scripts pass `go vet ./...`

#### Manual Verification:
- [ ] Script 1 successfully gets response from OpenAI
- [ ] Script 2 successfully passes messages: Slack → OpenAI → Slack
- [ ] All logging is clear and shows each step
- [ ] Error handling works (tested with invalid credentials)
- [ ] Documentation is complete and accurate
- [ ] Both scripts are production-ready for their intended test purpose

**Implementation Note**: This is the final validation phase. All manual testing must pass before marking this ticket complete.

---

## Testing Strategy

### Script 1 (openai-test)
**Unit Test**: Compilation test via `go build`
**Integration Test**: Manual run with real API key
**Key Edge Cases**:
- Missing `OPENAI_API_KEY` env var (should exit with error)
- Invalid API key (should show API error)
- Network timeout (should show timeout error)

### Script 2 (slack-openai-pass-through)
**Unit Test**: Compilation test via `go build`
**Integration Test**: Manual DM exchange via Slack
**Key Edge Cases**:
- Missing any required env var (should exit with error)
- Invalid tokens (should show connection error)
- Long messages (>500 words)
- Rapid successive messages
- Bot receives message from another bot (should ignore)
- Message in channel instead of DM (should ignore)

### Manual Testing Steps:
1. Test Script 1: `go run cmd/openai-test/main.go` → verify response
2. Test Script 2: Run bot, send DM → verify AI response
3. Test edge cases: long messages, rapid fire, error conditions
4. Verify logs are clear and complete for debugging

## Performance Considerations

- Both scripts are test/validation tools, not production systems
- No performance optimization needed
- Token limits set to reasonable values (100 for Script 1, 500 for Script 2)
- No rate limiting implemented (not needed for manual testing)

## Migration Notes

N/A - These are new standalone scripts with no migration needed.

## References

- Original ticket: `thoughts/shared/tickets/GO-002-mock-openai-pass-through/ticket.md`
- OpenAI Go SDK: https://github.com/sashabaranov/go-openai
- Slack Go SDK: https://github.com/slack-go/slack
- OpenAI Chat Completions API: https://platform.openai.com/docs/api-reference/chat
