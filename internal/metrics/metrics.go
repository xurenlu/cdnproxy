package metrics

import (
	"sync"
	"time"
)

// Metrics 性能指标收集器
type Metrics struct {
	mu sync.RWMutex
	
	// 请求统计
	TotalRequests    int64
	SuccessfulRequests int64
	FailedRequests   int64
	
	// 响应时间统计
	TotalResponseTime time.Duration
	MinResponseTime   time.Duration
	MaxResponseTime   time.Duration
	
	// 缓存统计
	CacheHits   int64
	CacheMisses int64
	
	// 并发统计
	ActiveConnections int64
	MaxConcurrent     int64
	
	// 错误统计
	ErrorCounts map[string]int64
	
	// 启动时间
	StartTime time.Time
}

var globalMetrics = &Metrics{
	ErrorCounts: make(map[string]int64),
	StartTime:   time.Now(),
}

// GetGlobalMetrics 获取全局指标
func GetGlobalMetrics() *Metrics {
	return globalMetrics
}

// RecordRequest 记录请求
func (m *Metrics) RecordRequest(success bool, responseTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.TotalRequests++
	if success {
		m.SuccessfulRequests++
	} else {
		m.FailedRequests++
	}
	
	// 更新响应时间统计
	m.TotalResponseTime += responseTime
	if m.MinResponseTime == 0 || responseTime < m.MinResponseTime {
		m.MinResponseTime = responseTime
	}
	if responseTime > m.MaxResponseTime {
		m.MaxResponseTime = responseTime
	}
}

// RecordCacheHit 记录缓存命中
func (m *Metrics) RecordCacheHit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CacheHits++
}

// RecordCacheMiss 记录缓存未命中
func (m *Metrics) RecordCacheMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CacheMisses++
}

// RecordError 记录错误
func (m *Metrics) RecordError(errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ErrorCounts[errorType]++
}

// RecordConnection 记录连接变化
func (m *Metrics) RecordConnection(delta int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveConnections += delta
	if m.ActiveConnections > m.MaxConcurrent {
		m.MaxConcurrent = m.ActiveConnections
	}
}

// GetStats 获取统计信息
func (m *Metrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	avgResponseTime := time.Duration(0)
	if m.TotalRequests > 0 {
		avgResponseTime = m.TotalResponseTime / time.Duration(m.TotalRequests)
	}
	
	cacheHitRate := float64(0)
	if m.CacheHits+m.CacheMisses > 0 {
		cacheHitRate = float64(m.CacheHits) / float64(m.CacheHits+m.CacheMisses) * 100
	}
	
	successRate := float64(0)
	if m.TotalRequests > 0 {
		successRate = float64(m.SuccessfulRequests) / float64(m.TotalRequests) * 100
	}
	
	return map[string]interface{}{
		"total_requests":        m.TotalRequests,
		"successful_requests":   m.SuccessfulRequests,
		"failed_requests":       m.FailedRequests,
		"success_rate":          successRate,
		"avg_response_time_ms":  avgResponseTime.Milliseconds(),
		"min_response_time_ms":  m.MinResponseTime.Milliseconds(),
		"max_response_time_ms":  m.MaxResponseTime.Milliseconds(),
		"cache_hits":            m.CacheHits,
		"cache_misses":          m.CacheMisses,
		"cache_hit_rate":        cacheHitRate,
		"active_connections":    m.ActiveConnections,
		"max_concurrent":        m.MaxConcurrent,
		"error_counts":          m.ErrorCounts,
		"uptime_seconds":        time.Since(m.StartTime).Seconds(),
	}
}

// Reset 重置指标
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.TotalRequests = 0
	m.SuccessfulRequests = 0
	m.FailedRequests = 0
	m.TotalResponseTime = 0
	m.MinResponseTime = 0
	m.MaxResponseTime = 0
	m.CacheHits = 0
	m.CacheMisses = 0
	m.ActiveConnections = 0
	m.MaxConcurrent = 0
	m.ErrorCounts = make(map[string]int64)
	m.StartTime = time.Now()
}
