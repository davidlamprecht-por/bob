# AI Layer ↔ OpenAI Module Interface Examples

This document shows minimal examples of how the AI Layer communicates with the OpenAI module.

## Architecture Flow

```
Workflow → Action → Orchestrator → AI Layer → OpenAI Module → OpenAI API
```

The workflow never talks directly to OpenAI - it goes through Actions → Orchestrator → AI Layer.

---

## OpenAI Module Interface

### Available Functions

The AI Layer can call these functions from the OpenAI module:

```go
package openai

// Primary function for sending messages with structured output
func SendMessage(
    ctx context.Context,
    conversationID *string,        // nil = create new conversation
    userPrompt string,              // User's message
    personality string,             // System message (instructions)
    responseStruct interface{},     // Pointer to struct defining expected output
    opts ...Option,                 // Optional parameters
) (*Response, error)

// Helper to create conversation explicitly (usually not needed)
func CreateConversation(ctx context.Context) (string, error)
```

### Response Type

```go
type Response struct {
    Data           interface{}  // Parsed structured output (cast to your struct type)
    ConversationID string       // Conversation ID (new or existing)
    ResponseID     string       // OpenAI response ID for debugging
    TokensUsed     int          // Total tokens consumed
    Model          string       // Model used (e.g., "gpt-4o")
    FinishReason   string       // "stop", "length", etc.
}
```

### Options

```go
// Common options the AI Layer can pass
openai.WithModel("gpt-4o")                    // Override model
openai.WithTemperature(0.7)                   // Creativity level (0.0-2.0)
openai.WithMaxTokens(2000)                    // Max response tokens
openai.WithTopP(0.9)                          // Nucleus sampling
openai.WithFrequencyPenalty(0.5)              // Reduce repetition
openai.WithPresencePenalty(0.3)               // Encourage new topics
```

---

## Example 1: Simple Question-Answer (No Conversation State)

### What AI Layer Sends

```go
package ai

import (
    "context"
    "bob/internal/openai"
)

type SimpleQAResponse struct {
    Answer string `json:"answer" openai:"description=Direct answer to user question,required"`
}

func HandleSimpleQuestion(userQuestion string) (string, error) {
    // No conversation ID = ephemeral, one-off interaction
    response, err := openai.SendMessage(
        context.Background(),
        nil,  // nil = no conversation tracking
        userQuestion,
        "You are a helpful assistant. Answer concisely.",
        &SimpleQAResponse{},
        openai.WithModel("gpt-4o-mini"),
        openai.WithTemperature(0.3),
    )

    if err != nil {
        return "", err
    }

    // Cast Data to our struct type
    data := response.Data.(*SimpleQAResponse)
    return data.Answer, nil
}
```

### What AI Layer Receives

```go
&openai.Response{
    Data: &SimpleQAResponse{
        Answer: "The answer to your question is...",
    },
    ConversationID: "conv-abc123",  // Created but we don't use it
    ResponseID:     "resp-xyz789",
    TokensUsed:     45,
    Model:          "gpt-4o-mini",
    FinishReason:   "stop",
}
```

---

## Example 2: Workflow Conversation (Stateful, Multi-Turn)

### What AI Layer Sends

```go
package ai

type CreateTicketResponse struct {
    Response       string `json:"response" openai:"description=Message to send to user,required"`
    OverallSummary string `json:"overall_summary" openai:"description=Summary of entire conversation,required"`
    RecentSummary  string `json:"recent_summary" openai:"description=Summary of last exchange,required"`

    // Extracted data (optional fields)
    TicketTitle       string `json:"ticket_title,omitempty" openai:"description=Ticket title if mentioned"`
    TicketDescription string `json:"ticket_description,omitempty" openai:"description=Ticket description if mentioned"`
    TicketPriority    string `json:"ticket_priority,omitempty" openai:"description=Priority level,enum=low|medium|high|critical"`
    AssignedTeam      string `json:"assigned_team,omitempty" openai:"description=Team to assign ticket to"`

    // Workflow control
    IsComplete        bool     `json:"is_complete" openai:"description=Whether all required info has been collected,required"`
    NextQuestion      string   `json:"next_question,omitempty" openai:"description=Next question to ask user if not complete"`
    MissingFields     []string `json:"missing_fields,omitempty" openai:"description=List of fields still needed"`
}

func HandleWorkflowMessage(conversationID *string, userMessage string) (*CreateTicketResponse, string, error) {
    personality := `You are a helpful assistant guiding users through creating Azure DevOps tickets.

