# RMS Research Sub-Workflow Design

## Overview
A sub-workflow for researching RMS-specific guidelines and context, optimized for speed while maintaining accuracy.

## Architecture

### 1. Database Schema Extensions

#### New Tables

```sql
-- RMS domain knowledge cache
CREATE TABLE rms_knowledge_cache (
    id INT PRIMARY KEY AUTO_INCREMENT,
    rms_identifier VARCHAR(100) NOT NULL,
    project_name VARCHAR(255) NOT NULL,
    project_url VARCHAR(500),
    summary TEXT,                          -- Markdown summary of RMS purpose/guidelines
    ai_conversation_id VARCHAR(255),       -- Persistent AI conversation for this RMS
    conversation_turn_count INT DEFAULT 0, -- Track when to flush/summarize
    last_accessed_at TIMESTAMP,
    last_updated_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY unique_rms_project (rms_identifier, project_name),
    INDEX idx_rms_identifier (rms_identifier),
    INDEX idx_last_accessed (last_accessed_at)
) ENGINE=InnoDB;

-- Cached file paths with tiering
CREATE TABLE rms_file_paths (
    id INT PRIMARY KEY AUTO_INCREMENT,
    rms_knowledge_cache_id INT NOT NULL,
    file_path VARCHAR(1000) NOT NULL,
    path_type ENUM('always', 'conditional', 'discovered') NOT NULL,
    relevance_tags JSON,                   -- ["documentation", "guidelines", "architecture"]
    file_hash VARCHAR(64),                 -- SHA-256 for change detection
    content_summary TEXT,                  -- AI-generated summary of file
    full_content MEDIUMTEXT,               -- Optional: cache actual content
    last_checked_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (rms_knowledge_cache_id) REFERENCES rms_knowledge_cache(id) ON DELETE CASCADE,
    INDEX idx_path_type (rms_knowledge_cache_id, path_type),
    INDEX idx_file_hash (file_hash)
) ENGINE=InnoDB;

-- Request-specific knowledge updates (for background enrichment)
CREATE TABLE rms_knowledge_updates (
    id INT PRIMARY KEY AUTO_INCREMENT,
    rms_knowledge_cache_id INT NOT NULL,
    request_context TEXT,                  -- What was the user asking about?
    knowledge_added TEXT,                  -- What did we learn?
    paths_discovered JSON,                 -- New paths found during enrichment
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (rms_knowledge_cache_id) REFERENCES rms_knowledge_cache(id) ON DELETE CASCADE,
    INDEX idx_rms_created (rms_knowledge_cache_id, created_at DESC)
) ENGINE=InnoDB;
```

### 2. Workflow Definition

**Name:** `rms_research`

**Inputs:**
- `rms_identifier`: String - RMS name/code
- `user_request_context`: String - What user is trying to do
- `project_hint`: String (optional) - Specific project if known

**Outputs:**
- `summary`: String - Markdown formatted research results
- `relevant_paths`: Array[String] - Key file paths for reference
- `confidence`: Float - How confident we are in the results (0-1)

**Steps:**
1. `init` - Load RMS cache from DB
2. `process_cached_knowledge` - Feed cached data to AI
3. `check_always_paths` - Fetch and process "always check" files
4. `targeted_research` - AI-driven targeted exploration
5. `return_results` - Format and return summary
6. `background_enrichment` (async) - Improve knowledge base

### 3. Git Integration Tools

#### Tool: `git_fetch_file`
```go
Input:
- project_url: String (Azure Repos project URL)
- file_path: String (relative path in repo)
- branch: String (default: "main")

Output:
- content: String (file contents)
- commit_hash: String (latest commit affecting this file)
- last_modified: Timestamp

Implementation:
- Use Azure Repos REST API
- Endpoint: GET https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}/items?path={path}&api-version=7.0
- Cache auth token in workflow context
- Return empty if 404 (file not found)
```

#### Tool: `git_search_files`
```go
Input:
- project_url: String
- search_pattern: String (e.g., "*.md", "guidelines", "README")
- max_results: Int (default: 20)

Output:
- files: Array[{path, size, last_modified}]

Implementation:
- Use Azure Repos Search API or Items API with path filters
- Sort by relevance (name match) and recency
```

