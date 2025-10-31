package storage

import (
	"context"
	"errors"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// RedisSessionStore 基于Redis的会话存储
type RedisSessionStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisSessionStore(client *redis.Client, ttl time.Duration) *RedisSessionStore {
	return &RedisSessionStore{
		client: client,
		ttl:    ttl,
	}
}

func (s *RedisSessionStore) sessionKey(token string) string {
	return "session:" + token
}

// Set 创建或更新会话
func (s *RedisSessionStore) Set(token, value string) error {
	if s.client == nil {
		return errors.New("redis client not initialized")
	}
	return s.client.Set(context.Background(), s.sessionKey(token), value, s.ttl).Err()
}

// Exists 检查会话是否存在且未过期
func (s *RedisSessionStore) Exists(token string) bool {
	if s.client == nil {
		return false
	}
	val := s.client.Exists(context.Background(), s.sessionKey(token)).Val()
	return val > 0
}

// Delete 删除会话
func (s *RedisSessionStore) Delete(token string) error {
	if s.client == nil {
		return errors.New("redis client not initialized")
	}
	return s.client.Del(context.Background(), s.sessionKey(token)).Err()
}

