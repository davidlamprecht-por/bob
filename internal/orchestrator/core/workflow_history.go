package core

import "time"

// WorkflowHistoryEntry records a completed workflow run for the thread.
type WorkflowHistoryEntry struct {
	WorkflowName   string    `json:"workflow_name"`
	TriggerMessage string    `json:"trigger_message"` // first user message that started it
	Summary        string    `json:"summary"`          // 1-sentence AI summary, may be empty
	CompletedAt    time.Time `json:"completed_at"`
}

const maxWorkflowHistory = 10
