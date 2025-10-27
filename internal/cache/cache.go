package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
}

type Entry struct {
	StatusCode  int
	Headers     map[string]string
	Body        []byte
	StoredAt    time.Time
	ContentType string
}

func NewRedisClient(redisURL string) (*redis.Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	c := redis.NewClient(opt)
	return c, nil
}

func NewCache(client *redis.Client) *Cache {
	return &Cache{client: client}
}

func (c *Cache) BuildKey(method, upstreamURL string) string {
	h := sha256.Sum256([]byte(method + " " + upstreamURL))
	return "cache:v3:" + hex.EncodeToString(h[:])
}

func (c *Cache) Get(ctx context.Context, key string) (*Entry, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("cache not initialized")
	}
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var e Entry
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&e); err != nil {
		return nil, err
	}
	return &e, nil
}

func (c *Cache) Set(ctx context.Context, key string, e *Entry, ttl time.Duration) error {
	if c == nil || c.client == nil {
		return errors.New("cache not initialized")
	}
	buf := &bytes.Buffer{}
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(e); err != nil {
		return err
	}
	return c.client.Set(ctx, key, buf.Bytes(), ttl).Err()
}

// GetTTLByContentType 根据Content-Type返回合适的TTL
func GetTTLByContentType(contentType string, defaultTTL time.Duration) time.Duration {
	if contentType == "" {
		return defaultTTL
	}

	contentType = strings.ToLower(contentType)

	// 静态资源 - 长期缓存 (7天)
	if strings.Contains(contentType, "text/css") ||
		strings.Contains(contentType, "application/javascript") ||
		strings.Contains(contentType, "text/javascript") ||
		strings.Contains(contentType, "application/json") {
		return 7 * 24 * time.Hour
	}

	// 图片资源 - 中期缓存 (1天)
	if strings.Contains(contentType, "image/") {
		return 24 * time.Hour
	}

	// 字体文件 - 长期缓存 (30天)
	if strings.Contains(contentType, "font/") ||
		strings.Contains(contentType, "application/font-") {
		return 30 * 24 * time.Hour
	}

	// 视频文件 - 长期缓存 (7天)
	if strings.Contains(contentType, "video/") {
		return 7 * 24 * time.Hour
	}

	// 音频文件 - 长期缓存 (7天)
	if strings.Contains(contentType, "audio/") {
		return 7 * 24 * time.Hour
	}

	// HTML文档 - 短期缓存 (1小时)
	if strings.Contains(contentType, "text/html") {
		return time.Hour
	}

	// 其他类型使用默认TTL
	return defaultTTL
}

// GetCacheControlByContentType 根据Content-Type返回合适的Cache-Control头
func GetCacheControlByContentType(contentType string) string {
	if contentType == "" {
		return "public, max-age=43200" // 默认12小时
	}

	contentType = strings.ToLower(contentType)

	// 静态资源 - 长期缓存
	if strings.Contains(contentType, "text/css") ||
		strings.Contains(contentType, "application/javascript") ||
		strings.Contains(contentType, "text/javascript") ||
		strings.Contains(contentType, "application/json") {
		return "public, max-age=604800, immutable" // 7天
	}

	// 图片资源 - 中期缓存
	if strings.Contains(contentType, "image/") {
		return "public, max-age=86400" // 1天
	}

	// 字体文件 - 长期缓存
	if strings.Contains(contentType, "font/") ||
		strings.Contains(contentType, "application/font-") {
		return "public, max-age=2592000, immutable" // 30天
	}

	// 视频文件 - 长期缓存
	if strings.Contains(contentType, "video/") {
		return "public, max-age=604800" // 7天
	}

	// 音频文件 - 长期缓存
	if strings.Contains(contentType, "audio/") {
		return "public, max-age=604800" // 7天
	}

	// HTML文档 - 短期缓存
	if strings.Contains(contentType, "text/html") {
		return "public, max-age=3600" // 1小时
	}

	// 其他类型
	return "public, max-age=43200" // 默认12小时
}