#### Tool: `git_get_recent_changes`
```go
Input:
- project_url: String
- file_path: String (optional - specific file or whole repo)
- since: Timestamp (optional - last N days)

Output:
- commits: Array[{hash, message, author, date, files_changed}]

Implementation:
- Use Commits API: GET {org}/{project}/_apis/git/repositories/{repo}/commits
- Useful for understanding recent documentation updates
```

### 4. Workflow Execution Flow

#### Phase 1: Fast Response (< 2 seconds target)

```
1. DB Cache Lookup
   ↓
   SELECT * FROM rms_knowledge_cache
   WHERE rms_identifier = ? AND project_name = ?

   If found:
   - Load AI conversation ID
   - Load summary
   - Load "always" paths
   ↓
2. Resume AI Conversation
   ↓
   conversationKey := fmt.Sprintf("rms_%s", rmsIdentifier)
   aiConvID := workflowCtx.GetAIConversation(&conversationKey)

   Prompt:
   "Here's cached knowledge about {RMS}:
   {summary}

   User request: {user_request_context}

   Based on what you know, provide initial guidance."
   ↓
3. Fetch "Always" Paths (parallel)
   ↓
   SELECT file_path, file_hash FROM rms_file_paths
   WHERE rms_knowledge_cache_id = ? AND path_type = 'always'

   For each path:
   - git_fetch_file() in parallel using ActionAsync
   - Check if hash changed (if so, mark for update)
   - Feed to AI conversation
   ↓
4. Generate Initial Response
   ↓
   AI synthesizes:
   - Cached summary
   - Always-check files
   - User request context

   Returns structured response
   ↓
5. Return to Parent Workflow
   ↓
   Result includes:
   - summary (markdown)
   - relevant_paths (for user reference)
   - confidence score
```

#### Phase 2: Background Enrichment (async, max 30 seconds)

```
Spawn ActionAsync → Background goroutine
   ↓
1. Targeted Discovery
   ↓
   AI analyzes user request + current knowledge
   Identifies gaps: "Need more info about X"
   ↓
   Use git_search_files() to find relevant files
   Example: User asked about "authentication flow"
            → Search for: ["auth", "login", "security", "README"]
   ↓
2. Selective File Fetch
   ↓
   Fetch top 5-10 most relevant files
   - Filter by path keywords (docs/, guidelines/, README)
   - Prioritize .md, .txt, architecture diagrams
   - Skip code files unless specifically about architecture
   ↓
3. Knowledge Extraction
   ↓
   Feed files to AI conversation:
   "Analyze these files and extract key guidelines/patterns
   relevant to: {user_request_context}"

   AI produces:
   - Updated summary
   - New important paths
   - Categorization (always vs conditional paths)
   ↓
4. DB Update (Transaction)
   ↓
   BEGIN TRANSACTION;

   -- Update summary
   UPDATE rms_knowledge_cache
   SET summary = ?,
       conversation_turn_count = conversation_turn_count + 1,
       last_updated_at = NOW()
   WHERE id = ?;

   -- Add new paths
   INSERT INTO rms_file_paths (...)
   VALUES (...)
   ON DUPLICATE KEY UPDATE file_hash = ?, content_summary = ?;

   -- Log what we learned
   INSERT INTO rms_knowledge_updates (request_context, knowledge_added, ...)
   VALUES (...);

   -- Check if conversation needs summarization
   IF conversation_turn_count > 25 THEN
       → Spawn summarization task
       → Create new AI conversation
       → Seed with summary
   END IF;

   COMMIT;
   ↓
5. Complete
   ↓
   ActionCompleteAsync
```

### 5. Conversation Lifecycle Management

#### Conversation Growth → Summarization

**Trigger:** `conversation_turn_count > 25`

**Process:**
1. Create new AI conversation
2. Feed it with:
   ```
   System: You are a domain expert on {RMS}.
   Here's accumulated knowledge from 25 previous interactions:

   {current_summary}

   Recent learnings:
   {last 5 knowledge_updates}

   Key files:
   {top 10 most referenced paths}
   ```
