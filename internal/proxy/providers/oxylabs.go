// Package providers Oxylabs 住宅IP代理提供者
// 作者: rocky<m@some.im>

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OxylabsProvider Oxylabs 住宅IP代理提供者
type OxylabsProvider struct {
	APIKey     string
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// OxylabsResponse Oxylabs API响应
type OxylabsResponse struct {
	Status string `json:"status"`
	Data   struct {
		Proxies []OxylabsProxy `json:"proxies"`
	} `json:"data"`
}

// OxylabsProxy Oxylabs 代理信息
type OxylabsProxy struct {
	ID          string `json:"id"`
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Location    string `json:"location"`
	Country     string `json:"country"`
	City        string `json:"city"`
	ISP         string `json:"isp"`
	Type        string `json:"type"`
	Quality     int    `json:"quality"`
	SuccessRate float64 `json:"success_rate"`
}

// NewOxylabsProvider 创建 Oxylabs 提供者
func NewOxylabsProvider(apiKey, username, password string) *OxylabsProvider {
	return &OxylabsProvider{
		APIKey:   apiKey,
		BaseURL:  "https://api.oxylabs.io",
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetProxy 获取一个可用的代理
func (op *OxylabsProvider) GetProxy(ctx context.Context) (*ResidentialProxy, error) {
	proxies, err := op.GetProxyList(ctx)
	if err != nil {
		return nil, err
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("no proxies available")
	}

	// 选择最佳代理
	bestProxy := proxies[0]
	for _, proxy := range proxies {
		if proxy.Quality > bestProxy.Quality && proxy.SuccessRate > bestProxy.SuccessRate {
			bestProxy = proxy
		}
	}

	return bestProxy, nil
}

// GetProxyList 获取代理列表
func (op *OxylabsProvider) GetProxyList(ctx context.Context) ([]*ResidentialProxy, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", op.BaseURL+"/proxy/list", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+op.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := op.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var apiResp OxylabsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	proxies := make([]*ResidentialProxy, 0, len(apiResp.Data.Proxies))
	for _, oxyProxy := range apiResp.Data.Proxies {
		proxy := &ResidentialProxy{
			ID:          oxyProxy.ID,
			IP:          oxyProxy.IP,
			Port:        oxyProxy.Port,
			Username:    oxyProxy.Username,
			Password:    oxyProxy.Password,
			Location:    oxyProxy.Location,
			Country:     oxyProxy.Country,
			City:        oxyProxy.City,
			ISP:         oxyProxy.ISP,
			Type:        oxyProxy.Type,
			Quality:     oxyProxy.Quality,
			SuccessRate: oxyProxy.SuccessRate,
			LastUsed:    time.Now(),
			Metadata: map[string]string{
				"provider": "oxylabs",
				"api_key":  op.APIKey,
			},
		}
		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

// ReportUsage 报告使用情况
func (op *OxylabsProvider) ReportUsage(proxy *ResidentialProxy, success bool, latency time.Duration) error {
	// Oxylabs 通常不需要手动报告使用情况
	return nil
}

// GetName 获取提供者名称
func (op *OxylabsProvider) GetName() string {
	return "oxylabs"
}

// GetCost 获取成本信息
func (op *OxylabsProvider) GetCost() *ProxyCost {
	return &ProxyCost{
		PerRequest: 0.002, // $0.002 per request
		PerGB:      1.00,  // $1.00 per GB
		PerHour:    0.20,  // $0.20 per hour
		Currency:   "USD",
	}
}
