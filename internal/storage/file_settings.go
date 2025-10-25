package storage

import (
	"context"
	"encoding/json"
	"errors"
	"log"
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

	// 原子写入：先写临时文件，再重命名
	tempFile := s.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tempFile, s.filePath)
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
	dirty    bool          // 标记是否有未保存的更改
	stopCh   chan struct{} // 用于停止后台goroutine
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
		dirty:    false,
		stopCh:   make(chan struct{}),
	}

	// 加载现有数据
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// 启动后台工作协程
	go store.backgroundWorker()

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

	// 原子写入：先写临时文件，再重命名
	tempFile := s.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tempFile, s.filePath)
}

func (s *FileCounterStore) IncrementReferrerCount(ctx context.Context, host string) (int64, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 防止恶意请求导致map无限增长
	const maxCounters = 100000 // 最多10万个不同的host
	if len(s.counters) >= maxCounters {
		// 清理过期条目
		now := time.Now()
		cleaned := 0
		for h, e := range s.counters {
			if now.After(e.ExpiresAt) {
				delete(s.counters, h)
				cleaned++
			}
		}
		log.Printf("Counter store reached limit (%d), cleaned %d expired entries", maxCounters, cleaned)

		// 如果清理后还是太多，拒绝新增
		if len(s.counters) >= maxCounters {
			return 0, errors.New("counter store full")
		}
	}

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

	s.dirty = true // 标记为脏数据，由后台异步保存

	return entry.Count, nil
}

// backgroundWorker 后台工作协程：定期清理过期数据和异步保存
func (s *FileCounterStore) backgroundWorker() {
	cleanupTicker := time.NewTicker(1 * time.Hour) // 清理过期计数器
	saveTicker := time.NewTicker(10 * time.Second) // 定期保存脏数据（计数器更新频繁，10秒保存一次）
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
			// 清理过期计数器
			s.mu.Lock()
			now := time.Now()
			for host, entry := range s.counters {
				if now.After(entry.ExpiresAt) {
					delete(s.counters, host)
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
func (s *FileCounterStore) Close() error {
	close(s.stopCh)
	time.Sleep(100 * time.Millisecond) // 等待goroutine退出
	return nil
}
