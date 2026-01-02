-- ============================================
-- BOB Database Initial Schema
-- Migration: m0001_schema
-- Description: Initial database schema for BOB conversation management
-- ============================================

-- ============================================
-- User External IDs (Multi-platform support)
-- ============================================
CREATE TABLE IF NOT EXISTS user_external_ids (
    id INT AUTO_INCREMENT PRIMARY KEY,
    external_id VARCHAR(100) NOT NULL COMMENT 'Platform-specific user identifier',
    platform VARCHAR(50) NOT NULL COMMENT 'Platform name: slack, teams, etc.',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE INDEX idx_external_platform (external_id, platform),
    INDEX idx_platform (platform)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- Thread External IDs (Multi-platform support)
-- ============================================
CREATE TABLE IF NOT EXISTS thread_external_ids (
    id INT AUTO_INCREMENT PRIMARY KEY,
    external_id VARCHAR(255) NOT NULL COMMENT 'Platform-specific thread identifier',
    platform VARCHAR(50) NOT NULL COMMENT 'Platform name: slack, teams, etc.',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE INDEX idx_external_platform (external_id, platform),
    INDEX idx_platform (platform)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- Workflow Context (Workflow state tracking)
-- ============================================
CREATE TABLE IF NOT EXISTS workflow_context (
    id INT AUTO_INCREMENT PRIMARY KEY,
    workflow_name VARCHAR(100) NOT NULL,
    workflow_step VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_workflow_name (workflow_name),
    INDEX idx_workflow_step (workflow_name, workflow_step)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- Workflow Context Data (Key-value store for workflow state)
-- ============================================
CREATE TABLE IF NOT EXISTS workflow_context_data (
    id INT AUTO_INCREMENT PRIMARY KEY,
    workflow_context_id INT NOT NULL,
    `key` VARCHAR(100) NOT NULL COMMENT 'Key name for the workflow data',
    value TEXT COMMENT 'Value stored as text (can be JSON string, number, etc.)',
    data_type VARCHAR(20) DEFAULT 'string' COMMENT 'Data type hint: string, json, int, timestamp, boolean',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    FOREIGN KEY (workflow_context_id)
        REFERENCES workflow_context(id)
        ON DELETE CASCADE,

    UNIQUE INDEX idx_workflow_unique_key (workflow_context_id, `key`),
    INDEX idx_key (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- Conversation Context (Main conversation tracking)
-- ============================================
CREATE TABLE IF NOT EXISTS conversation_context (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    thread_id INT NOT NULL,
    workflow_context_id INT NULL COMMENT 'Current active workflow',
    context_status VARCHAR(50) NOT NULL DEFAULT 'INITIAL' COMMENT 'Status: INITIAL, ACTIVE, WAITING, COMPLETED, FAILED, etc.',
    request_to_user TEXT NULL COMMENT 'Last question/request sent to user',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id)
        REFERENCES user_external_ids(id)
        ON DELETE RESTRICT,
    FOREIGN KEY (thread_id)
        REFERENCES thread_external_ids(id)
        ON DELETE RESTRICT,
    FOREIGN KEY (workflow_context_id)
        REFERENCES workflow_context(id)
        ON DELETE SET NULL,

    UNIQUE INDEX idx_user_thread (user_id, thread_id),
    INDEX idx_context_status (context_status),
    INDEX idx_workflow_context (workflow_context_id),
    INDEX idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- AI Conversations (Multiple AI provider conversations per context)
-- ============================================
CREATE TABLE IF NOT EXISTS ai_conversations (
    id INT AUTO_INCREMENT PRIMARY KEY,
    conversation_context_id INT NOT NULL,
    provider VARCHAR(50) NOT NULL DEFAULT 'openai' COMMENT 'AI provider: openai, anthropic, etc.',
    provider_conversation_id VARCHAR(255) NOT NULL COMMENT 'Provider-specific conversation ID',
    metadata JSON NULL COMMENT 'Provider-specific metadata',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    FOREIGN KEY (conversation_context_id)
        REFERENCES conversation_context(id)
        ON DELETE CASCADE,

    INDEX idx_conversation_context (conversation_context_id),
    INDEX idx_provider (provider, provider_conversation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- Migrations (Track applied migrations)
-- ============================================
CREATE TABLE IF NOT EXISTS migrations (
    id INT AUTO_INCREMENT PRIMARY KEY,
    migration_name VARCHAR(255) UNIQUE NOT NULL COMMENT 'Migration filename, e.g., m0001_schema',
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(64) NULL COMMENT 'SHA256 of migration file for integrity check',
    execution_time_ms INT NULL COMMENT 'Migration execution time in milliseconds',

    INDEX idx_applied_at (applied_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- Insert initial migration record
-- ============================================
INSERT INTO migrations (migration_name, checksum)
VALUES ('m0001_schema', SHA2('m0001_schema', 256))
ON DUPLICATE KEY UPDATE applied_at = applied_at;
