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
	"cdnproxy/internal/metrics"
	"cdnproxy/internal/proxy"
	"cdnproxy/internal/storage"
)

var startTime = time.Now()

func main() {
	// 设置文件描述符限制
	if err := setFDLimit(4096); err != nil {
		log.Printf("Failed to set FD limit: %v", err)
	}

	cfg := config.Load()

	// 使用硬盘缓存替代 Redis
	diskCache, err := cache.NewDiskCache(cfg.CacheDir, 250*1024*1024) // 最大缓存单文件 250MB
	if err != nil {
		log.Fatalf("failed to create disk cache: %v", err)
	}

	// 使用文件存储替代 Redis
	whitelistStore, err := storage.NewFileWhitelistStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("failed to create whitelist store: %v", err)
	}

	configStore, err := storage.NewFileConfigStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("failed to create config store: %v", err)
	}

	counterStore, err := storage.NewFileCounterStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("failed to create counter store: %v", err)
	}

	// 启动定期清理过期缓存
	go cleanupCache(diskCache)

	// 启动资源监控
	go monitorResources()

	adminServer, err := admin.NewServer(cfg, whitelistStore, configStore)
	if err != nil {
		log.Fatalf("failed to create admin server: %v", err)
	}

	proxyHandler := proxy.NewHandler(cfg, diskCache, whitelistStore, configStore, counterStore)

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

	// Metrics endpoint
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		// 获取应用指标
		appMetrics := metrics.GetGlobalMetrics().GetStats()
		
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(fmt.Sprintf(
			"# CDNProxy Metrics\n"+
				"# System Metrics\n"+
				"memory_alloc_bytes %d\n"+
				"memory_sys_bytes %d\n"+
				"goroutines_count %d\n"+
				"gc_runs_total %d\n"+
				"# Application Metrics\n"+
				"total_requests %v\n"+
				"successful_requests %v\n"+
				"failed_requests %v\n"+
				"success_rate %v\n"+
				"avg_response_time_ms %v\n"+
				"cache_hits %v\n"+
				"cache_misses %v\n"+
				"cache_hit_rate %v\n"+
				"active_connections %v\n"+
				"max_concurrent %v\n"+
				"uptime_seconds %v\n",
			m.Alloc, m.Sys, runtime.NumGoroutine(), m.NumGC,
			appMetrics["total_requests"],
			appMetrics["successful_requests"],
			appMetrics["failed_requests"],
			appMetrics["success_rate"],
			appMetrics["avg_response_time_ms"],
			appMetrics["cache_hits"],
			appMetrics["cache_misses"],
			appMetrics["cache_hit_rate"],
			appMetrics["active_connections"],
			appMetrics["max_concurrent"],
			appMetrics["uptime_seconds"],
		)))
	})

	// 详细统计端点
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		appMetrics := metrics.GetGlobalMetrics().GetStats()
		
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"system": {
				"memory_alloc_mb": %d,
				"memory_sys_mb": %d,
				"goroutines": %d,
				"gc_runs": %d
			},
			"application": %s,
			"timestamp": "%s"
		}`, 
			m.Alloc/1024/1024, m.Sys/1024/1024, runtime.NumGoroutine(), m.NumGC,
			toJSON(appMetrics),
			time.Now().Format(time.RFC3339),
		)))
	})

	// Docs page
	mux.HandleFunc("/docs", docs.Handler())

	// Admin routes mounted under /admin/
	adminServer.RegisterRoutes(mux)

	// Proxy catch-all (must be last)
	mux.Handle("/", proxyHandler)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,  // 添加读取超时
		WriteTimeout:      30 * time.Second,  // 添加写入超时
		IdleTimeout:       60 * time.Second,  // 缩短空闲超时
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

func cleanupCache(diskCache *cache.DiskCache) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if err := diskCache.Cleanup(context.Background()); err != nil {
			log.Printf("cache cleanup error: %v", err)
		}
	}
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
	rLimit.Cur = uint64(limit)
	rLimit.Max = uint64(limit)
	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
}

// monitorResources 定期监控资源使用情况
func monitorResources() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
	defer ticker.Stop()

	const (
		MaxMemoryMB   = 512
		MaxGoroutines = 1000
	)

	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		goroutines := runtime.NumGoroutine()

		// 记录性能指标
		log.Printf("MONITOR: memory=%dMB, goroutines=%d, sys=%dMB",
			m.Alloc/1024/1024,
			goroutines,
			m.Sys/1024/1024)

		// 检查内存使用
		if m.Alloc > MaxMemoryMB*1024*1024 {
			log.Printf("WARNING: High memory usage: %dMB", m.Alloc/1024/1024)
		}

		// 检查 Goroutine 数量
		if goroutines > MaxGoroutines {
			log.Printf("WARNING: High goroutine count: %d", goroutines)
		}

		// 定期输出状态（每5分钟）
		if time.Since(startTime).Minutes() > 0 && int(time.Since(startTime).Minutes())%5 == 0 {
			log.Printf("STATUS: Memory=%dMB, Goroutines=%d, Uptime=%s",
				m.Alloc/1024/1024, goroutines, time.Since(startTime).Round(time.Second))
		}
	}
}
