package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"cdnproxy/internal/admin"
	"cdnproxy/internal/cache"
	"cdnproxy/internal/config"
	"cdnproxy/internal/docs"
	"cdnproxy/internal/proxy"
	"cdnproxy/internal/storage"
)

var startTime = time.Now()

func main() {
	// 设置文件描述符限制
	if err := setFDLimit(4096); err != nil {
		log.Printf("Failed to set FD limit: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// 使用 Redis 缓存
	redisClient, err := cache.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("failed to create redis client: %v", err)
	}

	// Redis subsystems
	cacheStore := cache.NewCache(redisClient)
	whitelistStore := storage.NewWhitelistStore(redisClient)
	configStore := storage.NewConfigStore(redisClient)
	counterStore := storage.NewCounterStore(redisClient)

	// 启动资源监控
	go monitorResources()

	adminServer, err := admin.NewServer(cfg, redisClient, whitelistStore, configStore)
	if err != nil {
		log.Fatalf("failed to create admin server: %v", err)
	}
	ipBanStore := storage.NewIPBanStore(redisClient, storage.IPBanConfig{
		Enabled:   cfg.IPBanEnabled,
		Threshold: cfg.IPBanThreshold,
		WindowSec: cfg.IPBanWindowSec,
		BanSec:    cfg.IPBanDuration,
	})
	proxyHandler := proxy.NewHandler(cfg, cacheStore, whitelistStore, configStore, counterStore, ipBanStore)

	mux := http.NewServeMux()
	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		status := "ok"
		code := http.StatusOK
		
		// 检查内存使用
		if m.Alloc > 500*1024*1024 { // 500MB
			status = "warning: high memory usage"
			code = http.StatusOK
		}
		
		// 检查 Goroutine 数量
		if runtime.NumGoroutine() > 1000 {
			status = "warning: too many goroutines"
			code = http.StatusOK
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"status": "%s",
			"memory_alloc_mb": %d,
			"memory_sys_mb": %d,
			"goroutines": %d,
			"uptime_seconds": %d
		}`, status, m.Alloc/1024/1024, m.Sys/1024/1024, runtime.NumGoroutine(), int(time.Since(startTime).Seconds()))))
	})

	// Metrics endpoint (暂时注释，如果不需要可以删除)
	// mux.HandleFunc("/metrics", metrics.Handler())

	// Docs page
	mux.HandleFunc("/docs", docs.Handler())

	// AI 文档：llm.txt / llms.txt
	mux.HandleFunc("/llm.txt", docs.LLMTxtHandler())
	mux.HandleFunc("/llms.txt", docs.LLMsTxtHandler())

	// Admin routes mounted under /admin/
	adminServer.RegisterRoutes(mux)

	// Proxy catch-all (must be last)
	mux.Handle("/", proxyHandler)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           panicRecoveryMiddleware(loggingMiddleware(mux)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Minute,  // 支持长连接请求（API请求可能需要5分钟，SSE流式响应可能需要10分钟）
		WriteTimeout:      10 * time.Minute,  // 支持长连接响应（API响应可能需要5分钟，SSE流式响应可能需要10分钟）
		IdleTimeout:       60 * time.Second,  // 空闲超时保持较短，避免资源浪费
	}

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}

// panicRecoveryMiddleware 全局 panic 恢复中间件，防止单个请求的 panic 导致整个服务崩溃
func panicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC RECOVERED: %v, path: %s, method: %s", err, r.URL.Path, r.Method)
				// 尝试返回 500 错误（如果响应头还没有写入）
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &logResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)
		duration := time.Since(start)
		log.Printf("%s %s %d %s ua=%q ref=%q", r.Method, r.URL.Path, lrw.statusCode, duration, r.UserAgent(), r.Referer())
	})
}

type logResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *logResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// toJSON 将 map 转换为 JSON 字符串
func toJSON(data map[string]interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(jsonData)
}

// setFDLimit 设置文件描述符限制
func setFDLimit(limit int) error {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return err
	}
	if rLimit.Max < uint64(limit) {
		limit = int(rLimit.Max)
	}
	rLimit.Cur = uint64(limit)
	rLimit.Max = uint64(limit)
	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
}

// monitorResources 监控系统资源使用情况
func monitorResources() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		log.Printf("Resource stats: goroutines=%d, memory_alloc_mb=%d, memory_sys_mb=%d",
			runtime.NumGoroutine(), m.Alloc/1024/1024, m.Sys/1024/1024)
	}
}
