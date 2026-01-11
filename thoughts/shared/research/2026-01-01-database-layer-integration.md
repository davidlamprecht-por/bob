---
date: 2026-01-01T00:00:00Z
researcher: Claude Sonnet 4.5
git_commit: 065ab62434d6b1595e2c071076d65f112ae11f0d
branch: master
repository: bob
topic: "Database Layer Integration with Conversation Context and Cache"
tags: [research, codebase, database, conversation_context, cache, id-management, orchestrator]
status: complete
last_updated: 2026-01-01
last_updated_by: Claude Sonnet 4.5
---

# Research: Database Layer Integration with Conversation Context and Cache

**Date**: 2026-01-01T00:00:00Z
**Researcher**: Claude Sonnet 4.5
**Git Commit**: 065ab62434d6b1595e2c071076d65f112ae11f0d
**Branch**: master
**Repository**: bob

## Research Question

Identify how the codebase currently works for conversation_context, cache, and database components. Document the existing structure to understand how to add a DB layer that integrates with these components, including structured ways to automatically collect internal IDs when external IDs are given. The system should be simple to use, not too bloated, and easily addable.

## Summary

The Bob codebase implements a **three-tier architecture** for conversation state management:

1. **Hot Tier (Memory Cache)**: In-memory map-based cache with thread-safe access and priority-based eviction (`internal/orchestrator/cache.go`)
2. **Cold Tier (Database)**: MySQL-backed persistence layer with migration system (`internal/database/`)
3. **ID Mapping Layer**: External platform IDs (Slack, Teams, etc.) mapped to internal database IDs via dedicated tables

**Current Implementation Status**:
- ✅ **Complete**: Cache layer, database schema, migration system, connection pooling, message structure
- ⚠️ **Stubbed**: Database persistence operations (LoadContext, UpdateDB, ID resolution)
- 📋 **Planned**: TTL-based cache eviction, smart eviction recovery, automated ID resolution

The architecture uses a **lazy ID resolution pattern** where external IDs are used for cache lookups, and internal database IDs are resolved on-demand when needed for persistence. The conversation context tracks workflow state, action queues, user messages, and execution status with thread-safe accessor methods.

## Detailed Findings

### 1. Conversation Context System

**Location**: `internal/orchestrator/context.go`

The `ConversationContext` struct is the core state container for ongoing conversations:

```go
type ConversationContext struct {
    mu sync.RWMutex                    // Thread-safe access control

    currentWorkflow  *WorkflowContext  // Active workflow state
    currentStatus    ContextStatus     // Idle, Running, WaitingForUser, Error, Evicted
    lastUserMessages []*Message        // History of user messages

    remainingActions []Action          // Actions queued for processing
    requestToUser    string            // Last request/question sent to user

    lastUpdated time.Time              // For LRU eviction tracking
    createdAt   time.Time              // Context creation timestamp
}
```

**Key Methods**:
- Thread-safe getters (`GetCurrentWorkflow()`, `GetCurrentStatus()`, etc.) at lines 24-60
- Thread-safe setters that update `lastUpdated` timestamp at lines 62-97
- Atomic operations: `AppendUserMessage()`, `AppendRemainingActions()`, `PopRemainingActions()` at lines 99-123

**Context Loading** (`context.go:146-176`):
```go
func LoadContext(refMessage *Message) *ConversationContext {
    userID := refMessage.UserID.ExternalID
    threadID := refMessage.ThreadID.ExternalID

    // 1. Check cache (hot path)
    context := GetFromCache(userID, threadID)
    if context != nil {
        context.AppendUserMessage(refMessage)
        return context
    }

    // 2. Load from DB (cold path) - STUBBED
    context = loadContextFromDB(refMessage)

    // 3. Create new if not found
    if context == nil {
        context = &ConversationContext{
            currentStatus: StatusIdle,
            lastUpdated:   time.Now(),
            createdAt:     time.Now(),
        }
    }

    context.AppendUserMessage(refMessage)
    PutInCache(userID, threadID, context)

    return context
}
```

**Status State Machine** (`context.go:125-133`):
- `StatusIdle` - No active processing
- `StatusRunning` - Actions being processed
- `StatusWaitForUser` - Paused awaiting response
- `StatusError` - Processing encountered error
- `StatusEvicted` - Removed from cache due to memory pressure

**Integration with Orchestrator** (`orchestrator.go:15-63`):
- `HandleUserMessage()` loads context at line 18
- `handleEvictedContext()` provides recovery for evicted contexts at lines 55-63
- `RouteUserMessage()` determines action routing based on context status at lines 126-147
- `StartHandlingActions()` processes action queue and updates context status at lines 72-124

### 2. Cache System

**Location**: `internal/orchestrator/cache.go`