3. Update DB:
   ```sql
   UPDATE rms_knowledge_cache
   SET ai_conversation_id = ?,
       conversation_turn_count = 0,
       summary = ?
   WHERE id = ?;
   ```

#### Periodic Flush (Optional)

**Trigger:** Cron job every 3 months OR manual trigger

**Process:**
- Archive old conversations
- Re-summarize based on recent activity
- Prune low-relevance paths

### 6. Path Tiering Strategy

#### Always Paths
- READMEs
- Main documentation index
- Architecture decision records
- Critical guidelines (coding standards, security policies)
- Fetched on EVERY request

#### Conditional Paths
- Specific feature documentation
- Component-specific guidelines
- Historical decisions
- Fetched based on request context keywords

#### Discovered Paths
- Found during background enrichment
- Added to conditional pool if referenced multiple times
- Pruned if not accessed in 6 months

### 7. Efficiency Optimizations

#### Parallel Fetching
```go
// In workflow function
actions := []*core.Action{}

for _, path := range alwaysPaths {
    action := core.NewAction(core.ActionAsync)

    toolAction := core.NewAction(core.ActionTool)
    toolAction.Input[core.InputToolName] = tool.ToolGitFetchFile
    toolAction.Input[core.InputToolArgs] = map[string]any{
        "project_url": projectURL,
        "file_path": path,
    }

    action.AsyncActions = []*core.Action{toolAction}
    actions = append(actions, action)
}

return actions, nil
```

#### Hash-Based Change Detection
```go
// Only re-fetch if file changed
cachedHash := getCachedFileHash(path)
latestCommit := getLatestCommitHash(projectURL, path) // Lightweight API call

if cachedHash != latestCommit {
    // File changed, fetch new content
    content := fetchFile(projectURL, path)
    updateCache(path, content, latestCommit)
} else {
    // Use cached content
    content := getCachedContent(path)
}
```

#### Lazy Content Caching
```
First request:
- Fetch file content via API
- Generate summary with AI
- Store summary in DB (always)
- Store full content (optional, configurable)

Subsequent requests:
- Use summary for most queries
- Fetch full content only if AI needs details
```

### 8. Performance Targets

| Phase | Target | Acceptable |
|-------|--------|------------|
| DB cache lookup | < 50ms | < 100ms |
| AI response generation | < 1s | < 2s |
| Always-path fetching (parallel) | < 500ms | < 1s |
| **Total fast response** | **< 2s** | **< 3s** |
| Background enrichment | < 20s | < 30s |

### 9. Error Handling & Degradation

#### Cache Miss (First Time)
- No cached data available
- Fetch README + search for "guidelines"
- Initial response less confident
- Background enrichment more aggressive

#### API Rate Limiting
- Return cached data even if stale
- Log warning
- Retry background update later

#### File Not Found (404)
- Mark path as invalid in DB
- Remove from cache
- Continue with remaining paths

#### AI Timeout
- Return partial results
- Mark as low confidence
- Retry in background

### 10. Integration with Parent Workflow

#### Parent Workflow Call
```go
// In parent workflow function
func ProcessUserRequest(ctx *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
    userMessage := sourceAction.Input[core.InputMessage].(string)

    // Detect RMS mention (example: user mentioned "Payment RMS")
    rmsIdentifier := detectRMSFromMessage(userMessage)

    if rmsIdentifier != "" {
        // Spawn sub-workflow
        subWorkflow := core.NewAction(core.ActionWorkflow)
        subWorkflow.Input[core.InputWorkflowName] = workflow.WorkflowRMSResearch
        subWorkflow.Input[core.InputWorkflowData] = map[string]any{
            "rms_identifier": rmsIdentifier,
            "user_request_context": userMessage,
        }

        return []*core.Action{subWorkflow}, nil
    }

    // ... rest of workflow
}

// When sub-workflow completes
func HandleSubWorkflowResult(ctx *core.ConversationContext, result *core.Action) {
    subResult := result.Input[core.InputWorkflowResult].(map[string]any)

    summary := subResult["summary"].(string)
    paths := subResult["relevant_paths"].([]string)
    confidence := subResult["confidence"].(float64)

    // Feed into main workflow AI conversation
    mainAI := ctx.GetAIConversation(nil)
    aiAction := core.NewAction(core.ActionAi)
    aiAction.Input[core.InputSystemPrompt] = fmt.Sprintf(
        "RMS Research Results (confidence: %.0f%%):\n%s\n\nRelevant files: %v",
        confidence * 100,
        summary,
        paths,
    )

    return []*core.Action{aiAction}, nil
}
```

