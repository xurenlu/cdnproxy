package storage

import (
	"context"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

const (
	keyReferrerThreshold = "config:referrer_threshold"
	defaultThreshold     = int64(1000)
)

type ConfigStore struct {
	client *redis.Client
}

func NewConfigStore(client *redis.Client) *ConfigStore {
	return &ConfigStore{client: client}
}

func (s *ConfigStore) GetReferrerThreshold(ctx context.Context) (int64, error) {
	v, err := s.client.Get(ctx, keyReferrerThreshold).Result()
	if err != nil {
		if err == redis.Nil {
			return defaultThreshold, nil
		}
		return defaultThreshold, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return defaultThreshold, nil
	}
	if n <= 0 {
		return defaultThreshold, nil
	}
	return n, nil
}

func (s *ConfigStore) SetReferrerThreshold(ctx context.Context, n int64) error {
	if n <= 0 {
		n = defaultThreshold
	}
	return s.client.Set(ctx, keyReferrerThreshold, strconv.FormatInt(n, 10), 0).Err()
}

type CounterStore struct {
	client *redis.Client
}

func NewCounterStore(client *redis.Client) *CounterStore {
	return &CounterStore{client: client}
}

// IncrementReferrerCount increases the counter for a host with a rolling 24h TTL window.
func (s *CounterStore) IncrementReferrerCount(ctx context.Context, host string) (int64, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return 0, nil
	}
	key := "refcnt:" + host
	n, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	ttl, err := s.client.TTL(ctx, key).Result()
	if err == nil && (ttl <= 0 || ttl > 24*time.Hour) {
		_ = s.client.Expire(ctx, key, 24*time.Hour).Err()
	}
	return n, nil
}