Your job:
1. Extract ticket information from conversation (title, description, priority, team)
2. Ask ONE clarifying question at a time if info is missing
3. Be concise and professional
4. Set is_complete=true when you have: title, description, priority, and team`

    response, err := openai.SendMessage(
        context.Background(),
        conversationID,  // Continues existing conversation or creates new one
        userMessage,
        personality,
        &CreateTicketResponse{},
        openai.WithModel("gpt-4o"),
        openai.WithTemperature(0.7),
        openai.WithMaxTokens(1000),
    )

    if err != nil {
        return nil, "", err
    }

    data := response.Data.(*CreateTicketResponse)
    return data, response.ConversationID, nil
}
```

### First Message - What AI Layer Receives

```go
&openai.Response{
    Data: &CreateTicketResponse{
        Response:       "I'll help you create a ticket. What issue are you experiencing?",
        OverallSummary: "User wants to create a ticket. Starting information gathering.",
        RecentSummary:  "Greeted user and asked for issue description.",

        TicketTitle:       "",      // Not provided yet
        TicketDescription: "",
        TicketPriority:    "",
        AssignedTeam:      "",

        IsComplete:    false,
        NextQuestion:  "What issue are you experiencing?",
        MissingFields: []string{"title", "description", "priority", "team"},
    },
    ConversationID: "conv-workflow-123",  // New conversation created
    TokensUsed:     120,
}
```

### Third Message - What AI Layer Receives

```go
// User has provided: title, description, priority
// Conversation continues with same ID

&openai.Response{
    Data: &CreateTicketResponse{
        Response:       "Which team should I assign this to? (Options: Platform, Backend, Frontend, DevOps)",
        OverallSummary: "User creating ticket for login issue. Has provided title, description (users can't login), and marked as high priority. Waiting for team assignment.",
        RecentSummary:  "Asked user which team to assign ticket to.",

        TicketTitle:       "Users cannot login to dashboard",
        TicketDescription: "Multiple users reporting 500 errors when attempting to login via OAuth",
        TicketPriority:    "high",
        AssignedTeam:      "",  // Still missing

        IsComplete:    false,
        NextQuestion:  "Which team should I assign this to?",
        MissingFields: []string{"team"},
    },
    ConversationID: "conv-workflow-123",  // Same conversation
    TokensUsed:     185,
}
```

### Final Message - What AI Layer Receives

```go
&openai.Response{
    Data: &CreateTicketResponse{
        Response:       "Perfect! I'll create the ticket now with the following details:\n\nTitle: Users cannot login to dashboard\nPriority: High\nTeam: Backend\n\nCreating ticket...",
        OverallSummary: "User created ticket for high-priority login issue. All information collected: title, description, priority (high), team (Backend). Ready to create ticket in Azure DevOps.",
        RecentSummary:  "User specified Backend team. All information collected.",

        TicketTitle:       "Users cannot login to dashboard",
        TicketDescription: "Multiple users reporting 500 errors when attempting to login via OAuth",
        TicketPriority:    "high",
        AssignedTeam:      "backend",

        IsComplete:    true,  // Workflow can now proceed to create ticket
        NextQuestion:  "",
        MissingFields: []string{},
    },
    ConversationID: "conv-workflow-123",
    TokensUsed:     210,
}
```

---

## Example 3: Intent Classification (Structured Enum Output)

### What AI Layer Sends

```go
package ai

type IntentClassification struct {
    IntentType   string  `json:"intent_type" openai:"description=Type of user intent,required,enum=new_workflow|continue_workflow|side_question|clarification"`
    Confidence   float64 `json:"confidence" openai:"description=Confidence score between 0 and 1,required,min=0,max=1"`
    WorkflowName string  `json:"workflow_name,omitempty" openai:"description=Which workflow to route to if new_workflow"`
    Reasoning    string  `json:"reasoning" openai:"description=Brief explanation of classification,required"`
}

func ClassifyIntent(userMessage string, recentContext string) (*IntentClassification, error) {
    personality := `You are an intent classifier for a workflow system.

