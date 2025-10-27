// Package proxy 代理性能监控和统计
// 作者: rocky<m@some.im>

package proxy

import (
	"sync"
	"time"
)

// ProxyMetrics 代理性能指标
type ProxyMetrics struct {
	// 请求统计
	TotalRequests    int64 `json:"total_requests"`
	SuccessRequests  int64 `json:"success_requests"`
	FailedRequests   int64 `json:"failed_requests"`
	
	// 延迟统计
	TotalLatency     time.Duration `json:"total_latency"`
	AverageLatency   time.Duration `json:"average_latency"`
	MinLatency       time.Duration `json:"min_latency"`
	MaxLatency       time.Duration `json:"max_latency"`
	
	// 代理统计
	TotalProxies     int `json:"total_proxies"`
	HealthyProxies   int `json:"healthy_proxies"`
	UnhealthyProxies int `json:"unhealthy_proxies"`
	
	// 成本统计
	TotalCost        float64 `json:"total_cost"`
	AverageCost      float64 `json:"average_cost"`
	
	// 时间戳
	LastUpdated      time.Time `json:"last_updated"`
	
	mu sync.RWMutex
}

// NewProxyMetrics 创建代理性能指标
func NewProxyMetrics() *ProxyMetrics {
	return &ProxyMetrics{
		MinLatency: time.Hour, // 初始化为最大值
		LastUpdated: time.Now(),
	}
}

// RecordRequest 记录请求
func (pm *ProxyMetrics) RecordRequest(success bool, latency time.Duration, cost float64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.TotalRequests++
	if success {
		pm.SuccessRequests++
	} else {
		pm.FailedRequests++
	}
	
	pm.TotalLatency += latency
	if pm.TotalRequests > 0 {
		pm.AverageLatency = pm.TotalLatency / time.Duration(pm.TotalRequests)
	}
	
	if latency < pm.MinLatency {
		pm.MinLatency = latency
	}
	if latency > pm.MaxLatency {
		pm.MaxLatency = latency
	}
	
	pm.TotalCost += cost
	if pm.TotalRequests > 0 {
		pm.AverageCost = pm.TotalCost / float64(pm.TotalRequests)
	}
	
	pm.LastUpdated = time.Now()
}

// UpdateProxyHealth 更新代理健康状态
func (pm *ProxyMetrics) UpdateProxyHealth(total, healthy, unhealthy int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.TotalProxies = total
	pm.HealthyProxies = healthy
	pm.UnhealthyProxies = unhealthy
	pm.LastUpdated = time.Now()
}

// GetStats 获取统计信息
func (pm *ProxyMetrics) GetStats() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	return map[string]interface{}{
		"total_requests":      pm.TotalRequests,
		"success_requests":   pm.SuccessRequests,
		"failed_requests":    pm.FailedRequests,
		"success_rate":       pm.GetSuccessRate(),
		"average_latency_ms": pm.AverageLatency.Milliseconds(),
		"min_latency_ms":     pm.MinLatency.Milliseconds(),
		"max_latency_ms":     pm.MaxLatency.Milliseconds(),
		"total_proxies":      pm.TotalProxies,
		"healthy_proxies":    pm.HealthyProxies,
		"unhealthy_proxies":  pm.UnhealthyProxies,
		"health_rate":        pm.GetHealthRate(),
		"total_cost":         pm.TotalCost,
		"average_cost":       pm.AverageCost,
		"last_updated":       pm.LastUpdated.Format(time.RFC3339),
	}
}

// GetSuccessRate 获取成功率
func (pm *ProxyMetrics) GetSuccessRate() float64 {
	if pm.TotalRequests == 0 {
		return 0
	}
	return float64(pm.SuccessRequests) / float64(pm.TotalRequests) * 100
}

// GetHealthRate 获取健康率
func (pm *ProxyMetrics) GetHealthRate() float64 {
	if pm.TotalProxies == 0 {
		return 0
	}
	return float64(pm.HealthyProxies) / float64(pm.TotalProxies) * 100
}