**Storage Structure** (lines 12-13):
```go
var contextCache = make(map[string]*ConversationContext)
var cacheMutex sync.RWMutex
```

**Cache Key Format** (line 16-18):
```go
func cacheKey(userID, threadID string) string {
    return userID + ":" + threadID  // Format: "U123ABC:C456DEF"
}
```

**Core Operations**:

1. **GetFromCache()** (`cache.go:21-29`)
   - Thread-safe read with RWLock
   - Returns nil on cache miss
   - Hot path for context retrieval

2. **PutInCache()** (`cache.go:32-47`)
   - Thread-safe write with Lock
   - Triggers eviction if cache exceeds `MaxCacheSize` (default: 10,000)
   - Sets creation timestamp if not already set

3. **RemoveFromCache()** (`cache.go:50-54`)
   - Direct deletion by key
   - Currently unused in codebase

**Eviction Strategy** (`cache.go:56-155`):

Priority-based LRU eviction with four tiers:

1. **Tier 1 - Idle Contexts** (lines 104-111): Evict `StatusIdle` contexts first (oldest by `lastUpdated`)
2. **Tier 2 - Error Contexts** (lines 113-120): Evict `StatusError` contexts second
3. **Tier 3 - WaitForUser Contexts** (lines 122-131): Evict with special handling:
   - Mark as `StatusEvicted` (line 127)
   - TODO: Save to database before eviction (line 128)
   - Log warning (line 130)
4. **Tier 4 - Emergency Eviction** (lines 133-154): If cache exceeds `GraceBufferSize` (default: 11,000):
   - Evict all remaining contexts
   - Mark non-idle/error as `StatusEvicted` (line 147)
   - TODO: Save to database (line 148)
   - Log critical error (line 135)

**Configuration** (`config/config.go:39-41`):
- `MaxCacheSize`: 10,000 contexts (env: `SESSION_CACHE_MAX_SIZE`)
- `GraceBufferSize`: MaxCacheSize + 1,000 (env: `SESSION_CACHE_GRACE_BUFFER`)
- `CacheTTLSeconds`: 28,800 seconds / 8 hours (env: `SESSION_CACHE_TTL_SECONDS`) - **Not yet implemented**

**Data Flow**:
```
User Message → LoadContext()
    ↓
GetFromCache(userID, threadID)
    ↓ [cache miss]
loadContextFromDB(message) [STUB]
    ↓ [not found]
Create new ConversationContext
    ↓
PutInCache(userID, threadID, context)
    ↓ [if cache full]
evictToLimit() - Priority-based eviction
```

### 3. Database Layer

**Location**: `internal/database/`

#### Connection Management (`database.go:12-50`)

**Global Connection**:
```go
var DB *sql.DB
```

**Connect()** function (lines 15-34):
- Uses `github.com/go-sql-driver/mysql` driver
- Connection pool settings:
  - MaxOpenConns: 25
  - MaxIdleConns: 5
  - ConnMaxLifetime: 5 minutes
  - ConnMaxIdleTime: 2 minutes
- Tests connection with `Ping()`

**Configuration** (`config/config.go:62-114, 162-165`):
```go
func (c *Config) DBConnectionString() string {
    return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
        c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}
```

Environment variables:
- `DB_HOST` (required)
- `DB_PORT` (optional, default: 3306)
- `DB_USER` (required)
- `DB_PASSWORD` (required)
- `DB_NAME` (required)

#### Migration System (`database/migrations.go`)

**MigrationRunner** struct (lines 26-31):
```go
type MigrationRunner struct {
    db                 *sql.DB
    migrationsDir      string
    migrations         []Migration
    executedMigrations map[string]Migration  // Tracks executed migrations
}
```

**Key Functions**:
- `LoadMigrations()` (lines 44-78): Loads `m*.sql` files, calculates SHA256 checksums
- `LoadExecutedMigrations()` (lines 81-137): Queries `migrations` table for applied migrations
- `ValidateMigrations()` (lines 153-175): Validates checksums to detect file modifications
- `RunMigration()` (lines 178-229): Executes single migration and records execution time
- `RunPendingMigrations()` (lines 232-251): Executes all unapplied migrations in order

**Migration Command** (`cmd/migrate/main.go`):
- Entry point: `cmd/migrate/main.go:11-58`
- Commands: "run"/"up" (execute migrations), "status" (show status)
- Initializes config, connects to DB, runs migrations

#### Database Schema (`definitions/migrations/m0001_schema.sql`)

**Table 1: user_external_ids** (lines 10-18)
```sql
CREATE TABLE user_external_ids (
    id INT AUTO_INCREMENT PRIMARY KEY,           -- Internal user ID
    external_id VARCHAR(100) NOT NULL,           -- Platform user ID
    platform VARCHAR(50) NOT NULL,               -- "slack", "teams", etc.
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE INDEX idx_external_platform (external_id, platform),
    INDEX idx_platform (platform)
);
```

