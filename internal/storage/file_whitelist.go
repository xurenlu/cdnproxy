package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type FileWhitelistStore struct {
	filePath string
	mu       sync.RWMutex
	suffixes map[string]bool
}

func NewFileWhitelistStore(dataDir string) (*FileWhitelistStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	store := &FileWhitelistStore{
		filePath: filepath.Join(dataDir, "whitelist.json"),
		suffixes: make(map[string]bool),
	}

	// 加载现有数据
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return store, nil
}

func (s *FileWhitelistStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.suffixes = make(map[string]bool)
			return nil
		}
		return err
	}

	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	s.suffixes = make(map[string]bool)
	for _, suffix := range list {
		s.suffixes[strings.ToLower(strings.TrimSpace(suffix))] = true
	}

	return nil
}

func (s *FileWhitelistStore) save() error {
	list := make([]string, 0, len(s.suffixes))
	for suffix := range s.suffixes {
		if suffix != "" {
			list = append(list, suffix)
		}
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *FileWhitelistStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]string, 0, len(s.suffixes))
	for suffix := range s.suffixes {
		list = append(list, suffix)
	}
	return list, nil
}

func (s *FileWhitelistStore) Add(ctx context.Context, suffix string) error {
	suffix = strings.TrimSpace(strings.ToLower(suffix))
	if suffix == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.suffixes[suffix] = true
	return s.save()
}

func (s *FileWhitelistStore) Remove(ctx context.Context, suffix string) error {
	suffix = strings.TrimSpace(strings.ToLower(suffix))
	if suffix == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.suffixes, suffix)
	return s.save()
}

func (s *FileWhitelistStore) ContainsAllowedSuffix(ctx context.Context, host string) (bool, error) {
	host = strings.ToLower(host)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for suffix := range s.suffixes {
		if suffix != "" && strings.HasSuffix(host, suffix) {
			return true, nil
		}
	}
	return false, nil
}