// Reset 重置统计数据
func (pm *ProxyMetrics) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.TotalRequests = 0
	pm.SuccessRequests = 0
	pm.FailedRequests = 0
	pm.TotalLatency = 0
	pm.AverageLatency = 0
	pm.MinLatency = time.Hour
	pm.MaxLatency = 0
	pm.TotalProxies = 0
	pm.HealthyProxies = 0
	pm.UnhealthyProxies = 0
	pm.TotalCost = 0
	pm.AverageCost = 0
	pm.LastUpdated = time.Now()
}

// ProxyMetricsCollector 代理指标收集器
type ProxyMetricsCollector struct {
	metrics map[string]*ProxyMetrics
	mu      sync.RWMutex
}

// NewProxyMetricsCollector 创建代理指标收集器
func NewProxyMetricsCollector() *ProxyMetricsCollector {
	return &ProxyMetricsCollector{
		metrics: make(map[string]*ProxyMetrics),
	}
}

// GetMetrics 获取指定代理的指标
func (pmc *ProxyMetricsCollector) GetMetrics(proxyID string) *ProxyMetrics {
	pmc.mu.RLock()
	defer pmc.mu.RUnlock()
	
	metrics, exists := pmc.metrics[proxyID]
	if !exists {
		return NewProxyMetrics()
	}
	return metrics
}

// RecordRequest 记录请求
func (pmc *ProxyMetricsCollector) RecordRequest(proxyID string, success bool, latency time.Duration, cost float64) {
	pmc.mu.Lock()
	defer pmc.mu.Unlock()
	
	metrics, exists := pmc.metrics[proxyID]
	if !exists {
		metrics = NewProxyMetrics()
		pmc.metrics[proxyID] = metrics
	}
	
	metrics.RecordRequest(success, latency, cost)
}

// GetAllMetrics 获取所有指标
func (pmc *ProxyMetricsCollector) GetAllMetrics() map[string]map[string]interface{} {
	pmc.mu.RLock()
	defer pmc.mu.RUnlock()
	
	result := make(map[string]map[string]interface{})
	for proxyID, metrics := range pmc.metrics {
		result[proxyID] = metrics.GetStats()
	}
	return result
}

// GetSummary 获取汇总统计
func (pmc *ProxyMetricsCollector) GetSummary() map[string]interface{} {
	pmc.mu.RLock()
	defer pmc.mu.RUnlock()
	
	totalRequests := int64(0)
	totalSuccess := int64(0)
	totalFailed := int64(0)
	totalLatency := time.Duration(0)
	totalCost := 0.0
	totalProxies := 0
	healthyProxies := 0
	
	for _, metrics := range pmc.metrics {
		totalRequests += metrics.TotalRequests
		totalSuccess += metrics.SuccessRequests
		totalFailed += metrics.FailedRequests
		totalLatency += metrics.TotalLatency
		totalCost += metrics.TotalCost
		totalProxies += metrics.TotalProxies
		healthyProxies += metrics.HealthyProxies
	}
	
	avgLatency := time.Duration(0)
	if totalRequests > 0 {
		avgLatency = totalLatency / time.Duration(totalRequests)
	}
	
	avgCost := 0.0
	if totalRequests > 0 {
		avgCost = totalCost / float64(totalRequests)
	}
	
	successRate := 0.0
	if totalRequests > 0 {
		successRate = float64(totalSuccess) / float64(totalRequests) * 100
	}
	
	healthRate := 0.0
	if totalProxies > 0 {
		healthRate = float64(healthyProxies) / float64(totalProxies) * 100
	}
	
	return map[string]interface{}{
		"total_requests":      totalRequests,
		"success_requests":    totalSuccess,
		"failed_requests":     totalFailed,
		"success_rate":        successRate,
		"average_latency_ms":  avgLatency.Milliseconds(),
		"total_cost":          totalCost,
		"average_cost":        avgCost,
		"total_proxies":       totalProxies,
		"healthy_proxies":     healthyProxies,
		"health_rate":         healthRate,
	}
}