**Table 2: thread_external_ids** (lines 23-31)
```sql
CREATE TABLE thread_external_ids (
    id INT AUTO_INCREMENT PRIMARY KEY,           -- Internal thread ID
    external_id VARCHAR(255) NOT NULL,           -- Platform thread ID
    platform VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE INDEX idx_external_platform (external_id, platform),
    INDEX idx_platform (platform)
);
```

**Table 3: workflow_context** (lines 36-45)
```sql
CREATE TABLE workflow_context (
    id INT AUTO_INCREMENT PRIMARY KEY,
    workflow_name VARCHAR(100) NOT NULL,
    workflow_step VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_workflow_name (workflow_name),
    INDEX idx_workflow_step (workflow_name, workflow_step)
);
```

**Table 4: workflow_context_data** (lines 50-65)
```sql
CREATE TABLE workflow_context_data (
    id INT AUTO_INCREMENT PRIMARY KEY,
    workflow_context_id INT NOT NULL,
    `key` VARCHAR(100) NOT NULL,
    value TEXT,                                  -- Can be JSON, number, etc.
    data_type VARCHAR(20) DEFAULT 'string',     -- Type hint
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    FOREIGN KEY (workflow_context_id) REFERENCES workflow_context(id) ON DELETE CASCADE,
    UNIQUE INDEX idx_workflow_unique_key (workflow_context_id, `key`),
    INDEX idx_key (`key`)
);
```

**Table 5: conversation_context** (lines 70-94)
```sql
CREATE TABLE conversation_context (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,                        -- FK to user_external_ids
    thread_id INT NOT NULL,                      -- FK to thread_external_ids
    workflow_context_id INT NULL,                -- FK to workflow_context
    context_status VARCHAR(50) NOT NULL DEFAULT 'INITIAL',
    request_to_user TEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES user_external_ids(id) ON DELETE RESTRICT,
    FOREIGN KEY (thread_id) REFERENCES thread_external_ids(id) ON DELETE RESTRICT,
    FOREIGN KEY (workflow_context_id) REFERENCES workflow_context(id) ON DELETE SET NULL,
    UNIQUE INDEX idx_user_thread (user_id, thread_id),
    INDEX idx_context_status (context_status),
    INDEX idx_workflow_context (workflow_context_id),
    INDEX idx_updated_at (updated_at)
);
```

**Table 6: ai_conversations** (lines 99-114)
```sql
CREATE TABLE ai_conversations (
    id INT AUTO_INCREMENT PRIMARY KEY,
    conversation_context_id INT NOT NULL,        -- FK to conversation_context
    provider VARCHAR(50) NOT NULL DEFAULT 'openai',
    provider_conversation_id VARCHAR(255) NOT NULL,
    metadata JSON NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    FOREIGN KEY (conversation_context_id) REFERENCES conversation_context(id) ON DELETE CASCADE,
    INDEX idx_conversation_context (conversation_context_id),
    INDEX idx_provider (provider, provider_conversation_id)
);
```

**Table 7: migrations** (lines 119-127)
```sql
CREATE TABLE migrations (
    id INT AUTO_INCREMENT PRIMARY KEY,
    migration_name VARCHAR(255) UNIQUE NOT NULL,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(64) NULL,
    execution_time_ms INT NULL,

    INDEX idx_applied_at (applied_at)
);
```

#### Current Database Operations Status

**Implemented**:
- Database connection with pooling
- Migration system with checksum validation
- Schema creation and table definitions
- Health check functionality

**Stubbed (Not Implemented)**:
- `loadContextFromDB()` (`context.go:178-184`) - Returns nil
- `UpdateDB()` (`context.go:186-193`) - Empty implementation with TODO
- `Message.GetResolved()` (`message.go:32-39`) - ID resolution stub
- Context save on eviction (`cache.go:128, 148`) - TODO comments

### 4. ID Management Patterns

**Location**: `internal/orchestrator/message.go`

#### Two-Tier ID System

**External IDs** (Platform-Specific):
- Slack user IDs (e.g., "U123ABC")
- Slack channel/thread IDs (e.g., "C456DEF")
- Teams user IDs, Discord user IDs, etc.
- Variable format per platform

**Internal IDs** (Database-Specific):
- Auto-increment integers in `user_external_ids.id`
- Auto-increment integers in `thread_external_ids.id`
- Decoupled from platform for independence

#### PlatformRef Structure (`message.go:12-17`)

