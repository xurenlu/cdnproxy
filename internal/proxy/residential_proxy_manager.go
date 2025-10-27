// Package proxy 住宅IP代理管理器
// 作者: rocky<m@some.im>

package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"cdnproxy/internal/proxy/providers"
)

// ResidentialProxyManager 住宅IP代理管理器
type ResidentialProxyManager struct {
	providers     map[string]providers.ResidentialProxyProvider
	mu            sync.RWMutex
	healthChecker *ProxyHealthChecker
	metrics       *ProxyMetricsCollector
}

// ProxyHealthChecker 代理健康检查器
type ProxyHealthChecker struct {
	checkInterval time.Duration
	timeout       time.Duration
	results       map[string]*HealthResult
	mu            sync.RWMutex
}

// HealthResult 健康检查结果
type HealthResult struct {
	IsHealthy   bool          `json:"is_healthy"`
	Latency     time.Duration `json:"latency"`
	LastCheck   time.Time     `json:"last_check"`
	ErrorCount  int           `json:"error_count"`
	SuccessRate float64       `json:"success_rate"`
}

// NewResidentialProxyManager 创建住宅IP代理管理器
func NewResidentialProxyManager() *ResidentialProxyManager {
	// 创建提供者注册表并注册所有提供者
	registry := providers.NewProviderRegistry()
	registry.RegisterAllProviders()

	return &ResidentialProxyManager{
		providers: registry.GetProviders(),
		healthChecker: &ProxyHealthChecker{
			checkInterval: 30 * time.Second,
			timeout:       10 * time.Second,
			results:       make(map[string]*HealthResult),
		},
		metrics: NewProxyMetricsCollector(),
	}
}

// RegisterProvider 注册代理提供者
func (rpm *ResidentialProxyManager) RegisterProvider(name string, provider providers.ResidentialProxyProvider) {
	rpm.mu.Lock()
	defer rpm.mu.Unlock()
	rpm.providers[name] = provider
}

// GetBestProxy 获取最佳代理
func (rpm *ResidentialProxyManager) GetBestProxy(ctx context.Context, targetAPI string) (*providers.ResidentialProxy, error) {
	rpm.mu.RLock()
	defer rpm.mu.RUnlock()

	var bestProxy *providers.ResidentialProxy
	var bestScore float64

	for _, provider := range rpm.providers {
		proxy, err := provider.GetProxy(ctx)
		if err != nil {
			continue
		}

		// 计算代理评分
		score := rpm.calculateProxyScore(proxy, targetAPI)
		if score > bestScore {
			bestScore = score
			bestProxy = proxy
		}
	}

	if bestProxy == nil {
		return nil, fmt.Errorf("no available proxy found")
	}

	return bestProxy, nil
}

// calculateProxyScore 计算代理评分
func (rpm *ResidentialProxyManager) calculateProxyScore(proxy *providers.ResidentialProxy, targetAPI string) float64 {
	score := 0.0

	// 基础质量评分 (0-40分)
	score += float64(proxy.Quality) * 4

	// 成功率评分 (0-30分)
	score += proxy.SuccessRate * 30

	// 地理位置评分 (0-20分)
	score += rpm.getLocationScore(proxy, targetAPI)

	// 使用频率评分 (0-10分)
	timeSinceLastUsed := time.Since(proxy.LastUsed)
	if timeSinceLastUsed > time.Hour {
		score += 10 // 长时间未使用，优先选择
	} else {
		score += 10 - float64(timeSinceLastUsed.Minutes())/6 // 按时间递减
	}

	return score
}

// getLocationScore 获取地理位置评分
func (rpm *ResidentialProxyManager) getLocationScore(proxy *providers.ResidentialProxy, targetAPI string) float64 {
	// 根据目标API选择最佳地理位置
	preferredCountries := map[string][]string{
		"openai":  {"US", "CA", "GB", "AU"},
		"claude":  {"US", "CA", "GB", "AU"},
		"gemini":  {"US", "CA", "GB", "AU"},
		"default": {"US", "CA", "GB", "AU", "DE", "FR"},
	}

	countries, exists := preferredCountries[targetAPI]
	if !exists {
		countries = preferredCountries["default"]
	}

	for i, country := range countries {
		if proxy.Country == country {
			return 20 - float64(i)*2 // 按优先级递减
		}
	}

	return 5 // 默认分数
}

