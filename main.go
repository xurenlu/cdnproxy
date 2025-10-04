package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	cfg := config.Load()

	// 使用硬盘缓存替代 Redis
	diskCache, err := cache.NewDiskCache(cfg.CacheDir, 170*1024*1024) // 最大缓存单文件 100MB
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
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       0, // 无限制，允许长时间的大文件传输
		WriteTimeout:      0, // 无限制，允许长时间的大文件传输
		IdleTimeout:       120 * time.Second,
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