```go
type PlatformRef struct {
    ExternalID string         // Platform-specific ID (currently populated)
    Platform   PlatformType   // "Slack", "Teams", etc.
    InternalID *string        // Pointer to internal DB ID (stub for future)
}
```

**Platform Types** (`message.go:22-26`):
```go
type PlatformType string

const (
    PlatformSlack PlatformType = "Slack"
    // Future: PlatformTeams, PlatformDiscord, PlatformEmail
)
```

#### Message Structure (`message.go:5-10`)

```go
type Message struct {
    UserID    *PlatformRef   // User identifier with external/internal IDs
    ThreadID  *PlatformRef   // Thread identifier with external/internal IDs
    Message   string
    Timestamp time.Time
}
```

#### ID Resolution Pattern (Planned)

**GetResolved()** stub (`message.go:32-39`):
```go
func (m *Message) GetResolved() {
    if m.UserID.InternalID == nil {
        // TODO: Resolve with db class
    }
    if m.ThreadID.InternalID == nil {
        // TODO: Resolve with db class
    }
}
```

**Planned Resolution Flow**:
1. Message arrives with external IDs from platform (e.g., Slack)
2. PlatformRef stores `ExternalID` and `Platform`
3. `GetResolved()` queries `user_external_ids` table:
   - `SELECT id FROM user_external_ids WHERE external_id=? AND platform=?`
   - If not found, insert new record: `INSERT INTO user_external_ids (external_id, platform) VALUES (?, ?)`
   - Store returned `id` in `InternalID`
4. Same process for thread IDs with `thread_external_ids` table
5. Use internal IDs for database operations (foreign keys in `conversation_context`)

#### ID Usage Throughout System

**Cache Keys** (`cache.go:16-18`):
- Uses external IDs directly: `userID + ":" + threadID`
- Example: `"U123ABC:C456DEF"`
- External IDs extracted at entry point (`context.go:147-148`):
  ```go
  userID := refMessage.UserID.ExternalID
  threadID := refMessage.ThreadID.ExternalID
  ```

**Database Queries** (Planned):
- Use internal IDs for foreign key relationships
- `conversation_context.user_id` → `user_external_ids.id`
- `conversation_context.thread_id` → `thread_external_ids.id`
- Join pattern:
  ```sql
  SELECT cc.* FROM conversation_context cc
  JOIN user_external_ids uei ON cc.user_id = uei.id
  JOIN thread_external_ids tei ON cc.thread_id = tei.id
  WHERE uei.external_id = ? AND uei.platform = ?
    AND tei.external_id = ? AND tei.platform = ?
  ```

#### Other ID Patterns

**AsyncGroupID** (`action.go:5-17`):
- Workflow-generated string identifier for grouping async actions
- Used to correlate results from parallel operations
- Empty string indicates synchronous action

**Workflow Context Data Keys** (`workflow_context_data` table):
- Generic key-value storage with string keys
- Flexible schema for workflow-specific state
- Type hints stored in `data_type` column

**Migration Tracking** (`migrations.go:25-31`):
- Map-based ID tracking: `map[string]Migration`
- Migration name as unique identifier
- Enables O(1) lookup for executed migrations

### 5. Message Parsing and Entry Points

**Location**: `internal/slack/parser.go`

**ParseMessage()** function (lines 6-23):
```go
func ParseMessage(userId, threadId, text string, ts time.Time) (*orchestrator.Message, error) {
    return &orchestrator.Message{
        UserID: &orchestrator.PlatformRef{
            Platform:   orchestrator.PlatformSlack,
            ExternalID: userId,
        },
        ThreadID: &orchestrator.PlatformRef{
            Platform:   orchestrator.PlatformSlack,
            ExternalID: threadId,
        },
        Message:   text,
        Timestamp: ts,
    }, nil
}
```

**Key Observations**:
- Direct passthrough of external IDs from Slack
- No transformation or validation of IDs
- Platform type explicitly set to `PlatformSlack`
- `InternalID` left nil for lazy resolution
- Simple struct initialization pattern

### 6. Action Processing and Workflow Integration

**Location**: `internal/orchestrator/action.go`, `internal/orchestrator/orchestrator.go`

#### Action Structure (`action.go:5-17`)

```go
type Action struct {
    ActionType     ActionType
    SourceWorkflow string
    AsyncGroupID   string                 // Correlation ID for async groups
    AsyncGroupSize int                    // Expected result count
    Input          map[string]any         // Generic data carrier
    AsyncActions   []Action
}
```

#### Action Processing Loop (`orchestrator.go:72-124`)

**StartHandlingActions()** manages action queue and context state:

