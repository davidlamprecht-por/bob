package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

type WorkflowRepository struct {
	db *sql.DB
}

func NewWorkflowRepository(db *sql.DB) *WorkflowRepository {
	return &WorkflowRepository{db: db}
}

// WorkflowContext mirrors orchestrator.WorkflowContext to avoid import cycles
type WorkflowContext struct {
	ID                 *int
	WorkflowName       string
	Step               string
	MainConversationID *string
	WorkflowData       map[string]any
}

func (r *WorkflowRepository) SaveWorkflow(tx *sql.Tx, workflow *WorkflowContext) (int, error) {
	var resultID int

	// If we already have a transaction, use it directly
	if tx != nil {
		var err error
		resultID, err = r.saveWorkflowInTx(tx, workflow)
		return resultID, err
	}

	// Otherwise, create a transaction using WithTransaction helper
	err := WithTransaction(r.db, func(tx *sql.Tx) error {
		var err error
		resultID, err = r.saveWorkflowInTx(tx, workflow)
		return err
	})

	return resultID, err
}

// saveWorkflowInTx does the actual save/update logic within a transaction
func (r *WorkflowRepository) saveWorkflowInTx(tx *sql.Tx, workflow *WorkflowContext) (int, error) {
	var workflowID int

	// Treat both nil and 0 as "not yet in database" since 0 is not a valid auto-increment ID
	if workflow.ID == nil || *workflow.ID == 0 {
		// INSERT new workflow_context
		result, err := tx.Exec(
			"INSERT INTO workflow_context (workflow_name, workflow_step, main_conversation_id) VALUES (?, ?, ?)",
			workflow.WorkflowName, workflow.Step, toNullString(workflow.MainConversationID),
		)
		if err != nil {
			return 0, fmt.Errorf("failed to insert workflow context: %w", err)
		}
		id, _ := result.LastInsertId()
		workflowID = int(id)
	} else {
		// UPDATE existing workflow_context
		_, err := tx.Exec(
			"UPDATE workflow_context SET workflow_name=?, workflow_step=?, main_conversation_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
			workflow.WorkflowName, workflow.Step, toNullString(workflow.MainConversationID), *workflow.ID,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to update workflow context: %w", err)
		}
		workflowID = *workflow.ID

		// Delete old workflow_context_data entries (CASCADE would handle this, but we do it explicitly for clarity)
		_, err = tx.Exec("DELETE FROM workflow_context_data WHERE workflow_context_id=?", workflowID)
		if err != nil {
			return 0, fmt.Errorf("failed to delete old workflow data: %w", err)
		}
	}

	// Insert workflow_context_data entries (for both INSERT and UPDATE cases)
	for key, value := range workflow.WorkflowData {
		dataType, valueStr := serializeValue(value)
		_, err := tx.Exec(
			"INSERT INTO workflow_context_data (workflow_context_id, `key`, value, data_type) VALUES (?, ?, ?, ?)",
			workflowID, key, valueStr, dataType,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to insert workflow data for key %s: %w", key, err)
		}
	}

	return workflowID, nil
}

// LoadWorkflow loads workflow context and data by workflow_context_id
func (r *WorkflowRepository) LoadWorkflow(workflowID int) (workflow *WorkflowContext, err error) {
	workflow = &WorkflowContext{}

	// Load workflow_context
	var mainConvIDNull sql.NullString
	err = r.db.QueryRow(
		"SELECT workflow_name, workflow_step, main_conversation_id FROM workflow_context WHERE id=?",
		workflowID,
	).Scan(&workflow.WorkflowName, &workflow.Step, &mainConvIDNull)

	if err != nil {
		return nil, fmt.Errorf("failed to load workflow context: %w", err)
	}

	// Set the ID so the workflow knows its database ID for future updates
	workflow.ID = &workflowID

	// Set main conversation ID if present
	if mainConvIDNull.Valid {
		workflow.MainConversationID = &mainConvIDNull.String
	}

	// Load workflow_context_data
	rows, err := r.db.Query(
		"SELECT `key`, value, data_type FROM workflow_context_data WHERE workflow_context_id=?",
		workflowID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow data: %w", err)
	}
	defer rows.Close()

	workflow.WorkflowData = make(map[string]any)
	for rows.Next() {
		var key, valueStr, dataType string
		if err := rows.Scan(&key, &valueStr, &dataType); err != nil {
			return nil, fmt.Errorf("failed to scan workflow data: %w", err)
		}
		workflow.WorkflowData[key] = deserializeValue(valueStr, dataType)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating workflow data: %w", err)
	}

	return workflow, nil
}

// serializeValue converts Go value to string + type hint
func serializeValue(value any) (dataType, valueStr string) {
	switch v := value.(type) {
	case string:
		return "string", v
	case int:
		return "int", fmt.Sprintf("%d", v)
	case int64:
		return "int", fmt.Sprintf("%d", v)
	case int32:
		return "int", fmt.Sprintf("%d", v)
	case float64:
		return "float", fmt.Sprintf("%f", v)
	case float32:
		return "float", fmt.Sprintf("%f", v)
	case bool:
		return "boolean", fmt.Sprintf("%t", v)
	default:
		// Complex types -> JSON
		jsonBytes, _ := json.Marshal(v)
		return "json", string(jsonBytes)
	}
}

// deserializeValue converts string + type hint back to Go value
func deserializeValue(valueStr, dataType string) any {
	switch dataType {
	case "string":
		return valueStr
	case "int":
		var i int
		fmt.Sscanf(valueStr, "%d", &i)
		return i
	case "float":
		var f float64
		fmt.Sscanf(valueStr, "%f", &f)
		return f
	case "boolean":
		return valueStr == "true"
	case "json":
		var result any
		json.Unmarshal([]byte(valueStr), &result)
		return result
	default:
		return valueStr
	}
}

// toNullString converts *string to sql.NullString for database operations
func toNullString(ptr *string) sql.NullString {
	if ptr == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *ptr, Valid: true}
}