Classify the user's message as one of:
- new_workflow: User wants to start a new task
- continue_workflow: User is responding to a question in current workflow
- side_question: User asking a question unrelated to current workflow
- clarification: User asking for clarification about current workflow

Consider the recent context.`

    prompt := fmt.Sprintf("Recent context:\n%s\n\nUser message: %s", recentContext, userMessage)

    response, err := openai.SendMessage(
        context.Background(),
        nil,  // No conversation needed for classification
        prompt,
        personality,
        &IntentClassification{},
        openai.WithModel("gpt-4o-mini"),  // Fast, cheap model
        openai.WithTemperature(0.2),      // Low temperature for consistent classification
    )

    if err != nil {
        return nil, err
    }

    return response.Data.(*IntentClassification), nil
}
```

### What AI Layer Receives

```go
&openai.Response{
    Data: &IntentClassification{
        IntentType:   "side_question",
        Confidence:   0.92,
        WorkflowName: "",
        Reasoning:    "User is asking about company policy while in the middle of creating a ticket, unrelated to ticket creation workflow.",
    },
    ConversationID: "conv-intent-789",
    TokensUsed:     65,
}
```

---

## Example 4: Data Extraction (Arrays and Nested Objects)

### What AI Layer Sends

```go
package ai

type TicketSearchResult struct {
    ID          int    `json:"id" openai:"description=Ticket ID number,required"`
    Title       string `json:"title" openai:"description=Ticket title,required"`
    Status      string `json:"status" openai:"description=Current status,required,enum=open|in_progress|resolved|closed"`
    Priority    string `json:"priority" openai:"description=Priority level,required,enum=low|medium|high|critical"`
    AssignedTo  string `json:"assigned_to,omitempty" openai:"description=Person assigned to ticket"`
    Relevance   float64 `json:"relevance" openai:"description=Relevance score 0-1,required,min=0,max=1"`
}

type SearchTicketsResponse struct {
    Response        string               `json:"response" openai:"description=Message to user about search results,required"`
    OverallSummary  string               `json:"overall_summary" openai:"description=Conversation summary,required"`
    TicketsFound    int                  `json:"tickets_found" openai:"description=Number of tickets found,required,min=0"`
    Tickets         []TicketSearchResult `json:"tickets" openai:"description=List of matching tickets,required"`
    SearchQuery     string               `json:"search_query" openai:"description=Interpreted search query,required"`
}

func SearchTickets(userQuery string, ticketData string) (*SearchTicketsResponse, error) {
    personality := `You are a ticket search assistant.

Analyze the user's query and the ticket data provided.
Extract relevant tickets and rank them by relevance.
Return the top 5 most relevant results.`

    prompt := fmt.Sprintf("User query: %s\n\nAvailable tickets:\n%s", userQuery, ticketData)

    response, err := openai.SendMessage(
        context.Background(),
        nil,
        prompt,
        personality,
        &SearchTicketsResponse{},
        openai.WithModel("gpt-4o"),
        openai.WithTemperature(0.3),
    )

    if err != nil {
        return nil, err
    }

    return response.Data.(*SearchTicketsResponse), nil
}
```

### What AI Layer Receives

```go
&openai.Response{
    Data: &SearchTicketsResponse{
        Response:       "I found 3 high-priority login-related tickets. The most recent is #1234 'OAuth timeout issues' assigned to Sarah.",
        OverallSummary: "User searched for high-priority login issues. Found 3 matching tickets.",
        TicketsFound:   3,
        Tickets: []TicketSearchResult{
            {
                ID:         1234,
                Title:      "OAuth timeout issues causing login failures",
                Status:     "in_progress",
                Priority:   "high",
                AssignedTo: "Sarah Johnson",
                Relevance:  0.95,
            },
            {
                ID:         1189,
                Title:      "Users unable to login via SSO",
                Status:     "open",
                Priority:   "high",
                AssignedTo: "",
                Relevance:  0.87,
            },
            {
                ID:         1156,
                Title:      "Login page returns 500 error intermittently",
                Status:     "resolved",
                Priority:   "high",
                AssignedTo: "Mike Chen",
                Relevance:  0.76,
            },
        },
        SearchQuery: "high priority login issues",
    },
    ConversationID: "conv-search-456",
    TokensUsed:     340,
}
```