1. **Initialization** (line 77): `context.SetCurrentStatus(StatusRunning)`
2. **Queue Management** (line 82): `actionQueue = append(actionQueue, context.PopRemainingActions()...)`
3. **Wait Check** (lines 85-89): If `StatusWaitForUser`, store remaining actions and break
4. **Processing** (line 96): `currentAction.ProcessAction(context, responder, actionChan)`
5. **Error Handling** (lines 110-114): Set `StatusError` and store queue
6. **Completion** (lines 118-121): Set `StatusIdle` if finished normally

#### Message Routing (`orchestrator.go:126-147`)

**RouteUserMessage()** determines action flow based on context status:

- **WaitForUser** (lines 128-136): User responding to pending request
  - If new request: Create new workflow, clear actions
  - Return `true` to start new action loop
- **Running** (lines 140-143): User interjecting during processing
  - Append new actions to context
  - Return `false` (main loop picks them up)
- **Default** (line 146): Return `true` to start action loop

## Code References

### Core Components

- `internal/orchestrator/context.go:8-22` - ConversationContext struct definition
- `internal/orchestrator/context.go:146-176` - LoadContext() entry point
- `internal/orchestrator/context.go:178-184` - loadContextFromDB() stub
- `internal/orchestrator/context.go:186-193` - UpdateDB() stub
- `internal/orchestrator/cache.go:12-13` - Global cache storage
- `internal/orchestrator/cache.go:16-18` - Cache key generation
- `internal/orchestrator/cache.go:21-47` - Cache operations (Get/Put)
- `internal/orchestrator/cache.go:56-155` - Eviction logic with priority tiers
- `internal/orchestrator/message.go:5-17` - Message and PlatformRef structures
- `internal/orchestrator/message.go:32-39` - GetResolved() stub

### Database Layer

- `internal/database/database.go:12-34` - Connection management
- `internal/database/database.go:44-50` - Health check
- `internal/database/migrations.go:26-31` - MigrationRunner struct
- `internal/database/migrations.go:44-78` - Migration loading
- `internal/database/migrations.go:178-229` - Migration execution
- `definitions/migrations/m0001_schema.sql:10-127` - Complete schema

### Configuration

- `internal/config/config.go:39-41` - Cache configuration
- `internal/config/config.go:62-114` - Database configuration
- `internal/config/config.go:162-165` - Connection string formatting

### Orchestration

- `internal/orchestrator/orchestrator.go:15-63` - HandleUserMessage() and eviction handling
- `internal/orchestrator/orchestrator.go:72-124` - Action processing loop
- `internal/orchestrator/orchestrator.go:126-147` - Message routing logic
- `internal/orchestrator/action.go:5-17` - Action structure
- `internal/slack/parser.go:6-23` - Message parsing from Slack

## Architecture Documentation

### Data Flow Architecture

```
External Platform (Slack/Teams)
    ↓
ParseMessage() - Creates Message with PlatformRef
    ↓
HandleUserMessage(message)
    ↓
LoadContext(message)
    ├─→ GetFromCache(externalUserID, externalThreadID)
    │   ├─→ [HIT] Return cached context
    │   └─→ [MISS] ↓
    ├─→ loadContextFromDB(message) [STUB]
    │   ├─→ Query by internal IDs (after resolving external IDs)
    │   └─→ [NOT FOUND] ↓
    └─→ Create new ConversationContext
    ↓
PutInCache(userID, threadID, context)
    ├─→ Check if cache full (>= MaxCacheSize)
    └─→ evictToLimit() if needed
        ├─→ Evict Idle contexts (oldest first)
        ├─→ Evict Error contexts
        ├─→ Evict WaitForUser contexts (save to DB first)
        └─→ Emergency: Evict all others if > GraceBufferSize
    ↓
AnalyzeIntent() + ProcessUserIntent()
    ↓
RouteUserMessage()
    ├─→ StatusWaitForUser? → Start new workflow
    ├─→ StatusRunning? → Append actions
    └─→ Default → Start action loop
    ↓
StartHandlingActions(actionQueue, context)
    ├─→ SetCurrentStatus(StatusRunning)
    ├─→ LOOP: Process each action
    │   ├─→ PopRemainingActions() from context
    │   ├─→ Check StatusWaitForUser → Break loop
    │   ├─→ ProcessAction()
    │   └─→ Handle results/errors
    ├─→ On error: SetCurrentStatus(StatusError)
    └─→ On complete: SetCurrentStatus(StatusIdle)
```

### ID Resolution Architecture (Planned)

