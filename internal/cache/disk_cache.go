package cache

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DiskCache 硬盘缓存，用于存储大文件
type DiskCache struct {
	basePath string
	maxSize  int64        // 最大文件大小（字节）
	mu       sync.RWMutex // 读写锁保护并发访问
}

// NewDiskCache 创建硬盘缓存实例
func NewDiskCache(basePath string, maxSize int64) (*DiskCache, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}
	return &DiskCache{
		basePath: basePath,
		maxSize:  maxSize,
		mu:       sync.RWMutex{},
	}, nil
}

// Get 从硬盘缓存获取文件
func (d *DiskCache) Get(ctx context.Context, key string) (*Entry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	filePath := d.getFilePath(key)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, nil
	}

	// 读取文件
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 解码Entry
	var entry Entry
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&entry); err != nil {
		return nil, err
	}

	// 检查文件是否过期
	if time.Since(entry.StoredAt) > 24*time.Hour {
		// 删除过期文件
		os.Remove(filePath)
		return nil, nil
	}

	return &entry, nil
}

// Set 将文件存储到硬盘缓存
func (d *DiskCache) Set(ctx context.Context, key string, entry *Entry, ttl time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// 检查文件大小
	if int64(len(entry.Body)) > d.maxSize {
		return errors.New("file too large for disk cache")
	}

	filePath := d.getFilePath(key)

	// 创建目录
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	// 写入文件
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 编码并写入
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(entry); err != nil {
		return err
	}

	return nil
}

// Delete 删除硬盘缓存文件
func (d *DiskCache) Delete(ctx context.Context, key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	filePath := d.getFilePath(key)
	return os.Remove(filePath)
}

// getFilePath 根据key生成文件路径
func (d *DiskCache) getFilePath(key string) string {
	// 使用key的hash作为文件名，避免路径冲突
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:])

	// 使用前两位作为子目录，避免单个目录文件过多
	subDir := hashStr[:2]
	fileName := hashStr[2:] + ".cache"

	return filepath.Join(d.basePath, subDir, fileName)
}

// Cleanup 清理过期的硬盘缓存文件
func (d *DiskCache) Cleanup(ctx context.Context) error {
	var filesToDelete []string

	// 收集需要删除的文件
	err := filepath.Walk(d.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 只处理.cache文件
		if !strings.HasSuffix(path, ".cache") {
			return nil
		}

		// 检查文件修改时间
		if time.Since(info.ModTime()) > 24*time.Hour {
			filesToDelete = append(filesToDelete, path)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 批量删除文件
	for _, file := range filesToDelete {
		d.mu.Lock()
		if err := os.Remove(file); err != nil {
			// 记录删除失败的文件，但不中断批量删除
			log.Printf("Failed to delete cache file %s: %v", file, err)
		}
		d.mu.Unlock()
	}

	return nil
}