---

## Example 5: Complex Nested Structures

### What AI Layer Sends

```go
package ai

type CodeReviewComment struct {
    LineNumber int    `json:"line_number" openai:"description=Line number in code,required,min=1"`
    Severity   string `json:"severity" openai:"description=Issue severity,required,enum=info|warning|error"`
    Category   string `json:"category" openai:"description=Type of issue,required,enum=style|security|performance|bug|best_practice"`
    Message    string `json:"message" openai:"description=Description of the issue,required"`
    Suggestion string `json:"suggestion,omitempty" openai:"description=Suggested fix"`
}

type FileReview struct {
    FilePath     string              `json:"file_path" openai:"description=Path to file,required"`
    OverallScore int                 `json:"overall_score" openai:"description=Code quality score 1-10,required,min=1,max=10"`
    Summary      string              `json:"summary" openai:"description=Brief summary of file quality,required"`
    Comments     []CodeReviewComment `json:"comments" openai:"description=List of review comments,required"`
}

type CodeReviewResponse struct {
    Response        string       `json:"response" openai:"description=Overall review message,required"`
    OverallSummary  string       `json:"overall_summary" openai:"description=Conversation summary,required"`
    FilesReviewed   int          `json:"files_reviewed" openai:"description=Number of files reviewed,required,min=0"`
    Files           []FileReview `json:"files" openai:"description=Detailed file reviews,required"`
    ApprovalStatus  string       `json:"approval_status" openai:"description=Review decision,required,enum=approved|changes_requested|needs_discussion"`
}

func ReviewCode(codeFiles map[string]string) (*CodeReviewResponse, error) {
    personality := `You are a code reviewer. Analyze code for:
- Security vulnerabilities
- Performance issues
- Best practice violations
- Style consistency
- Potential bugs

Be specific with line numbers and provide actionable suggestions.`

    // Build prompt with all files
    prompt := "Please review the following code files:\n\n"
    for path, content := range codeFiles {
        prompt += fmt.Sprintf("=== %s ===\n%s\n\n", path, content)
    }

    response, err := openai.SendMessage(
        context.Background(),
        nil,
        prompt,
        personality,
        &CodeReviewResponse{},
        openai.WithModel("gpt-4o"),
        openai.WithTemperature(0.4),
    )

    if err != nil {
        return nil, err
    }

    return response.Data.(*CodeReviewResponse), nil
}
```

### What AI Layer Receives

```go
&openai.Response{
    Data: &CodeReviewResponse{
        Response:       "I've reviewed 2 files. Found 1 security issue in auth.go and 3 style improvements in handler.go. Overall code quality is good but please address the SQL injection vulnerability.",
        OverallSummary: "Code review completed for authentication module. Found security vulnerability requiring immediate attention.",
        FilesReviewed:  2,
        Files: []FileReview{
            {
                FilePath:     "internal/auth/auth.go",
                OverallScore: 6,
                Summary:      "Critical SQL injection vulnerability found. Otherwise decent structure.",
                Comments: []CodeReviewComment{
                    {
                        LineNumber: 45,
                        Severity:   "error",
                        Category:   "security",
                        Message:    "SQL injection vulnerability: user input directly concatenated into query",
                        Suggestion: "Use parameterized queries: db.Query(\"SELECT * FROM users WHERE id = ?\", userID)",
                    },
                    {
                        LineNumber: 23,
                        Severity:   "warning",
                        Category:   "best_practice",
                        Message:    "Error not properly wrapped before returning",
                        Suggestion: "Use fmt.Errorf(\"failed to authenticate: %w\", err)",
                    },
                },
            },
            {
                FilePath:     "internal/handler/handler.go",
                OverallScore: 8,
                Summary:      "Good structure. Minor style improvements recommended.",
                Comments: []CodeReviewComment{
                    {
                        LineNumber: 12,
                        Severity:   "info",
                        Category:   "style",
                        Message:    "Variable name 'tmp' is not descriptive",
                        Suggestion: "Rename to 'parsedRequest' or similar",
                    },
                    {
                        LineNumber: 67,
                        Severity:   "info",
                        Category:   "performance",
                        Message:    "String concatenation in loop could be optimized",
                        Suggestion: "Use strings.Builder for better performance",
                    },
                    {
                        LineNumber: 89,
                        Severity:   "warning",
                        Category:   "bug",
                        Message:    "Potential nil pointer dereference if ctx.User is nil",
                        Suggestion: "Add nil check: if ctx.User == nil { return ErrUnauthorized }",
                    },
                },
            },
        },
        ApprovalStatus: "changes_requested",
    },
    ConversationID: "conv-review-321",
    TokensUsed:     890,
}
```