```
Message arrives with:
  UserID.ExternalID = "U123ABC"
  UserID.Platform = "Slack"
  ThreadID.ExternalID = "C456DEF"
  ThreadID.Platform = "Slack"

    ↓

Cache Lookup:
  cacheKey = "U123ABC:C456DEF"
  GetFromCache(userID, threadID) → uses external IDs

    ↓ [CACHE MISS]

ID Resolution (GetResolved()):
  1. Query user_external_ids:
     SELECT id FROM user_external_ids
     WHERE external_id='U123ABC' AND platform='Slack'

  2. If not found, insert:
     INSERT INTO user_external_ids (external_id, platform)
     VALUES ('U123ABC', 'Slack')
     RETURNING id → e.g., 42

  3. Store: UserID.InternalID = "42"

  4. Repeat for ThreadID → e.g., 789

    ↓

Database Query:
  SELECT * FROM conversation_context
  WHERE user_id=42 AND thread_id=789

    ↓ [NOT FOUND]

Create New Context:
  INSERT INTO conversation_context
  (user_id, thread_id, context_status, created_at, updated_at)
  VALUES (42, 789, 'INITIAL', NOW(), NOW())

    ↓

Load related data:
  - workflow_context (if workflow_context_id not null)
  - workflow_context_data (key-value pairs)
  - ai_conversations (provider-specific data)

    ↓

Populate ConversationContext struct and return
```

### Cache Eviction Priority Flow

```
PutInCache() triggered
    ↓
len(contextCache) >= MaxCacheSize?
    ↓ [YES]
evictToLimit()
    ↓
Categorize all contexts:
    ├─→ Idle: StatusIdle
    ├─→ Error: StatusError
    ├─→ Waiting: StatusWaitForUser
    └─→ Other: StatusRunning, StatusEvicted

Sort each category by lastUpdated (oldest first)

    ↓

TIER 1: Evict Idle contexts
    ├─→ While cache > MaxCacheSize
    └─→ Remove oldest idle contexts

    ↓ [Still over limit?]

TIER 2: Evict Error contexts
    ├─→ While cache > MaxCacheSize
    └─→ Remove oldest error contexts

    ↓ [Still over limit?]

TIER 3: Evict WaitForUser contexts
    ├─→ While cache > MaxCacheSize
    ├─→ Set status = StatusEvicted
    ├─→ TODO: saveContextToDB()
    └─→ Remove from cache

    ↓ [Over GraceBufferSize?]

TIER 4: EMERGENCY - Evict all Others
    ├─→ Set status = StatusEvicted
    ├─→ TODO: saveContextToDB()
    ├─→ Remove from cache
    └─→ Log CRITICAL error
```

### Thread Safety Model

**Context-Level Protection**:
- Each `ConversationContext` has `sync.RWMutex`
- Getters acquire read lock (multiple concurrent reads allowed)
- Setters acquire write lock (exclusive access)
- All mutations update `lastUpdated` timestamp

**Cache-Level Protection**:
- Global `cacheMutex sync.RWMutex` protects cache map
- Independent from context mutexes
- Read lock for lookups
- Write lock for insertions/deletions/evictions

**Lock Hierarchy**:
1. Cache mutex acquired first (cache operations)
2. Context mutex acquired second (field access)
3. No lock inversions observed

### Current Design Patterns

1. **Lazy Resolution**: External IDs used immediately, internal IDs resolved on-demand
2. **Cache-First**: Hot path checks cache before database
3. **Priority-Based Eviction**: Status-aware eviction with idle contexts evicted first
4. **Thread-Safe Access**: RWMutex pattern for concurrent safety
5. **Atomic Operations**: `PopRemainingActions()` atomically retrieves and clears
6. **Status-Driven Routing**: Context status determines message routing
7. **Action Queue Preservation**: Incomplete actions stored in context on block
8. **Multi-Platform Support**: Platform-specific IDs mapped to internal IDs
9. **Key-Value Workflow Data**: Flexible schema for workflow state
10. **Migration-Based Schema**: Checksum-validated migrations for integrity

## Historical Context (from thoughts/)

### Implementation Tracker
**Source**: `thoughts/implementation-tracker.md`

**Context Storage & Caching Design** (lines 61-100):
- Hot/cold storage strategy with in-memory cache and DB persistence
- TTL requirements for cache entries
- AIMessage structure with Role, Content, Timestamp
- Context persistence strategy: what to save vs ephemeral data
- DB layer abstraction questions and decisions
- Open questions on cache TTL, async result storage, persistence frequency

**Key Decisions**:
- Action queue cached in memory, NOT persisted to DB
- Only "cold facts" persisted: workflow state, wait context, AI history
- Minimal persistence to reduce DB load

### Session Summary: Orchestrator Foundation
**Source**: `thoughts/session-summary/2025-12-30-orchestrator-foundation.md`

**Thread-Safe Context with Mutex** (lines 101-152):
- Decision to use `sync.RWMutex` for thread safety
- Getter/setter pattern rationale
- Timestamps for eviction tracking

