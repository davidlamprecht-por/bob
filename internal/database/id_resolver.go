package database

import (
	"database/sql"
	"fmt"
)

// IDResolver handles resolution of external platform IDs to internal database IDs
type IDResolver struct {
	db *sql.DB
}

// NewIDResolver creates a new ID resolver
func NewIDResolver(db *sql.DB) *IDResolver {
	return &IDResolver{db: db}
}

// ResolveUserID returns internal ID for external user ID (get-or-create pattern)
// If the external ID doesn't exist, it will be inserted and the new internal ID returned
func (r *IDResolver) ResolveUserID(externalID string, platform string) (int, error) {
	// 1. Try SELECT first
	var internalID int
	err := r.db.QueryRow(
		"SELECT id FROM user_external_ids WHERE external_id=? AND platform=?",
		externalID, platform,
	).Scan(&internalID)

	if err == sql.ErrNoRows {
		// 2. INSERT and get new ID
		result, insertErr := r.db.Exec(
			"INSERT INTO user_external_ids (external_id, platform) VALUES (?, ?)",
			externalID, platform,
		)
		if insertErr != nil {
			return 0, fmt.Errorf("failed to insert user external ID: %w", insertErr)
		}
		id, _ := result.LastInsertId()
		return int(id), nil
	}

	if err != nil {
		return 0, fmt.Errorf("failed to query user external ID: %w", err)
	}

	return internalID, nil
}

// ResolveThreadID returns internal ID for external thread ID (get-or-create pattern)
// If the external ID doesn't exist, it will be inserted and the new internal ID returned
func (r *IDResolver) ResolveThreadID(externalID string, platform string) (int, error) {
	// 1. Try SELECT first
	var internalID int
	err := r.db.QueryRow(
		"SELECT id FROM thread_external_ids WHERE external_id=? AND platform=?",
		externalID, platform,
	).Scan(&internalID)

	if err == sql.ErrNoRows {
		// 2. INSERT and get new ID
		result, insertErr := r.db.Exec(
			"INSERT INTO thread_external_ids (external_id, platform) VALUES (?, ?)",
			externalID, platform,
		)
		if insertErr != nil {
			return 0, fmt.Errorf("failed to insert thread external ID: %w", insertErr)
		}
		id, _ := result.LastInsertId()
		return int(id), nil
	}

	if err != nil {
		return 0, fmt.Errorf("failed to query thread external ID: %w", err)
	}

	return internalID, nil
}

// ResolveBoth resolves both user and thread IDs atomically
// This is a convenience method that resolves both IDs in sequence
func (r *IDResolver) ResolveBoth(userExtID, threadExtID, platform string) (userID, threadID int, err error) {
	userID, err = r.ResolveUserID(userExtID, platform)
	if err != nil {
		return 0, 0, err
	}

	threadID, err = r.ResolveThreadID(threadExtID, platform)
	if err != nil {
		return 0, 0, err
	}

	return userID, threadID, nil
}
