package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cdnproxy/internal/adapters/serverless"
	"cdnproxy/internal/cache"
	"cdnproxy/internal/config"
	"cdnproxy/internal/proxy"
	"cdnproxy/internal/storage"
)

// 全局变量，在冷启动时初始化
var (
	proxyHandler http.Handler
	adminHandler http.Handler
)

// 初始化函数，在冷启动时调用
func init() {
	log.Println("初始化 CDNProxy 云函数版本...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// 初始化缓存（使用内存缓存，适合云函数）
	diskCache, err := cache.NewDiskCache("/tmp/cache", 100*1024*1024) // 100MB
	if err != nil {
		log.Fatalf("failed to create disk cache: %v", err)
	}

	// 初始化存储（使用内存存储，适合云函数）
	whitelistStore, err := storage.NewFileWhitelistStore("/tmp/data")
	if err != nil {
		log.Fatalf("failed to create whitelist store: %v", err)
	}

	configStore, err := storage.NewFileConfigStore("/tmp/data")
	if err != nil {
		log.Fatalf("failed to create config store: %v", err)
	}

	counterStore, err := storage.NewFileCounterStore("/tmp/data")
	if err != nil {
		log.Fatalf("failed to create counter store: %v", err)
	}

	// 初始化 IP 封禁存储（云函数环境禁用，因为没有持久化）
	ipBanStore := storage.NewIPBanStore(nil, storage.IPBanConfig{
		Enabled: false, // 云函数环境禁用 IP 封禁
	})

	// 创建处理器
	proxyHandler = proxy.NewHandler(cfg, diskCache, whitelistStore, configStore, counterStore, ipBanStore)

	// 云函数版本暂时不启用管理后台
	adminHandler = nil

	log.Println("CDNProxy 云函数版本初始化完成")
}

// 腾讯云函数入口
func TencentCloudMain(ctx context.Context, event *serverless.TencentCloudEvent) (*serverless.TencentCloudResponse, error) {
	adapter := serverless.NewTencentCloudAdapter(proxyHandler)
	return adapter.Wrap(proxyHandler)(ctx, event)
}

// 阿里云函数计算入口
func AliyunFCMain(ctx context.Context, event *serverless.AliyunFCEvent) (*serverless.AliyunFCResponse, error) {
	adapter := serverless.NewAliyunFCAdapter(proxyHandler)
	return adapter.Wrap(proxyHandler)(ctx, event)
}

// 根据环境变量选择入口点
func main() {
	platform := os.Getenv("SERVERLESS_PLATFORM")

	switch platform {
	case "tencent":
		// 腾讯云函数
		log.Println("使用腾讯云函数平台")
	case "aliyun":
		// 阿里云函数计算
		log.Println("使用阿里云函数计算平台")
	default:
		log.Fatal("未指定云函数平台，请设置 SERVERLESS_PLATFORM 环境变量")
	}
}
