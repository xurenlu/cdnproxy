// Package proxy 智能成本优化算法
// 作者: rocky<m@some.im>

package proxy

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
	
	"cdnproxy/internal/proxy/providers"
)

// CostOptimizer 成本优化器
type CostOptimizer struct {
	providers map[string]providers.ResidentialProxyProvider
	metrics   *ProxyMetricsCollector
	mu        sync.RWMutex
	
	// 优化策略
	strategy CostOptimizationStrategy
}

// CostOptimizationStrategy 成本优化策略
type CostOptimizationStrategy struct {
	// 预算限制
	DailyBudget   float64 `json:"daily_budget"`
	MonthlyBudget float64 `json:"monthly_budget"`
	
	// 优先级权重
	CostWeight    float64 `json:"cost_weight"`    // 成本权重
	QualityWeight float64 `json:"quality_weight"` // 质量权重
	LatencyWeight float64 `json:"latency_weight"` // 延迟权重
	
	// 优化模式
	Mode string `json:"mode"` // "cost_first", "quality_first", "balanced"
}

// NewCostOptimizer 创建成本优化器
func NewCostOptimizer(providers map[string]providers.ResidentialProxyProvider, metrics *ProxyMetricsCollector) *CostOptimizer {
	return &CostOptimizer{
		providers: providers,
		metrics:   metrics,
		strategy: CostOptimizationStrategy{
			DailyBudget:    100.0,
			MonthlyBudget:  3000.0,
			CostWeight:     0.4,
			QualityWeight:  0.3,
			LatencyWeight:  0.3,
			Mode:           "balanced",
		},
	}
}

// SelectBestProxy 选择最佳代理（基于成本优化）
func (co *CostOptimizer) SelectBestProxy(ctx context.Context, targetAPI string) (*providers.ResidentialProxy, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	
	var bestProxy *providers.ResidentialProxy
	var bestScore float64
	
	for _, provider := range co.providers {
		proxy, err := provider.GetProxy(ctx)
		if err != nil {
			continue
		}
		
		// 计算综合评分
		score := co.calculateOptimizedScore(proxy, targetAPI)
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

// calculateOptimizedScore 计算优化评分
func (co *CostOptimizer) calculateOptimizedScore(proxy *providers.ResidentialProxy, targetAPI string) float64 {
	score := 0.0
	
	// 1. 成本评分 (0-100分，成本越低评分越高)
	costScore := co.calculateCostScore(proxy)
	score += costScore * co.strategy.CostWeight
	
	// 2. 质量评分 (0-100分)
	qualityScore := float64(proxy.Quality) * 10
	score += qualityScore * co.strategy.QualityWeight
	
	// 3. 延迟评分 (0-100分，基于历史延迟)
	latencyScore := co.calculateLatencyScore(proxy)
	score += latencyScore * co.strategy.LatencyWeight
	
	// 4. 成功率评分 (0-100分)
	successScore := proxy.SuccessRate * 100
	score += successScore * 0.1 // 10%权重
	
	return score
}

// calculateCostScore 计算成本评分
func (co *CostOptimizer) calculateCostScore(proxy *providers.ResidentialProxy) float64 {
	co.mu.RLock()
	provider, exists := co.providers[proxy.ISP]
	co.mu.RUnlock()
	
	if !exists {
		return 50 // 默认分数
	}
	
	cost := provider.GetCost()
	
	// 计算总成本
	totalCost := cost.PerRequest + (cost.PerGB / 1024 / 1024) + (cost.PerHour / 3600)
	
	// 归一化到0-100分（假设最高成本为$0.1）
	maxCost := 0.1
	normalizedCost := totalCost / maxCost
	if normalizedCost > 1 {
		normalizedCost = 1
	}
	
	// 成本越低评分越高
	return (1 - normalizedCost) * 100
}

// calculateLatencyScore 计算延迟评分
func (co *CostOptimizer) calculateLatencyScore(proxy *providers.ResidentialProxy) float64 {
	metrics := co.metrics.GetMetrics(proxy.ID)
	if metrics.TotalRequests == 0 {
		return 50 // 默认分数
	}
	
	avgLatency := metrics.AverageLatency
	
	// 归一化到0-100分（假设最差延迟为5秒）
	maxLatency := 5 * time.Second
	normalizedLatency := float64(avgLatency) / float64(maxLatency)
	if normalizedLatency > 1 {
		normalizedLatency = 1
	}
	
	// 延迟越低评分越高
	return (1 - normalizedLatency) * 100
}

// CheckBudget 检查预算
func (co *CostOptimizer) CheckBudget() (bool, float64, error) {
	summary := co.metrics.GetSummary()
	
	totalCost, ok := summary["total_cost"].(float64)
	if !ok {
		return true, 0, nil
	}
	
	// 检查每日预算
	if totalCost > co.strategy.DailyBudget {
		return false, totalCost, fmt.Errorf("daily budget exceeded: %.2f > %.2f", totalCost, co.strategy.DailyBudget)
	}
	
	return true, totalCost, nil
}

// GetCostRecommendations 获取成本优化建议
func (co *CostOptimizer) GetCostRecommendations() []string {
	recommendations := []string{}
	
	summary := co.metrics.GetSummary()
	totalCost, ok := summary["total_cost"].(float64)
	if !ok {
		return recommendations
	}
	
	// 成本分析
	if totalCost > co.strategy.DailyBudget*0.8 {
		recommendations = append(recommendations, "⚠️ 今日成本接近预算限制，建议切换到更便宜的代理")
	}
	
	// 质量分析
	successRate, ok := summary["success_rate"].(float64)
	if ok && successRate < 80 {
		recommendations = append(recommendations, "⚠️ 成功率较低，建议切换到更高质量的代理")
	}
	
	// 延迟分析
	avgLatency, ok := summary["average_latency_ms"].(int64)
	if ok && avgLatency > 3000 {
		recommendations = append(recommendations, "⚠️ 平均延迟较高，建议切换到更快的代理")
	}
	
	return recommendations
}

// OptimizeProxySelection 优化代理选择策略
func (co *CostOptimizer) OptimizeProxySelection() {
	co.mu.Lock()
	defer co.mu.Unlock()
	
	summary := co.metrics.GetSummary()
	
	// 根据统计调整策略
	cost, ok := summary["total_cost"].(float64)
	if ok && cost > co.strategy.DailyBudget*0.7 {
		// 成本偏高，提高成本权重
		co.strategy.CostWeight = 0.6
		co.strategy.QualityWeight = 0.2
		co.strategy.LatencyWeight = 0.2
		co.strategy.Mode = "cost_first"
	}
	
	successRate, ok := summary["success_rate"].(float64)
	if ok && successRate < 80 {
		// 成功率偏低，提高质量权重
		co.strategy.QualityWeight = 0.5
		co.strategy.CostWeight = 0.3
		co.strategy.LatencyWeight = 0.2
		co.strategy.Mode = "quality_first"
	}
	
	avgLatency, ok := summary["average_latency_ms"].(int64)
	if ok && avgLatency < 1000 && cost < co.strategy.DailyBudget*0.5 {
		// 延迟和成本都很好，均衡模式
		co.strategy.CostWeight = 0.4
		co.strategy.QualityWeight = 0.3
		co.strategy.LatencyWeight = 0.3
		co.strategy.Mode = "balanced"
	}
}

// GetProviderRanking 获取提供者排名
func (co *CostOptimizer) GetProviderRanking() []ProviderRanking {
	co.mu.RLock()
	defer co.mu.RUnlock()
	
	rankings := []ProviderRanking{}
	
	for name, provider := range co.providers {
		cost := provider.GetCost()
		metrics := co.metrics.GetSummary()
		
		ranking := ProviderRanking{
			Name:            name,
			CostPerRequest:  cost.PerRequest,
			CostPerGB:       cost.PerGB,
			AverageLatency:  int64(metrics["average_latency_ms"].(int64)),
			SuccessRate:     metrics["success_rate"].(float64),
			Score:           co.calculateProviderScore(name, cost, metrics),
		}
		
		rankings = append(rankings, ranking)
	}
	
	// 按评分排序
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Score > rankings[j].Score
	})
	
	return rankings
}

