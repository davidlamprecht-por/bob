-- ============================================
-- Database Reset Script
-- WARNING: This script drops ALL tables!
-- Only use in development environments!
-- ============================================

SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS ai_conversations;
DROP TABLE IF EXISTS conversation_context;
DROP TABLE IF EXISTS workflow_context_data;
DROP TABLE IF EXISTS workflow_context;
DROP TABLE IF EXISTS thread_external_ids;
DROP TABLE IF EXISTS user_external_ids;
DROP TABLE IF EXISTS migrations;

SET FOREIGN_KEY_CHECKS = 1;

-- ============================================
-- After running this script, run migrations:
-- make migrate
-- ============================================
