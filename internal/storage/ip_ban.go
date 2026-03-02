package storage

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

const (
	keyIPErrorsPrefix = "ip_err:"
	keyIPBanPrefix    = "ip_ban:"
)

// IPBanStore IP 封禁存储（基于错误次数自动封禁）
type IPBanStore struct {
	client     *redis.Client
	enabled    bool
	threshold  int           // 窗口内错误次数阈值
	window     time.Duration // 统计窗口
	banDur     time.Duration // 封禁时长
	banMessage string        // 封禁时返回的提示
}

// IPBanConfig 封禁配置
type IPBanConfig struct {
	Enabled   bool
	Threshold int
	WindowSec int
	BanSec    int
}

// NewIPBanStore 创建 IP 封禁存储
func NewIPBanStore(client *redis.Client, cfg IPBanConfig) *IPBanStore {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 30
	}
	if cfg.WindowSec <= 0 {
		cfg.WindowSec = 300
	}
	if cfg.BanSec <= 0 {
		cfg.BanSec = 3600
	}
	return &IPBanStore{
		client:     client,
		enabled:    cfg.Enabled,
		threshold:  cfg.Threshold,
		window:     time.Duration(cfg.WindowSec) * time.Second,
		banDur:     time.Duration(cfg.BanSec) * time.Second,
		banMessage: "Your IP has been temporarily banned due to excessive invalid requests. Please read /docs for correct usage. Ban will expire automatically.",
	}
}

// IsBanned 检查 IP 是否被封禁
func (s *IPBanStore) IsBanned(ctx context.Context, ip string) (bool, time.Duration) {
	if !s.enabled || ip == "" {
		return false, 0
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false, 0
	}
	key := keyIPBanPrefix + ip
	ttl, err := s.client.TTL(ctx, key).Result()
	if err != nil || ttl <= 0 {
		return false, 0
	}
	return true, ttl
}

// RecordError 记录错误，若超过阈值则自动封禁
func (s *IPBanStore) RecordError(ctx context.Context, ip string, statusCode int) {
	if !s.enabled || ip == "" {
		return
	}
	// 只统计 400 和 503
	if statusCode != 400 && statusCode != 503 {
		return
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return
	}
	keyErr := keyIPErrorsPrefix + ip
	keyBan := keyIPBanPrefix + ip
	pipe := s.client.Pipeline()
	incr := pipe.Incr(ctx, keyErr)
	pipe.Expire(ctx, keyErr, s.window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("IPBanStore RecordError: %v", err)
		return
	}
	n := incr.Val()
	if n >= int64(s.threshold) {
		_ = s.client.Set(ctx, keyBan, "1", s.banDur).Err()
		_ = s.client.Del(ctx, keyErr).Err()
		log.Printf("IP banned: %s (errors=%d, threshold=%d, ban=%s)", ip, n, s.threshold, s.banDur)
	}
}

// Unban 手动解除封禁（供管理后台使用）
func (s *IPBanStore) Unban(ctx context.Context, ip string) error {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return nil
	}
	_ = s.client.Del(ctx, keyIPErrorsPrefix+ip).Err()
	return s.client.Del(ctx, keyIPBanPrefix+ip).Err()
}

// GetBanTTL 获取封禁剩余时间（未封禁返回 0）
func (s *IPBanStore) GetBanTTL(ctx context.Context, ip string) time.Duration {
	_, ttl := s.IsBanned(ctx, ip)
	return ttl
}

// BanMessage 返回封禁提示文案
func (s *IPBanStore) BanMessage() string {
	return s.banMessage
}

// FormatBanResponse 生成封禁响应的 JSON 或纯文本
func (s *IPBanStore) FormatBanResponse(ttl time.Duration) string {
	mins := int(ttl.Minutes())
	if mins < 1 {
		mins = 1
	}
	return s.banMessage + " Expires in " + strconv.Itoa(mins) + " minutes."
}