// ProviderRanking 提供者排名
type ProviderRanking struct {
	Name            string  `json:"name"`
	CostPerRequest  float64 `json:"cost_per_request"`
	CostPerGB       float64 `json:"cost_per_gb"`
	AverageLatency  int64   `json:"average_latency_ms"`
	SuccessRate     float64 `json:"success_rate"`
	Score           float64 `json:"score"`
}

// calculateProviderScore 计算提供者评分
func (co *CostOptimizer) calculateProviderScore(name string, cost *providers.ProxyCost, metrics map[string]interface{}) float64 {
	score := 0.0
	
	// 成本评分
	totalCost := cost.PerRequest + (cost.PerGB / 1024 / 1024)
	costScore := (1 - totalCost/0.1) * 100
	if costScore < 0 {
		costScore = 0
	}
	score += costScore * co.strategy.CostWeight
	
	// 延迟评分
	avgLatency, ok := metrics["average_latency_ms"].(int64)
	if ok {
		latencyScore := (1 - float64(avgLatency)/5000) * 100
		if latencyScore < 0 {
			latencyScore = 0
		}
		score += latencyScore * co.strategy.LatencyWeight
	}
	
	// 成功率评分
	successRate, ok := metrics["success_rate"].(float64)
	if ok {
		score += successRate * co.strategy.QualityWeight
	}
	
	return score
}

// UpdateStrategy 更新优化策略
func (co *CostOptimizer) UpdateStrategy(strategy CostOptimizationStrategy) {
	co.mu.Lock()
	defer co.mu.Unlock()
	
	co.strategy = strategy
}

// GetStrategy 获取当前策略
func (co *CostOptimizer) GetStrategy() CostOptimizationStrategy {
	co.mu.RLock()
	defer co.mu.RUnlock()
	
	return co.strategy
}