## Implementation Phases

### Phase 1: Foundation (Week 1)
- [ ] Database schema (3 new tables)
- [ ] Git tool implementations (3 tools)
- [ ] Basic cache lookup/update logic

### Phase 2: Core Workflow (Week 2)
- [ ] RMS research workflow definition
- [ ] Fast response path (cache + AI)
- [ ] Path tiering logic
- [ ] Integration with parent workflows

### Phase 3: Background Enrichment (Week 3)
- [ ] Async update mechanism
- [ ] Targeted file discovery
- [ ] Knowledge update logging
- [ ] Hash-based change detection

### Phase 4: Lifecycle Management (Week 4)
- [ ] Conversation summarization
- [ ] Path pruning/promotion
- [ ] Periodic maintenance jobs
- [ ] Performance monitoring

### Phase 5: Optimization (Week 5)
- [ ] Parallel fetching optimization
- [ ] Lazy content caching
- [ ] API rate limit handling
- [ ] Error recovery strategies

## Testing Strategy

### Unit Tests
- Cache lookup/update operations
- Path tiering logic
- Hash comparison
- Summarization triggers

### Integration Tests
- Full workflow execution with mock git API
- Async background update completion
- Sub-workflow → parent workflow communication
- Database transaction rollback scenarios

### Performance Tests
- Measure response times under various cache states
- Concurrent request handling
- Large file handling (>1MB markdown files)
- API timeout scenarios

## Monitoring & Observability

### Metrics to Track
- Cache hit rate
- Average response time (fast phase)
- Background enrichment completion time
- API call count per request
- Conversation turn count distribution
- Path type distribution (always/conditional/discovered)

### Logging
- All git API calls (with timing)
- Cache invalidations
- Background enrichment completions
- Summarization events
- Error rates by type

## Security Considerations

### Authentication
- Azure DevOps PAT (Personal Access Token) stored encrypted
- Token rotation support
- Scope: Code (Read) only

### Access Control
- Users can only access RMS data they have ADO permissions for
- Git tool validates user access before fetching

### Data Privacy
- No PII stored in summaries
- File content caching opt-in per RMS
- Conversation IDs anonymized in logs

## Cost Estimation

### Storage (per RMS)
- Knowledge cache: ~10 KB
- File paths (avg 50): ~50 KB
- Cached summaries: ~20 KB
- Updates log (1 year): ~100 KB
- **Total: ~180 KB per RMS**

For 100 RMS: ~18 MB (negligible)

### API Calls (per request)
- Cache hit: 0 API calls
- Cache miss + 10 always paths: 11 API calls
- Background enrichment: 10-20 API calls

Azure DevOps: Free tier = 1,800 API calls/hour
Expected: ~50 requests/hour = ~500 API calls/hour (well within limits)

### AI Costs (per request)
- Fast response: ~1,000 tokens input, ~500 tokens output
- Background enrichment: ~5,000 tokens input, ~1,000 tokens output
- Conversation persistence reduces repeated context loading

## Future Enhancements

### Semantic Search
- Embed file summaries
- Vector similarity for path selection
- Reduces need for "always" paths

### Multi-Project Correlation
- Detect patterns across RMS projects
- "Payment RMS and Billing RMS both use pattern X"

### Predictive Pre-Fetching
- Learn which paths users typically need together
- Pre-fetch conditional paths proactively

### Collaborative Filtering
- "Users who asked about X also needed Y"
- Suggest related guidelines

