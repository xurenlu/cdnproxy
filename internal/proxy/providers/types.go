// Package providers 住宅IP代理提供者类型定义
// 作者: rocky<m@some.im>

package providers

import (
	"context"
	"time"
)

// ResidentialProxyProvider 住宅IP代理提供者接口
type ResidentialProxyProvider interface {
	// GetProxy 获取一个可用的代理
	GetProxy(ctx context.Context) (*ResidentialProxy, error)

	// GetProxyList 获取代理列表
	GetProxyList(ctx context.Context) ([]*ResidentialProxy, error)

	// ReportUsage 报告使用情况
	ReportUsage(proxy *ResidentialProxy, success bool, latency time.Duration) error

	// GetName 获取提供者名称
	GetName() string

	// GetCost 获取成本信息
	GetCost() *ProxyCost
}

// ResidentialProxy 住宅IP代理
type ResidentialProxy struct {
	ID          string            `json:"id"`
	IP          string            `json:"ip"`
	Port        int               `json:"port"`
	Username    string            `json:"username"`
	Password    string            `json:"password"`
	Location    string            `json:"location"`
	Country     string            `json:"country"`
	City        string            `json:"city"`
	ISP         string            `json:"isp"`
	Type        string            `json:"type"`    // residential, mobile, datacenter
	Quality     int               `json:"quality"` // 1-10
	SuccessRate float64           `json:"success_rate"`
	LastUsed    time.Time         `json:"last_used"`
	Metadata    map[string]string `json:"metadata"`
}

// ProxyCost 代理成本信息
type ProxyCost struct {
	PerRequest float64 `json:"per_request"` // 每次请求成本
	PerGB      float64 `json:"per_gb"`      // 每GB成本
	PerHour    float64 `json:"per_hour"`    // 每小时成本
	Currency   string  `json:"currency"`    // 货币
}
