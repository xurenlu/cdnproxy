package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port          string
	RedisURL      string
	CacheTTL      time.Duration
	AdminUsername string
	AdminPassword string
	SessionTTL    time.Duration
	APIDomains    []string // API 服务域名列表（不缓存、支持 WebSocket/SSE）
	WebPEnabled   bool     // WebP 转换功能开关

	// IP 封禁：错误请求过多时自动封禁
	IPBanEnabled   bool
	IPBanThreshold int // 窗口内 400/503 次数阈值
	IPBanWindowSec int // 统计窗口（秒）
	IPBanDuration  int // 封禁时长（秒）
}

func Load() Config {
	// PORT default 8080 (user preference)
	port := getenv("PORT", "8080")

	// Redis URL
	redisURL := getenv("REDIS_URL", "redis://localhost:6379/0")

	ttlSeconds := getenvInt("CACHE_TTL_SECONDS", 43200)          // 12h
	sessionTTLSeconds := getenvInt("SESSION_TTL_SECONDS", 86400) // 24h

	adminUser := getenv("ADMIN_USERNAME", "admin")
	adminPass := getenv("ADMIN_PASSWORD", "cdnproxy123!")

	// WebP 功能开关（默认关闭）
	webpEnabled := getenvBool("WEBP_ENABLED", false)

	// IP 封禁配置（400/503 过多时自动封禁）
	ipBanEnabled := getenvBool("IP_BAN_ENABLED", true)
	ipBanThreshold := getenvInt("IP_BAN_THRESHOLD", 30)
	ipBanWindow := getenvInt("IP_BAN_WINDOW_SEC", 300)
	ipBanDuration := getenvInt("IP_BAN_DURATION_SEC", 3600)

	// 默认的 API 域名列表（支持环境变量自定义）
	apiDomains := []string{
		"api.openai.com",
		"api.anthropic.com",
		"claude.ai",
		"poe.com",
		"api.poe.com",
		"gemini.google.com",
		"generativelanguage.googleapis.com",
		"api.cohere.ai",
		"api.together.xyz",
		"api.groq.com",
	}
	// 支持通过环境变量添加额外的域名（逗号分隔）
	if extraDomains := os.Getenv("API_DOMAINS"); extraDomains != "" {
		for _, domain := range splitAndTrim(extraDomains, ",") {
			if domain != "" {
				apiDomains = append(apiDomains, domain)
			}
		}
	}

	return Config{
		Port:            port,
		RedisURL:        redisURL,
		CacheTTL:        time.Duration(ttlSeconds) * time.Second,
		AdminUsername:   adminUser,
		AdminPassword:   adminPass,
		SessionTTL:     time.Duration(sessionTTLSeconds) * time.Second,
		APIDomains:     apiDomains,
		WebPEnabled:    webpEnabled,
		IPBanEnabled:   ipBanEnabled,
		IPBanThreshold: ipBanThreshold,
		IPBanWindowSec: ipBanWindow,
		IPBanDuration:  ipBanDuration,
	}
}

func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	// 支持 true/false, 1/0, yes/no, on/off
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

func splitAndTrim(s, sep string) []string {
	var result []string
	for _, item := range splitString(s, sep) {
		trimmed := trimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	var current string
	for _, ch := range s {
		if string(ch) == sep {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	parts = append(parts, current)
	return parts
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func generateRandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
