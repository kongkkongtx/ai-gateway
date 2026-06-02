package middleware

import (
	"encoding/json"
	"os"
	"sync"
	"time"
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



// AuditStore interface for pluggable audit log backends.
type AuditStore interface {
	Push(entry AuditEntry)
	Query(limit int, level, keyName, path string, status int) []AuditEntry
	Count() int
}

// MemoryAuditStore is a thread-safe ring buffer for in-memory audit logs.
type MemoryAuditStore struct {
	mu       sync.RWMutex
	entries  []AuditEntry
	head     int
	count    int
	capacity int
}

func NewMemoryAuditStore(capacity int) *MemoryAuditStore {
	if capacity <= 0 {
		capacity = 10000
	}
	return &MemoryAuditStore{
		entries:  make([]AuditEntry, capacity),
		capacity: capacity,
	}
}

func (s *MemoryAuditStore) Push(entry AuditEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[s.head] = entry
	s.head = (s.head + 1) % s.capacity
	if s.count < s.capacity {
		s.count++
	}
}

func (s *MemoryAuditStore) Query(limit int, level, keyName, path string, status int) []AuditEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return queryEntries(s.entries, s.head, s.count, s.capacity, limit, level, keyName, path, status)
}

func (s *MemoryAuditStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.count
}

// FileAuditStore persists audit entries to a JSON-lines file with an
// in-memory query buffer for recent lookups.
type FileAuditStore struct {
	mu       sync.Mutex
	filePath string
	file     *os.File
	buffer   []AuditEntry
	maxBuf   int
}

func NewFileAuditStore(filePath string, bufferSize int) (*FileAuditStore, error) {
	if bufferSize <= 0 {
		bufferSize = 10000
	}
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	s := &FileAuditStore{
		filePath: filePath,
		file:     f,
		buffer:   make([]AuditEntry, 0, bufferSize),
		maxBuf:   bufferSize,
	}
	// Load existing entries into buffer (last bufferSize entries)
	s.loadRecent()
	return s, nil
}

func (s *FileAuditStore) loadRecent() {
	data, err := os.ReadFile(s.filePath)
	if err != nil || len(data) == 0 {
		return
	}
	// Simple line-by-line JSON parsing
	lines := splitLines(string(data))
	start := 0
	if len(lines) > s.maxBuf {
		start = len(lines) - s.maxBuf
	}
	for _, line := range lines[start:] {
		line = trimSpace(line)
		if line == "" {
			continue
		}
		var entry AuditEntry
		if json.Unmarshal([]byte(line), &entry) == nil {
			s.buffer = append(s.buffer, entry)
		}
	}
}

func (s *FileAuditStore) Push(entry AuditEntry) {
	entry.Timestamp = time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	// Persist to file
	data, err := json.Marshal(entry)
	if err == nil {
		s.file.Write(append(data, '\n'))
	}

	// Update in-memory buffer
	s.buffer = append(s.buffer, entry)
	if len(s.buffer) > s.maxBuf {
		s.buffer = s.buffer[len(s.buffer)-s.maxBuf:]
	}
}

func (s *FileAuditStore) Query(limit int, level, keyName, path string, status int) []AuditEntry {
	s.mu.Lock()
	buf := make([]AuditEntry, len(s.buffer))
	copy(buf, s.buffer)
	s.mu.Unlock()

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	result := make([]AuditEntry, 0)
	for i := len(buf) - 1; i >= 0 && len(result) < limit; i-- {
		e := buf[i]
		if !matchEntry(e, level, keyName, path, status) {
			continue
		}
		result = append(result, e)
	}
	return result
}

func (s *FileAuditStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.buffer)
}

func (s *FileAuditStore) Close() error {
	return s.file.Close()
}

// queryEntries walks a ring buffer and returns matching entries (newest first).
func queryEntries(entries []AuditEntry, head, count, capacity, limit int, level, keyName, path string, status int) []AuditEntry {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	result := make([]AuditEntry, 0)
	for i := 0; i < count; i++ {
		idx := (head - 1 - i + capacity) % capacity
		e := entries[idx]
		if !matchEntry(e, level, keyName, path, status) {
			continue
		}
		result = append(result, e)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func matchEntry(e AuditEntry, level, keyName, path string, status int) bool {
	if level != "" && e.Level != level {
		return false
	}
	if keyName != "" && !containsFold(e.APIKeyName, keyName) {
		return false
	}
	if path != "" && !containsFold(e.Path, path) {
		return false
	}
	if status > 0 && e.Status != status {
		return false
	}
	return true
}

func containsFold(s, substr string) bool {
	return len(s) >= len(substr) && containsFoldImpl(s, substr)
}

func containsFoldImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		sc := s[i]
		tc := t[i]
		if sc >= 'A' && sc <= 'Z' {
			sc += 32
		}
		if tc >= 'A' && tc <= 'Z' {
			tc += 32
		}
		if sc != tc {
			return false
		}
	}
	return true
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
