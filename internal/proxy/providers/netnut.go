// Package providers NetNut 住宅IP代理提供者
// 作者: rocky<m@some.im>

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// NetNutProvider NetNut 住宅IP代理提供者
type NetNutProvider struct {
	APIKey     string
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// NetNutResponse NetNut API响应
type NetNutResponse struct {
	Status string `json:"status"`
	Data   struct {
		Proxies []NetNutProxy `json:"proxies"`
	} `json:"data"`
}

// NetNutProxy NetNut 代理信息
type NetNutProxy struct {
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

// NewNetNutProvider 创建 NetNut 提供者
func NewNetNutProvider(apiKey, username, password string) *NetNutProvider {
	return &NetNutProvider{
		APIKey:   apiKey,
		BaseURL:  "https://api.netnut.io",
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetProxy 获取一个可用的代理
func (nn *NetNutProvider) GetProxy(ctx context.Context) (*ResidentialProxy, error) {
	proxies, err := nn.GetProxyList(ctx)
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
func (nn *NetNutProvider) GetProxyList(ctx context.Context) ([]*ResidentialProxy, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", nn.BaseURL+"/proxy/list", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+nn.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := nn.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var apiResp NetNutResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	proxies := make([]*ResidentialProxy, 0, len(apiResp.Data.Proxies))
	for _, nnProxy := range apiResp.Data.Proxies {
		proxy := &ResidentialProxy{
			ID:          nnProxy.ID,
			IP:          nnProxy.IP,
			Port:        nnProxy.Port,
			Username:    nnProxy.Username,
			Password:    nnProxy.Password,
			Location:    nnProxy.Location,
			Country:     nnProxy.Country,
			City:        nnProxy.City,
			ISP:         nnProxy.ISP,
			Type:        nnProxy.Type,
			Quality:     nnProxy.Quality,
			SuccessRate: nnProxy.SuccessRate,
			LastUsed:    time.Now(),
			Metadata: map[string]string{
				"provider": "netnut",
				"api_key":  nn.APIKey,
			},
		}
		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

// ReportUsage 报告使用情况
func (nn *NetNutProvider) ReportUsage(proxy *ResidentialProxy, success bool, latency time.Duration) error {
	// NetNut 通常不需要手动报告使用情况
	return nil
}

// GetName 获取提供者名称
func (nn *NetNutProvider) GetName() string {
	return "netnut"
}

// GetCost 获取成本信息
func (nn *NetNutProvider) GetCost() *ProxyCost {
	return &ProxyCost{
		PerRequest: 0.0012, // $0.0012 per request
		PerGB:      0.60,   // $0.60 per GB
		PerHour:    0.12,   // $0.12 per hour
		Currency:   "USD",
	}
}
