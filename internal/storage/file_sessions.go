package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileSessionStore 基于文件的会话存储
type FileSessionStore struct {
	filePath string
	mu       sync.RWMutex
	sessions map[string]*sessionData
	ttl      time.Duration
}

type sessionData struct {
	Value     string    `json:"value"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewFileSessionStore(dataDir string, ttl time.Duration) (*FileSessionStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	store := &FileSessionStore{
		filePath: filepath.Join(dataDir, "sessions.json"),
		sessions: make(map[string]*sessionData),
		ttl:      ttl,
	}

	// 加载现有数据
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// 启动清理过期会话的 goroutine
	go store.cleanupExpired()

	return store, nil
}

func (s *FileSessionStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.sessions = make(map[string]*sessionData)
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, &s.sessions); err != nil {
		return err
	}

	// 清理已过期的会话
	now := time.Now()
	for token, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, token)
		}
	}

	return nil
}

func (s *FileSessionStore) save() error {
	data, err := json.MarshalIndent(s.sessions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// Set 创建或更新会话
func (s *FileSessionStore) Set(token, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[token] = &sessionData{
		Value:     value,
		ExpiresAt: time.Now().Add(s.ttl),
	}

	return s.save()
}

// Exists 检查会话是否存在且未过期
func (s *FileSessionStore) Exists(token string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[token]
	if !exists {
		return false
	}

	return time.Now().Before(session.ExpiresAt)
}

// Delete 删除会话
func (s *FileSessionStore) Delete(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, token)
	return s.save()
}

func (s *FileSessionStore) cleanupExpired() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		changed := false

		for token, session := range s.sessions {
			if now.After(session.ExpiresAt) {
				delete(s.sessions, token)
				changed = true
			}
		}

		if changed {
			_ = s.save()
		}
		s.mu.Unlock()
	}
}

