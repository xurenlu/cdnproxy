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
	"syscall"
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

	// 检查磁盘空间
	if err := d.checkDiskSpace(); err != nil {
		log.Printf("Disk space check failed: %v", err)
		return err
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

// checkDiskSpace 检查磁盘空间是否充足
func (d *DiskCache) checkDiskSpace() error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(d.basePath, &stat); err != nil {
		return err
	}

	// 计算可用空间（字节）
	availableSpace := stat.Bavail * uint64(stat.Bsize)

	// 如果可用空间小于1GB，返回错误
	const minFreeSpace = 1 * 1024 * 1024 * 1024 // 1GB
	if availableSpace < minFreeSpace {
		return errors.New("insufficient disk space")
	}

	return nil
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
	// 使用批量删除，避免slice无限增长
	const batchSize = 100
	deletedCount := 0

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
			// 立即删除，不累积到slice中
			d.mu.Lock()
			if err := os.Remove(path); err != nil {
				log.Printf("Failed to delete cache file %s: %v", path, err)
			} else {
				deletedCount++
			}
			d.mu.Unlock()

			// 每删除batchSize个文件，检查一次context
			if deletedCount%batchSize == 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}
		}

		return nil
	})

	if deletedCount > 0 {
		log.Printf("Cleanup completed: deleted %d expired cache files", deletedCount)
	}

	return err
}
