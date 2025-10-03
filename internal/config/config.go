package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port          string
	DataDir       string        // 数据存储目录
	CacheDir      string        // 缓存文件目录
	CacheTTL      time.Duration
	AdminUsername string
	AdminPassword string
	SessionTTL    time.Duration
}

func Load() Config {
	// PORT default 8080 (user preference)
	port := getenv("PORT", "8080")

	// 数据目录配置
	dataDir := getenv("DATA_DIR", "./data")
	cacheDir := getenv("CACHE_DIR", "./data/cache")

	ttlSeconds := getenvInt("CACHE_TTL_SECONDS", 43200)          // 12h
	sessionTTLSeconds := getenvInt("SESSION_TTL_SECONDS", 86400) // 24h

	adminUser := getenv("ADMIN_USERNAME", "admin")
	adminPass := getenv("ADMIN_PASSWORD", "cdnproxy123!")

	return Config{
		Port:          port,
		DataDir:       dataDir,
		CacheDir:      cacheDir,
		CacheTTL:      time.Duration(ttlSeconds) * time.Second,
		AdminUsername: adminUser,
		AdminPassword: adminPass,
		SessionTTL:    time.Duration(sessionTTLSeconds) * time.Second,
	}
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