**Cache Layer with Intelligent Eviction** (lines 262-425):
- Two-tier caching strategy design
- cacheEntry type with TTL tracking
- Cache key format: `userID:threadID`
- Priority-based eviction logic details
- Integration with LoadContext flow

**Configuration Management** (lines 428-470):
- config.go package design
- Environment variable loading
- Default values for cache and DB settings

**Database Layer** (lines 618-623):
- Planned DB implementation details
- Persistence triggers: after AI calls, on ActionUserWait, workflow transitions
- Fields to persist vs fields to exclude

### Next Steps and Architectural Decisions
**Source**: `thoughts/nextSteps.md`

**Core Principles**:
- Action queue caching strategy: hot in memory, not persisted
- Blocking actions and minimal cold facts persistence
- Flat action loop (no recursion)
- Channel-based async communication

**Explicitly Deferred**:
- Complex workflow recovery mechanisms
- Advanced cache warm-up strategies
- Multi-region database replication

## Integration Recommendations

Based on the current architecture, here are observations for adding DB integration:

### 1. ID Resolution Implementation

**Where to implement**: New file `internal/database/id_resolver.go`

**Pattern to follow**:
```go
type IDResolver struct {
    db *sql.DB
}

func (r *IDResolver) ResolveUserID(externalID string, platform PlatformType) (int, error) {
    // 1. Try SELECT first
    var internalID int
    err := r.db.QueryRow(
        "SELECT id FROM user_external_ids WHERE external_id=? AND platform=?",
        externalID, platform,
    ).Scan(&internalID)

    if err == sql.ErrNoRows {
        // 2. Insert if not found
        result, err := r.db.Exec(
            "INSERT INTO user_external_ids (external_id, platform) VALUES (?, ?)",
            externalID, platform,
        )
        if err != nil {
            return 0, err
        }
        id, _ := result.LastInsertId()
        return int(id), nil
    }

    return internalID, err
}

func (r *IDResolver) ResolveThreadID(externalID string, platform PlatformType) (int, error) {
    // Same pattern as ResolveUserID
}
```

**Integration point**: Call from `Message.GetResolved()` at `message.go:32-39`

### 2. Context Persistence Implementation

**Where to implement**: New file `internal/database/context_repository.go`

**Pattern to follow**:
```go
type ContextRepository struct {
    db         *sql.DB
    idResolver *IDResolver
}

func (r *ContextRepository) SaveContext(ctx *ConversationContext, msg *Message) error {
    // 1. Resolve IDs
    userID, _ := r.idResolver.ResolveUserID(msg.UserID.ExternalID, msg.UserID.Platform)
    threadID, _ := r.idResolver.ResolveThreadID(msg.ThreadID.ExternalID, msg.ThreadID.Platform)

    // 2. Save/update conversation_context
    // 3. Save workflow_context if exists
    // 4. Save workflow_context_data key-value pairs
    // 5. Transaction management for atomicity
}

func (r *ContextRepository) LoadContext(msg *Message) (*ConversationContext, error) {
    // 1. Resolve IDs
    // 2. Query conversation_context
    // 3. Load workflow_context
    // 4. Load workflow_context_data
    // 5. Populate ConversationContext struct
}
```

**Integration points**:
- Call `SaveContext()` from `context.UpdateDB()` at `context.go:186-193`
- Call `LoadContext()` from `context.loadContextFromDB()` at `context.go:178-184`
- Call `SaveContext()` before eviction at `cache.go:128, 148`

### 3. Simple Usage Pattern

**For adding new tables**:
1. Create new migration file: `definitions/migrations/mXXXX_description.sql`
2. Run `cmd/migrate/main.go` to apply
3. Add repository pattern file in `internal/database/`
4. Follow existing foreign key patterns with `user_external_ids` and `thread_external_ids`

**For querying by external IDs**:
```go
// Always resolve external IDs first
userID, _ := idResolver.ResolveUserID(externalID, platform)

// Then use internal ID in queries
rows, _ := db.Query("SELECT * FROM some_table WHERE user_id=?", userID)
```

**For caching**:
```go
// Cache uses external IDs
cacheKey := cacheKey(externalUserID, externalThreadID)

// Database uses internal IDs
internalUserID, _ := ResolveUserID(externalUserID, platform)
```

### 4. Not Too Bloated Design

**Existing design strengths**:
- ✅ Single global `DB` connection (no connection-per-request overhead)
- ✅ Connection pooling configured (25 max open, 5 idle)
- ✅ Migration system prevents manual schema changes
- ✅ Repository pattern separation (planned)
- ✅ Lazy ID resolution (only when needed)
- ✅ Cache-first pattern (reduces DB load)

**To maintain simplicity**:
- Keep using `database/sql` standard library (no ORM)
- One repository file per table/domain concept
- Plain SQL queries (readable, debuggable)
- Transaction management only where atomicity required
- Avoid complex query builders

