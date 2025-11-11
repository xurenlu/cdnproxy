// Package proxy AI API代理处理器（支持住宅IP）
// 作者: rocky<m@some.im>

package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cdnproxy/internal/proxy/providers"
)

// AIAPIProxy AI API代理处理器
type AIAPIProxy struct {
	proxyManager *ResidentialProxyManager
	httpClient   *http.Client
}

// NewAIAPIProxy 创建AI API代理处理器
func NewAIAPIProxy(proxyManager *ResidentialProxyManager) *AIAPIProxy {
	return &AIAPIProxy{
		proxyManager: proxyManager,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // AI API通常需要更长的超时时间
		},
	}
}

// ProxyAIRequest 代理AI API请求
func (aap *AIAPIProxy) ProxyAIRequest(w http.ResponseWriter, r *http.Request, upstreamURL string) error {
	// 检测目标API类型
	apiType := aap.detectAPIType(upstreamURL)

	// 获取最佳住宅IP代理
	proxy, err := aap.proxyManager.GetBestProxy(r.Context(), apiType)
	if err != nil {
		// 如果没有住宅IP代理，回退到普通代理
		return aap.proxyWithoutResidentialIP(w, r, upstreamURL)
	}

	// 创建使用住宅IP的HTTP客户端
	client, err := aap.proxyManager.CreateHTTPClient(proxy)
	if err != nil {
		return fmt.Errorf("failed to create proxy client: %w", err)
	}

	// 创建上游请求
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 复制请求头
	aap.copyHeaders(req.Header, r.Header)

	// 添加住宅IP特定的请求头
	aap.addResidentialIPHeaders(req, proxy)

	// 记录开始时间
	start := time.Now()

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		// 报告代理使用失败
		aap.proxyManager.ReportUsage(proxy, false, time.Since(start))
		return fmt.Errorf("proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	// 记录延迟
	latency := time.Since(start)

	// 复制响应头
	aap.copyHeaders(w.Header(), resp.Header)

	// 设置状态码
	w.WriteHeader(resp.StatusCode)

	// 流式传输响应体
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		// 报告代理使用失败
		aap.proxyManager.ReportUsage(proxy, false, latency)
		return fmt.Errorf("failed to copy response body: %w", err)
	}

	// 报告代理使用成功
	aap.proxyManager.ReportUsage(proxy, true, latency)

	return nil
}

// detectAPIType 检测API类型
func (aap *AIAPIProxy) detectAPIType(upstreamURL string) string {
	upstreamURL = strings.ToLower(upstreamURL)

	if strings.Contains(upstreamURL, "api.openai.com") {
		return "openai"
	}
	if strings.Contains(upstreamURL, "api.anthropic.com") {
		return "claude"
	}
	if strings.Contains(upstreamURL, "generativelanguage.googleapis.com") {
		return "gemini"
	}
	if strings.Contains(upstreamURL, "poe.com") {
		return "poe"
	}

	return "default"
}

// proxyWithoutResidentialIP 不使用住宅IP的代理（回退方案）
func (aap *AIAPIProxy) proxyWithoutResidentialIP(w http.ResponseWriter, r *http.Request, upstreamURL string) error {
	// 创建普通HTTP客户端
	client := &http.Client{
		Timeout: 5 * time.Minute, // AI API请求可能需要较长时间，设置为5分钟
	}

	// 创建上游请求
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 复制请求头
	aap.copyHeaders(req.Header, r.Header)

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 复制响应头
	aap.copyHeaders(w.Header(), resp.Header)

	// 设置状态码
	w.WriteHeader(resp.StatusCode)

	// 流式传输响应体
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy response body: %w", err)
	}

	return nil
}

// copyHeaders 复制请求头
func (aap *AIAPIProxy) copyHeaders(dst, src http.Header) {
	for k, v := range src {
		// 跳过一些不应该转发的头
		lowerK := strings.ToLower(k)
		if lowerK == "host" || lowerK == "connection" || lowerK == "upgrade" {
			continue
		}
		dst[k] = v
	}
}

// addResidentialIPHeaders 添加住宅IP特定的请求头
func (aap *AIAPIProxy) addResidentialIPHeaders(req *http.Request, proxy *providers.ResidentialProxy) {
	// 添加真实浏览器请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Cache-Control", "max-age=0")

	// 添加代理元数据头（用于调试和监控）
	req.Header.Set("X-Proxy-Provider", proxy.ISP)
	req.Header.Set("X-Proxy-Location", proxy.Location)
	req.Header.Set("X-Proxy-Country", proxy.Country)
	req.Header.Set("X-Proxy-Quality", fmt.Sprintf("%d", proxy.Quality))
}

// GetProxyStats 获取代理统计信息
func (aap *AIAPIProxy) GetProxyStats() map[string]interface{} {
	healthStatus := aap.proxyManager.GetHealthStatus()

	stats := map[string]interface{}{
		"total_proxies":     len(healthStatus),
		"healthy_proxies":   0,
		"unhealthy_proxies": 0,
		"average_latency":   0.0,
		"providers":         make(map[string]interface{}),
	}

	totalLatency := 0.0
	latencyCount := 0

	for _, result := range healthStatus {
		if result.IsHealthy {
			stats["healthy_proxies"] = stats["healthy_proxies"].(int) + 1
		} else {
			stats["unhealthy_proxies"] = stats["unhealthy_proxies"].(int) + 1
		}

		if result.Latency > 0 {
			totalLatency += float64(result.Latency.Milliseconds())
			latencyCount++
		}
	}

	if latencyCount > 0 {
		stats["average_latency"] = totalLatency / float64(latencyCount)
	}

	return stats
}
