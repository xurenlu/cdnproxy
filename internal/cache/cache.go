package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
}

type Entry struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	StoredAt   time.Time
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
	return "cache:" + hex.EncodeToString(h[:])
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
