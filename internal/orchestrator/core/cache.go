package core

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"bob/internal/config"
	"bob/internal/logger"
)

var contextCache = make(map[string]*ConversationContext)
var cacheMutex sync.RWMutex

// cacheKey generates unique key for user+thread
func cacheKey(userID, threadID int) string {
	return fmt.Sprintf("%d:%d", userID, threadID)
}

// GetFromCache retrieves context (thread-safe)
func GetFromCache(userID, threadID int) *ConversationContext {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	if entry, ok := contextCache[cacheKey(userID, threadID)]; ok {
		return entry
	}
	return nil
}

// PutInCache stores context with eviction logic (thread-safe)
func PutInCache(userID, threadID int, ctx *ConversationContext) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	key := cacheKey(userID, threadID)

	// Check if over hard limit - evict down to limit
	if len(contextCache) >= config.Current.MaxCacheSize {
		evictToLimit()
	}

	if ctx.createdAt.IsZero() {
		ctx.createdAt = time.Now()
	}
	contextCache[key] = ctx
}

// RemoveFromCache deletes context (thread-safe)
func RemoveFromCache(userID, threadID int) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	delete(contextCache, cacheKey(userID, threadID))
}

// evictToLimit evicts Idle/Error contexts until we're at MaxCacheSize
func evictToLimit() {
	// Collect eviction candidates by priority
	type candidate struct {
		key      string
		entry    *ConversationContext
		lastUsed time.Time
	}

	idleCandidates := []candidate{}
	errorCandidates := []candidate{}
	waitingCandidates := []candidate{}
	otherCandidates := []candidate{}

	for key, entry := range contextCache {
		status := entry.GetCurrentStatus()
		lastUsed := entry.GetLastUpdated()

		c := candidate{
			key:      key,
			entry:    entry,
			lastUsed: lastUsed,
		}

		switch status {
		case StatusIdle:
			idleCandidates = append(idleCandidates, c)
		case StatusError:
			errorCandidates = append(errorCandidates, c)
		case StatusWaitForUser:
			waitingCandidates = append(waitingCandidates, c)
		default:
			otherCandidates = append(otherCandidates, c)
		}
	}

	// Sort by lastUsed (oldest first)
	sortByLastUsed := func(candidates []candidate) {
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].lastUsed.Before(candidates[j].lastUsed)
		})
	}

	sortByLastUsed(idleCandidates)
	sortByLastUsed(errorCandidates)
	sortByLastUsed(waitingCandidates)
	sortByLastUsed(otherCandidates)

	// Evict Idle first
	for _, c := range idleCandidates {
		if len(contextCache) <= config.Current.MaxCacheSize {
			return
		}
		delete(contextCache, c.key)
		logger.Debugf("Cache: Evicted idle context: %s", c.key)
	}

	// Then Error
	for _, c := range errorCandidates {
		if len(contextCache) <= config.Current.MaxCacheSize {
			return
		}
		delete(contextCache, c.key)
		logger.Debugf("Cache: Evicted error context: %s", c.key)
	}

	// Then WaitForUser (mark as evicted, save to DB)
	for _, c := range waitingCandidates {
		if len(contextCache) <= config.Current.MaxCacheSize {
			return
		}
		c.entry.SetCurrentStatus(StatusEvicted)

		// Save to DB before eviction
		if err := c.entry.UpdateDB(); err != nil {
			logger.Errorf("Failed to save evicted WaitForUser context to DB: %v (key: %s)", err, c.key)
		}

		delete(contextCache, c.key)
		logger.Warnf("High traffic - evicted StatusWaitForUser context: %s", c.key)
	}

	// Check if over grace buffer - emergency eviction
	if len(contextCache) >= config.Current.GraceBufferSize {
		logger.Errorf("CRITICAL: Cache hit grace buffer (%d contexts), emergency eviction starting", len(contextCache))

		// Evict until under grace buffer
		for _, c := range otherCandidates {
			if len(contextCache) < config.Current.GraceBufferSize {
				logger.Warnf("Emergency eviction complete - cache size: %d", len(contextCache))
				return
			}

			status := c.entry.GetCurrentStatus()
			if status != StatusIdle && status != StatusError {
				// Mark as evicted for non-idle/error contexts
				c.entry.SetCurrentStatus(StatusEvicted)

				// Save to DB before eviction
				if err := c.entry.UpdateDB(); err != nil {
					logger.Errorf("Failed to save emergency evicted context to DB: %v (key: %s)", err, c.key)
				}

				logger.Errorf("Emergency evicted %s context: %s", status, c.key)
			}

			delete(contextCache, c.key)
		}
	}
}

