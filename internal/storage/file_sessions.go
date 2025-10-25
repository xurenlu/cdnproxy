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
	dirty    bool           // 标记是否有未保存的更改
	stopCh   chan struct{}  // 用于停止后台goroutine
	wg       sync.WaitGroup // 等待后台goroutine退出
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
		dirty:    false,
		stopCh:   make(chan struct{}),
	}

	// 加载现有数据
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// 启动定期清理过期会话和异步保存的 goroutine
	store.wg.Add(1)
	go store.backgroundWorker()

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

	// 原子写入：先写临时文件，再重命名
	tempFile := s.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tempFile, s.filePath)
}

// Set 创建或更新会话
func (s *FileSessionStore) Set(token, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[token] = &sessionData{
		Value:     value,
		ExpiresAt: time.Now().Add(s.ttl),
	}
	s.dirty = true // 标记为脏数据，等待后台异步保存

	return nil
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
	s.dirty = true // 标记为脏数据

	return nil
}

// backgroundWorker 后台工作协程：定期清理过期数据和异步保存
func (s *FileSessionStore) backgroundWorker() {
	defer s.wg.Done() // 确保退出时通知WaitGroup

	cleanupTicker := time.NewTicker(10 * time.Minute) // 清理过期会话
	saveTicker := time.NewTicker(5 * time.Second)     // 定期保存脏数据
	defer cleanupTicker.Stop()
	defer saveTicker.Stop()

	for {
		select {
		case <-s.stopCh:
			// 收到停止信号，保存最后的数据并退出
			s.mu.Lock()
			if s.dirty {
				_ = s.save()
			}
			s.mu.Unlock()
			return

		case <-cleanupTicker.C:
			// 清理过期会话
			s.mu.Lock()
			now := time.Now()
			for token, session := range s.sessions {
				if now.After(session.ExpiresAt) {
					delete(s.sessions, token)
					s.dirty = true
				}
			}
			s.mu.Unlock()

		case <-saveTicker.C:
			// 定期保存脏数据
			s.mu.Lock()
			if s.dirty {
				if err := s.save(); err == nil {
					s.dirty = false
				}
			}
			s.mu.Unlock()
		}
	}
}

// Close 停止后台goroutine并保存数据
func (s *FileSessionStore) Close() error {
	close(s.stopCh)
	s.wg.Wait() // 等待goroutine真正退出
	return nil
}
