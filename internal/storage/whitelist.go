package storage

import (
	"context"
	"strings"

	redis "github.com/redis/go-redis/v9"
)

const whitelistKey = "whitelist:suffixes"

type WhitelistStore struct {
	client *redis.Client
}

func NewWhitelistStore(client *redis.Client) *WhitelistStore {
	return &WhitelistStore{client: client}
}

func (s *WhitelistStore) List(ctx context.Context) ([]string, error) {
	members, err := s.client.SMembers(ctx, whitelistKey).Result()
	if err != nil {
		if err == redis.Nil {
			return []string{}, nil
		}
		return nil, err
	}
	return members, nil
}

func (s *WhitelistStore) Add(ctx context.Context, suffix string) error {
	suffix = strings.TrimSpace(strings.ToLower(suffix))
	if suffix == "" {
		return nil
	}
	return s.client.SAdd(ctx, whitelistKey, suffix).Err()
}

func (s *WhitelistStore) Remove(ctx context.Context, suffix string) error {
	suffix = strings.TrimSpace(strings.ToLower(suffix))
	if suffix == "" {
		return nil
	}
	return s.client.SRem(ctx, whitelistKey, suffix).Err()
}

// ContainsAllowedSuffix checks if host ends with any whitelisted suffix.
func (s *WhitelistStore) ContainsAllowedSuffix(ctx context.Context, host string) (bool, error) {
	host = strings.ToLower(host)
	suffixes, err := s.List(ctx)
	if err != nil {
		return false, err
	}
	for _, suf := range suffixes {
		suf = strings.ToLower(strings.TrimSpace(suf))
		if suf == "" {
			continue
		}
		if strings.HasSuffix(host, suf) {
			return true, nil
		}
	}
	return false, nil
}
