-- ============================================
-- BOB Database Migration
-- Migration: m0002_add_main_conversation_id
-- Description: Add main_conversation_id column to workflow_context table
-- ============================================

ALTER TABLE workflow_context
ADD COLUMN main_conversation_id VARCHAR(255) NULL COMMENT 'Main AI conversation ID for this workflow'
AFTER workflow_step;

-- Add index for faster lookups
CREATE INDEX idx_main_conversation ON workflow_context(main_conversation_id);

-- ============================================
-- Insert migration record
-- ============================================
INSERT INTO migrations (migration_name, checksum)
VALUES ('m0002_add_main_conversation_id', SHA2('m0002_add_main_conversation_id', 256))
ON DUPLICATE KEY UPDATE applied_at = applied_at;
