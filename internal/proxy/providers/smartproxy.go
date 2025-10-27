// Package providers Smartproxy 住宅IP代理提供者
// 作者: rocky<m@some.im>

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SmartproxyProvider Smartproxy 住宅IP代理提供者
type SmartproxyProvider struct {
	APIKey     string
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// SmartproxyResponse Smartproxy API响应
type SmartproxyResponse struct {
	Status string `json:"status"`
	Data   struct {
		Proxies []SmartproxyProxy `json:"proxies"`
	} `json:"data"`
}

// SmartproxyProxy Smartproxy 代理信息
type SmartproxyProxy struct {
	ID          string  `json:"id"`
	IP          string  `json:"ip"`
	Port        int     `json:"port"`
	Username    string  `json:"username"`
	Password    string  `json:"password"`
	Location    string  `json:"location"`
	Country     string  `json:"country"`
	City        string  `json:"city"`
	ISP         string  `json:"isp"`
	Type        string  `json:"type"`
	Quality     int     `json:"quality"`
	SuccessRate float64 `json:"success_rate"`
}

// NewSmartproxyProvider 创建 Smartproxy 提供者
func NewSmartproxyProvider(apiKey, username, password string) *SmartproxyProvider {
	return &SmartproxyProvider{
		APIKey:   apiKey,
		BaseURL:  "https://api.smartproxy.com",
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetProxy 获取一个可用的代理
func (sp *SmartproxyProvider) GetProxy(ctx context.Context) (*ResidentialProxy, error) {
	proxies, err := sp.GetProxyList(ctx)
	if err != nil {
		return nil, err
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("no proxies available")
	}

	// 选择最佳代理
	bestProxy := proxies[0]
	for _, proxy := range proxies {
		if proxy.SuccessRate > bestProxy.SuccessRate {
			bestProxy = proxy
		}
	}

	return bestProxy, nil
}

// GetProxyList 获取代理列表
func (sp *SmartproxyProvider) GetProxyList(ctx context.Context) ([]*ResidentialProxy, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", sp.BaseURL+"/proxy/list", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+sp.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := sp.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var apiResp SmartproxyResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	proxies := make([]*ResidentialProxy, 0, len(apiResp.Data.Proxies))
	for _, spProxy := range apiResp.Data.Proxies {
		proxy := &ResidentialProxy{
			ID:          spProxy.ID,
			IP:          spProxy.IP,
			Port:        spProxy.Port,
			Username:    spProxy.Username,
			Password:    spProxy.Password,
			Location:    spProxy.Location,
			Country:     spProxy.Country,
			City:        spProxy.City,
			ISP:         spProxy.ISP,
			Type:        spProxy.Type,
			Quality:     spProxy.Quality,
			SuccessRate: spProxy.SuccessRate,
			LastUsed:    time.Now(),
			Metadata: map[string]string{
				"provider": "smartproxy",
				"api_key":  sp.APIKey,
			},
		}
		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

// ReportUsage 报告使用情况
func (sp *SmartproxyProvider) ReportUsage(proxy *ResidentialProxy, success bool, latency time.Duration) error {
	// Smartproxy 通常不需要手动报告使用情况
	return nil
}

// GetName 获取提供者名称
func (sp *SmartproxyProvider) GetName() string {
	return "smartproxy"
}

// GetCost 获取成本信息
func (sp *SmartproxyProvider) GetCost() *ProxyCost {
	return &ProxyCost{
		PerRequest: 0.0008, // $0.0008 per request
		PerGB:      0.40,   // $0.40 per GB
		PerHour:    0.08,   // $0.08 per hour
		Currency:   "USD",
	}
}
