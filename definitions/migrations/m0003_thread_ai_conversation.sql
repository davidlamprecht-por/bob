-- Move main AI conversation from workflow_context to conversation_context (thread-level)
ALTER TABLE conversation_context
    ADD COLUMN main_conversation_id VARCHAR(255) NULL COMMENT 'Persistent OpenAI conversation ID for this thread',
    ADD COLUMN last_response_id VARCHAR(255) NULL COMMENT 'Most recent response ID in the main conversation';

ALTER TABLE workflow_context DROP COLUMN main_conversation_id;
