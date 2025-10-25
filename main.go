package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"cdnproxy/internal/admin"
	"cdnproxy/internal/cache"
	"cdnproxy/internal/config"
	"cdnproxy/internal/docs"
	"cdnproxy/internal/proxy"
	"cdnproxy/internal/storage"
)

func main() {
	// 设置文件描述符限制
	if err := setFDLimit(4096); err != nil {
		log.Printf("Failed to set FD limit: %v", err)
	}

	cfg := config.Load()

	// 使用硬盘缓存替代 Redis
	diskCache, err := cache.NewDiskCache(cfg.CacheDir, 250*1024*1024) // 最大缓存单文件 100MB
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

	// 启动定期资源监控
	go monitorResources()

	adminServer, err := admin.NewServer(cfg, whitelistStore, configStore)
	if err != nil {
		log.Fatalf("failed to create admin server: %v", err)
	}

	proxyHandler := proxy.NewHandler(cfg, diskCache, whitelistStore, configStore, counterStore)

	mux := http.NewServeMux()
	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
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
		ReadTimeout:       30 * time.Second, // 添加读取超时
		WriteTimeout:      30 * time.Second, // 添加写入超时
		IdleTimeout:       60 * time.Second, // 缩短空闲超时
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

	cleaning := false
	var cleanMutex sync.Mutex

	for range ticker.C {
		// 检查是否已有清理任务在运行
		cleanMutex.Lock()
		if cleaning {
			log.Printf("Previous cleanup still running, skipping this round")
			cleanMutex.Unlock()
			continue
		}
		cleaning = true
		cleanMutex.Unlock()

		// 异步清理，避免阻塞主流程
		go func() {
			defer func() {
				cleanMutex.Lock()
				cleaning = false
				cleanMutex.Unlock()
			}()

			time.Sleep(5 * time.Minute) // 延迟清理，避免与高峰期冲突
			if err := diskCache.Cleanup(context.Background()); err != nil {
				log.Printf("cache cleanup error: %v", err)
			}
		}()
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

		// 检查资源使用限制
		if m.Alloc/1024/1024 > MaxMemoryMB {
			log.Printf("WARNING: Memory usage too high: %dMB (limit: %dMB)", m.Alloc/1024/1024, MaxMemoryMB)
		}

		if goroutines > MaxGoroutines {
			log.Printf("WARNING: Too many goroutines: %d (limit: %d)", goroutines, MaxGoroutines)
		}
	}
}
