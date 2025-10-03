package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	fileDefaultThreshold = int64(1000)
)

type FileConfigStore struct {
	filePath  string
	mu        sync.RWMutex
	threshold int64
}

func NewFileConfigStore(dataDir string) (*FileConfigStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	store := &FileConfigStore{
		filePath:  filepath.Join(dataDir, "config.json"),
		threshold: fileDefaultThreshold,
	}

	// 加载现有数据
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return store, nil
}

func (s *FileConfigStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.threshold = fileDefaultThreshold
			return nil
		}
		return err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if threshold, ok := config["referrer_threshold"].(float64); ok {
		s.threshold = int64(threshold)
	}

	return nil
}

func (s *FileConfigStore) save() error {
	config := map[string]interface{}{
		"referrer_threshold": s.threshold,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *FileConfigStore) GetReferrerThreshold(ctx context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.threshold, nil
}

func (s *FileConfigStore) SetReferrerThreshold(ctx context.Context, n int64) error {
	if n <= 0 {
		n = fileDefaultThreshold
	}

	s.mu.Lock()
	s.threshold = n
	s.mu.Unlock()

	return s.save()
}

// FileCounterStore 基于文件的计数器存储
type FileCounterStore struct {
	filePath string
	mu       sync.RWMutex
	counters map[string]*counterEntry
}

type counterEntry struct {
	Count     int64     `json:"count"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewFileCounterStore(dataDir string) (*FileCounterStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	store := &FileCounterStore{
		filePath: filepath.Join(dataDir, "counters.json"),
		counters: make(map[string]*counterEntry),
	}

	// 加载现有数据
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// 启动清理过期数据的 goroutine
	go store.cleanupExpired()

	return store, nil
}

func (s *FileCounterStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.counters = make(map[string]*counterEntry)
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, &s.counters); err != nil {
		return err
	}

	// 清理已过期的条目
	now := time.Now()
	for host, entry := range s.counters {
		if now.After(entry.ExpiresAt) {
			delete(s.counters, host)
		}
	}

	return nil
}

func (s *FileCounterStore) save() error {
	data, err := json.MarshalIndent(s.counters, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *FileCounterStore) IncrementReferrerCount(ctx context.Context, host string) (int64, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entry, exists := s.counters[host]

	if !exists || now.After(entry.ExpiresAt) {
		// 创建新条目，24小时过期
		entry = &counterEntry{
			Count:     1,
			ExpiresAt: now.Add(24 * time.Hour),
		}
		s.counters[host] = entry
	} else {
		entry.Count++
	}

	// 每10次增量保存一次，减少IO
	if entry.Count%10 == 0 {
		_ = s.save()
	}

	return entry.Count, nil
}

func (s *FileCounterStore) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		changed := false

		for host, entry := range s.counters {
			if now.After(entry.ExpiresAt) {
				delete(s.counters, host)
				changed = true
			}
		}

		if changed {
			_ = s.save()
		}
		s.mu.Unlock()
	}
}