---

## Flexibility of Structured Responses

### 1. Optional vs Required Fields

```go
type FlexibleResponse struct {
    // Required fields - AI MUST provide these
    Action string `json:"action" openai:"description=Action to take,required"`

    // Optional fields - AI provides only if relevant
    Data   string `json:"data,omitempty" openai:"description=Additional data if needed"`
}
```

### 2. Enums for Controlled Values

```go
type StatusResponse struct {
    Status string `json:"status" openai:"description=Status,required,enum=success|error|pending"`
    //                                                                        ^^^^^^^^^^^^^^^
    // AI can ONLY return one of these three values
}
```

### 3. Numeric Constraints

```go
type ScoreResponse struct {
    Score      int     `json:"score" openai:"description=Score 1-100,required,min=1,max=100"`
    Confidence float64 `json:"confidence" openai:"description=Confidence 0-1,required,min=0,max=1"`
}
```

### 4. String Length Constraints

```go
type SummaryResponse struct {
    ShortSummary string `json:"short" openai:"description=One sentence summary,required,maxLength=100"`
    FullSummary  string `json:"full" openai:"description=Detailed summary,required,minLength=50,maxLength=500"`
}
```

### 5. Arrays of Arbitrary Length

```go
type ListResponse struct {
    Items []string `json:"items" openai:"description=List of items,required"`
    //    ^^ No length limit - AI returns as many as needed
}
```

### 6. Deeply Nested Structures

```go
type User struct {
    Name    string `json:"name" openai:"description=User name,required"`
    Email   string `json:"email" openai:"description=Email,required"`
}

type Team struct {
    Name    string `json:"name" openai:"description=Team name,required"`
    Members []User `json:"members" openai:"description=Team members,required"`
}

type Organization struct {
    Name  string `json:"name" openai:"description=Org name,required"`
    Teams []Team `json:"teams" openai:"description=Teams in org,required"`
}
// OpenAI can return complex nested hierarchies
```

### 7. Mix of Types

```go
type AnalysisResponse struct {
    // String
    Summary string `json:"summary" openai:"description=Analysis summary,required"`

    // Number
    Score int `json:"score" openai:"description=Score,required"`

    // Boolean
    IsComplete bool `json:"is_complete" openai:"description=Analysis complete,required"`

    // Array of strings
    Keywords []string `json:"keywords" openai:"description=Key terms,required"`

    // Array of objects
    Issues []Issue `json:"issues" openai:"description=Found issues,required"`

    // Map (object with dynamic keys)
    Metadata map[string]string `json:"metadata,omitempty" openai:"description=Additional metadata"`
}
```

---

## Summary: AI Layer's Perspective

**What AI Layer Sends:**
- Context (conversation ID or nil)
- User message
- Personality/instructions
- Empty struct pointer defining expected response shape
- Optional parameters (model, temperature, etc.)

**What AI Layer Receives:**
- Typed data matching the struct you provided
- Conversation ID (for continuing conversation)
- Metadata (tokens, model, response ID)

**Key Benefits:**
1. **Type safety** - Response data is already parsed into Go structs
2. **Validation** - OpenAI validates response matches schema before returning
3. **Flexibility** - Structs can be simple or complex, optional or required
4. **Consistency** - Structured output eliminates parsing errors
5. **Reusability** - Same struct definition used for schema generation and response parsing