// CreateHTTPClient 创建使用住宅IP的HTTP客户端
func (rpm *ResidentialProxyManager) CreateHTTPClient(proxy *providers.ResidentialProxy) (*http.Client, error) {
	// 构建代理URL
	proxyURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", proxy.IP, proxy.Port),
		User:   url.UserPassword(proxy.Username, proxy.Password),
	}

	// 创建代理传输
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}, nil
}

// ReportUsage 报告使用情况
func (rpm *ResidentialProxyManager) ReportUsage(proxy *providers.ResidentialProxy, success bool, latency time.Duration) {
	rpm.mu.RLock()
	provider, exists := rpm.providers[proxy.ISP]
	rpm.mu.RUnlock()

	if exists {
		provider.ReportUsage(proxy, success, latency)
	}
	
	// 计算成本
	cost := rpm.calculateCost(proxy, latency)
	
	// 记录指标
	rpm.metrics.RecordRequest(proxy.ID, success, latency, cost)
}

// StartHealthCheck 启动健康检查
func (rpm *ResidentialProxyManager) StartHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(rpm.healthChecker.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rpm.performHealthCheck(ctx)
		}
	}
}

// performHealthCheck 执行健康检查
func (rpm *ResidentialProxyManager) performHealthCheck(ctx context.Context) {
	rpm.mu.RLock()
	providers := make([]providers.ResidentialProxyProvider, 0, len(rpm.providers))
	for _, provider := range rpm.providers {
		providers = append(providers, provider)
	}
	rpm.mu.RUnlock()

	for _, provider := range providers {
		go rpm.checkProviderHealth(ctx, provider)
	}
}

// checkProviderHealth 检查提供者健康状态
func (rpm *ResidentialProxyManager) checkProviderHealth(ctx context.Context, provider providers.ResidentialProxyProvider) {
	proxy, err := provider.GetProxy(ctx)
	if err != nil {
		return
	}

	client, err := rpm.CreateHTTPClient(proxy)
	if err != nil {
		return
	}

	start := time.Now()
	resp, err := client.Get("https://httpbin.org/ip")
	latency := time.Since(start)

	rpm.healthChecker.mu.Lock()
	defer rpm.healthChecker.mu.Unlock()

	result := &HealthResult{
		IsHealthy: err == nil && resp.StatusCode == 200,
		Latency:   latency,
		LastCheck: time.Now(),
	}

	if err != nil {
		result.ErrorCount++
	}

	rpm.healthChecker.results[proxy.ID] = result
}

// GetHealthStatus 获取健康状态
func (rpm *ResidentialProxyManager) GetHealthStatus() map[string]*HealthResult {
	rpm.healthChecker.mu.RLock()
	defer rpm.healthChecker.mu.RUnlock()

	results := make(map[string]*HealthResult)
	for id, result := range rpm.healthChecker.results {
		results[id] = result
	}

	return results
}

// calculateCost 计算成本
func (rpm *ResidentialProxyManager) calculateCost(proxy *providers.ResidentialProxy, latency time.Duration) float64 {
	rpm.mu.RLock()
	provider, exists := rpm.providers[proxy.ISP]
	rpm.mu.RUnlock()
	
	if !exists {
		return 0
	}
	
	cost := provider.GetCost()
	
	// 计算每次请求的成本（简化处理，假设每次请求1KB）
	return cost.PerRequest + (cost.PerGB / 1024 / 1024) + (cost.PerHour / 3600 * float64(latency.Seconds()))
}

// GetMetrics 获取指标收集器
func (rpm *ResidentialProxyManager) GetMetrics() *ProxyMetricsCollector {
	return rpm.metrics
}

// GetSummary 获取汇总统计
func (rpm *ResidentialProxyManager) GetSummary() map[string]interface{} {
	return rpm.metrics.GetSummary()
}
