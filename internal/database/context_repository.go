package database

import (
	"database/sql"
	"fmt"
	"time"
)

// ContextRepository handles persistence of conversation contexts
type ContextRepository struct {
	db           *sql.DB
	workflowRepo *WorkflowRepository
}

// NewContextRepository creates a new context repository
func NewContextRepository(db *sql.DB) *ContextRepository {
	return &ContextRepository{
		db:           db,
		workflowRepo: NewWorkflowRepository(db),
	}
}

// Context mirrors orchestrator.Context to avoid import cycles
type Context struct {
	UserID        int // Internal DB ID
	ThreadID      int // Internal DB ID
	Workflow      *WorkflowContext
	ContextStatus string // Parsed by orchestrator
	RequestToUser string
}

// SaveContext persists ConversationContext to database
// Accepts internal IDs (already resolved) - no redundant ID resolution
// Returns updated workflow.ID (may have changed if workflow was saved)
func (r *ContextRepository) SaveContext(context *Context) (*int, error) {
	var updatedWorkflowID *int
	err := WithTransaction(r.db, func(tx *sql.Tx) error {
		// 1. Save or update workflow if exists
		if context.Workflow != nil {
			wfID, err := r.workflowRepo.SaveWorkflow(tx, context.Workflow)
			if err != nil {
				return fmt.Errorf("failed to save workflow: %w", err)
			}
			updatedWorkflowID = &wfID
		}

		// 2. Check if conversation_context already exists
		var existingID int
		err := tx.QueryRow(
			"SELECT id FROM conversation_context WHERE user_id=? AND thread_id=?",
			context.UserID, context.ThreadID,
		).Scan(&existingID)

		if err == sql.ErrNoRows {
			// INSERT new conversation_context
			_, err = tx.Exec(`
				INSERT INTO conversation_context
				(user_id, thread_id, workflow_context_id, context_status, request_to_user, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				context.UserID, context.ThreadID, toNullInt64(updatedWorkflowID), context.ContextStatus, nullString(context.RequestToUser),
				time.Now(), time.Now(),
			)
			if err != nil {
				return fmt.Errorf("failed to insert conversation context: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to check existing context: %w", err)
		} else {
			// UPDATE existing conversation_context
			_, err = tx.Exec(`
				UPDATE conversation_context
				SET workflow_context_id=?, context_status=?, request_to_user=?, updated_at=?
				WHERE id=?`,
				toNullInt64(updatedWorkflowID), context.ContextStatus, nullString(context.RequestToUser), time.Now(), existingID,
			)
			if err != nil {
				return fmt.Errorf("failed to update conversation context: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return updatedWorkflowID, nil
}

// LoadContext retrieves ConversationContext from database by internal IDs
// Returns workflowDBID so we can UPDATE it later instead of always INSERTing
func (r *ContextRepository) LoadContext(
	userID, threadID int,
) (context *Context, err error) {
	// Load conversation_context
	var workflowIDNull sql.NullInt64
	var requestToUserNull sql.NullString
	var contextStatus string

	err = r.db.QueryRow(`
		SELECT workflow_context_id, context_status, request_to_user
		FROM conversation_context
		WHERE user_id=? AND thread_id=?`,
		userID, threadID,
	).Scan(&workflowIDNull, &contextStatus, &requestToUserNull)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation context: %w", err)
	}

	context = &Context{
		UserID: userID,
		ThreadID: threadID,
		ContextStatus: contextStatus,
	}

	if requestToUserNull.Valid {
		context.RequestToUser = requestToUserNull.String
	}

	// Load workflow if exists
	if workflowIDNull.Valid {
		wfID := int(workflowIDNull.Int64)

		workflow, err := r.workflowRepo.LoadWorkflow(wfID)
		if err != nil {
			return nil, fmt.Errorf("failed to load workflow: %w", err)
		}
		context.Workflow = workflow
	}

	return context, nil
}

// Helper functions

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func toNullInt64(ptr *int) sql.NullInt64 {
	if ptr == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*ptr), Valid: true}
}
