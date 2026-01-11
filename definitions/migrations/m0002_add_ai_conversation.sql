-- Migration: Add ai_conversation column to workflow_context table
-- This column stores the main AI conversation ID for persistence across messages

ALTER TABLE workflow_context 
ADD COLUMN ai_conversation VARCHAR(255) NULL 
COMMENT 'Main AI conversation ID for persistent conversations';
