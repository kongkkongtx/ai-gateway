package user

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store provides persistent user storage backed by a JSON file.
type Store struct {
	mu    sync.RWMutex
	path  string
	users map[string]*User
}

// NewStore creates a new user store. Creates default admin if none exists.
func NewStore(path string) (*Store, error) {
	s := &Store{path: path, users: make(map[string]*User)}
	if _, err := os.Stat(path); err == nil {
		if err := s.load(); err != nil {
			return nil, fmt.Errorf("load users: %w", err)
		}
	}
	if _, ok := s.users["admin"]; !ok {
		hash, _ := HashPassword("admin123")
		s.users["admin"] = &User{
			Username: "admin", PasswordHash: hash,
			Role: "admin", Team: "default", CreatedAt: time.Now(),
		}
		if err := s.save(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Authenticate checks credentials and returns the user if valid.
func (s *Store) Authenticate(username, password string) (*User, error) {
	s.mu.RLock()
	u, ok := s.users[username]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrInvalidCredentials
	}
	if !CheckPassword(password, u.PasswordHash) {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

// List returns all users (without password hashes in API responses).
func (s *Store) List() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		result = append(result, u)
	}
	return result
}

// Create adds a new user with hashed password.
func (s *Store) Create(username, password, role, team string) (*User, error) {
	if len(password) < 8 {
		return nil, ErrWeakPassword
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[username]; ok {
		return nil, ErrUserExists
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	u := &User{
		Username: username, PasswordHash: hash,
		Role: role, Team: team, CreatedAt: time.Now(),
	}
	s.users[username] = u
	if err := s.save(); err != nil {
		delete(s.users, username)
		return nil, err
	}
	return u, nil
}

// Delete removes a user. Prevents self-delete and last-admin delete.
func (s *Store) Delete(username, requester string) error {
	if username == requester {
		return ErrCannotDeleteSelf
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[username]
	if !ok {
		return ErrUserNotFound
	}
	if u.Role == "admin" {
		adminCount := 0
		for _, u2 := range s.users {
			if u2.Role == "admin" {
				adminCount++
			}
		}
		if adminCount <= 1 {
			return fmt.Errorf("cannot delete the last admin user")
		}
	}
	delete(s.users, username)
	return s.save()
}

// UpdateRole changes a user's role.
func (s *Store) UpdateRole(username, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[username]
	if !ok {
		return ErrUserNotFound
	}
	u.Role = role
	return s.save()
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.users)
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.users, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}
