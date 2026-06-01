package middleware

import (
	"sync"
	"time"
	"strings"
)

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	Duration   string    `json:"duration"`
	RemoteAddr string    `json:"remote_addr"`
	APIKeyName string    `json:"api_key_name,omitempty"`
	Level      string    `json:"level"`
	Model      string    `json:"model,omitempty"`
	TokensIn   int       `json:"tokens_in,omitempty"`
	TokensOut  int       `json:"tokens_out,omitempty"`
}

// AuditStore is a thread-safe ring buffer for storing recent audit log entries.
type AuditStore struct {
	mu      sync.RWMutex
	entries []AuditEntry
	head    int
	count   int
	capacity int
}

// NewAuditStore creates an audit store with the given capacity.
func NewAuditStore(capacity int) *AuditStore {
	if capacity <= 0 {
		capacity = 10000
	}
	return &AuditStore{
		entries:  make([]AuditEntry, capacity),
		capacity: capacity,
	}
}

// Push adds an entry to the store.
func (s *AuditStore) Push(entry AuditEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[s.head] = entry
	s.head = (s.head + 1) % s.capacity
	if s.count < s.capacity {
		s.count++
	}
}

// Query returns audit entries matching the given filters, ordered newest first.
func (s *AuditStore) Query(limit int, level string, keyName string, path string, status int) []AuditEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	result := make([]AuditEntry, 0)
	// Iterate from newest to oldest
	for i := 0; i < s.count; i++ {
		idx := (s.head - 1 - i + s.capacity) % s.capacity
		e := s.entries[idx]

		// Apply filters
		if level != "" && e.Level != level {
			continue
		}
		if keyName != "" && !strings.Contains(e.APIKeyName, keyName) {
			continue
		}
		if path != "" && !strings.Contains(e.Path, path) {
			continue
		}
		if status > 0 && e.Status != status {
			continue
		}

		result = append(result, e)
		if len(result) >= limit {
			break
		}
	}
	return result
}