### 5. Easily Addable Pattern

**For new data requirements**:

1. **Create migration**:
   ```bash
   # definitions/migrations/m0002_add_feature.sql
   CREATE TABLE feature_data (
       id INT AUTO_INCREMENT PRIMARY KEY,
       conversation_context_id INT NOT NULL,
       feature_value TEXT,
       FOREIGN KEY (conversation_context_id) REFERENCES conversation_context(id) ON DELETE CASCADE
   );
   ```

2. **Run migration**:
   ```bash
   go run cmd/migrate/main.go run
   ```

3. **Create repository** (`internal/database/feature_repository.go`):
   ```go
   type FeatureRepository struct {
       db *sql.DB
   }

   func (r *FeatureRepository) Save(contextID int, value string) error {
       _, err := r.db.Exec(
           "INSERT INTO feature_data (conversation_context_id, feature_value) VALUES (?, ?)",
           contextID, value,
       )
       return err
   }
   ```

4. **Use in orchestrator**:
   ```go
   // Already have context loaded
   featureRepo := &database.FeatureRepository{db: database.DB}
   featureRepo.Save(contextInternalID, "value")
   ```

## Open Questions

Based on the current implementation and stubs, these areas need clarification:

1. **ID Resolution Timing**:
   - Should `GetResolved()` be called eagerly at message parsing or lazily when saving to DB?
   - Current pattern: Lazy (only when needed for DB operations)
   - Trade-off: Eager adds latency to all messages, lazy defers cost to cache misses

2. **Cache TTL Implementation**:
   - `CacheTTLSeconds` configured but not implemented in eviction logic
   - Should TTL be checked on every cache access or via background job?
   - Should expired contexts be saved to DB before removal?

3. **Action Queue Persistence**:
   - `context.go:190` explicitly excludes ActionQueue from persistence
   - How to handle long-running action queues during eviction?
   - Should critical actions be saved separately?

4. **Async Result Storage**:
   - Where should async action results be stored while waiting for group completion?
   - Current pattern: In-memory action channel (buffer size 100)
   - Should results be persisted if context evicted mid-processing?

5. **Smart Eviction Recovery**:
   - `orchestrator.go:59` notes need for "smarter handling of eviction process"
   - Should evicted contexts be automatically restored on next message?
   - How to communicate state loss vs state restoration to user?

6. **Workflow State Complexity**:
   - `workflow_context_data` uses flexible key-value storage with type hints
   - Should complex workflow state (maps, slices) be JSON-serialized?
   - How to handle schema evolution for workflow data?

7. **Multi-Platform User Identity**:
   - Same user on multiple platforms (Slack + Teams) has separate `user_external_ids` entries
   - Should there be a `users` table linking platform identities?
   - How to handle cross-platform conversation continuity?

8. **Database Transaction Boundaries**:
   - Which operations require transactions?
   - Context save with workflow state should be atomic - use transaction?
   - ID resolution + context insert should be atomic?

9. **Cache Warming Strategy**:
   - Should frequently accessed contexts be preloaded on startup?
   - How to identify "hot" contexts for preferential caching?
   - Should cache misses trigger background preload of related contexts?

10. **Error Context Cleanup**:
    - Error contexts evicted but not marked as resolved
    - Should error contexts expire after threshold (e.g., 24 hours)?
    - How to surface unresolved errors to monitoring/alerts?

## Related Research

This is the initial comprehensive research document for the Bob database layer integration. Future research documents can be linked here as they are created.

## Appendix: File Structure

```
bob/
├── cmd/
│   └── migrate/
│       └── main.go                          # Migration CLI entry point
├── definitions/
│   └── migrations/
│       ├── m0001_schema.sql                 # Initial schema migration
│       └── README.md                        # Migration documentation
├── internal/
│   ├── config/
│   │   └── config.go                        # Configuration management
│   ├── database/
│   │   ├── database.go                      # Connection management
│   │   └── migrations.go                    # Migration runner
│   ├── orchestrator/
│   │   ├── action.go                        # Action definitions
│   │   ├── cache.go                         # Cache implementation
│   │   ├── context.go                       # ConversationContext
│   │   ├── intent.go                        # Intent types
│   │   ├── message.go                       # Message & PlatformRef
│   │   └── orchestrator.go                  # Main orchestrator
│   └── slack/
│       └── parser.go                        # Slack message parsing
├── thoughts/
│   ├── implementation-tracker.md            # Living implementation tracker
│   ├── nextSteps.md                         # Architectural decisions
│   └── session-summary/
│       └── 2025-12-30-orchestrator-foundation.md  # Session summary
└── scripts/
    └── reset_database.sql                   # Dev database reset
```
