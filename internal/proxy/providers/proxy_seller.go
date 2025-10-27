// Package providers Proxy-Seller 住宅IP代理提供者
// 作者: rocky<m@some.im>

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ProxySellerProvider Proxy-Seller 住宅IP代理提供者
type ProxySellerProvider struct {
	APIKey     string
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// ProxySellerResponse Proxy-Seller API响应
type ProxySellerResponse struct {
	Status string `json:"status"`
	Data   struct {
		Proxies []ProxySellerProxy `json:"proxies"`
	} `json:"data"`
}

// ProxySellerProxy Proxy-Seller 代理信息
type ProxySellerProxy struct {
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

// NewProxySellerProvider 创建 Proxy-Seller 提供者
func NewProxySellerProvider(apiKey, username, password string) *ProxySellerProvider {
	return &ProxySellerProvider{
		APIKey:   apiKey,
		BaseURL:  "https://api.proxy-seller.com",
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetProxy 获取一个可用的代理
func (psp *ProxySellerProvider) GetProxy(ctx context.Context) (*ResidentialProxy, error) {
	proxies, err := psp.GetProxyList(ctx)
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
func (psp *ProxySellerProvider) GetProxyList(ctx context.Context) ([]*ResidentialProxy, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", psp.BaseURL+"/proxy/list", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+psp.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := psp.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var apiResp ProxySellerResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	proxies := make([]*ResidentialProxy, 0, len(apiResp.Data.Proxies))
	for _, psProxy := range apiResp.Data.Proxies {
		proxy := &ResidentialProxy{
			ID:          psProxy.ID,
			IP:          psProxy.IP,
			Port:        psProxy.Port,
			Username:    psProxy.Username,
			Password:    psProxy.Password,
			Location:    psProxy.Location,
			Country:     psProxy.Country,
			City:        psProxy.City,
			ISP:         psProxy.ISP,
			Type:        psProxy.Type,
			Quality:     psProxy.Quality,
			SuccessRate: psProxy.SuccessRate,
			LastUsed:    time.Now(),
			Metadata: map[string]string{
				"provider": "proxy_seller",
				"api_key":  psp.APIKey,
			},
		}
		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

// ReportUsage 报告使用情况
func (psp *ProxySellerProvider) ReportUsage(proxy *ResidentialProxy, success bool, latency time.Duration) error {
	// Proxy-Seller 通常不需要手动报告使用情况
	return nil
}

// GetName 获取提供者名称
func (psp *ProxySellerProvider) GetName() string {
	return "proxy_seller"
}

// GetCost 获取成本信息
func (psp *ProxySellerProvider) GetCost() *ProxyCost {
	return &ProxyCost{
		PerRequest: 0.0005, // $0.0005 per request
		PerGB:      0.20,   // $0.20 per GB
		PerHour:    0.05,   // $0.05 per hour
		Currency:   "USD",
	}
}
