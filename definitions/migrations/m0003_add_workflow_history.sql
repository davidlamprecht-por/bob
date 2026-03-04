ALTER TABLE conversation_context
  ADD COLUMN workflow_history TEXT NULL;

ALTER TABLE workflow_context
  ADD COLUMN last_response_id VARCHAR(255) NULL;
