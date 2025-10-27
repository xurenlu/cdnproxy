// Package providers Bright Data 住宅IP代理提供者
// 作者: rocky<m@some.im>

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// BrightDataProvider Bright Data 住宅IP代理提供者
type BrightDataProvider struct {
	APIKey     string
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// BrightDataResponse Bright Data API响应
type BrightDataResponse struct {
	Status string `json:"status"`
	Data   struct {
		Proxies []BrightDataProxy `json:"proxies"`
	} `json:"data"`
}

// BrightDataProxy Bright Data 代理信息
type BrightDataProxy struct {
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

// NewBrightDataProvider 创建 Bright Data 提供者
func NewBrightDataProvider(apiKey, username, password string) *BrightDataProvider {
	return &BrightDataProvider{
		APIKey:   apiKey,
		BaseURL:  "https://api.brightdata.com",
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetProxy 获取一个可用的代理
func (bd *BrightDataProvider) GetProxy(ctx context.Context) (*ResidentialProxy, error) {
	proxies, err := bd.GetProxyList(ctx)
	if err != nil {
		return nil, err
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("no proxies available")
	}

	// 选择最佳代理（这里简化处理，实际应该根据质量、成功率等选择）
	bestProxy := proxies[0]
	for _, proxy := range proxies {
		if proxy.Quality > bestProxy.Quality {
			bestProxy = proxy
		}
	}

	return bestProxy, nil
}

// GetProxyList 获取代理列表
func (bd *BrightDataProvider) GetProxyList(ctx context.Context) ([]*ResidentialProxy, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", bd.BaseURL+"/proxy/list", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+bd.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := bd.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var apiResp BrightDataResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	proxies := make([]*ResidentialProxy, 0, len(apiResp.Data.Proxies))
	for _, bdProxy := range apiResp.Data.Proxies {
		proxy := &ResidentialProxy{
			ID:          bdProxy.ID,
			IP:          bdProxy.IP,
			Port:        bdProxy.Port,
			Username:    bdProxy.Username,
			Password:    bdProxy.Password,
			Location:    bdProxy.Location,
			Country:     bdProxy.Country,
			City:        bdProxy.City,
			ISP:         bdProxy.ISP,
			Type:        bdProxy.Type,
			Quality:     bdProxy.Quality,
			SuccessRate: bdProxy.SuccessRate,
			LastUsed:    time.Now(),
			Metadata: map[string]string{
				"provider": "bright_data",
				"api_key":  bd.APIKey,
			},
		}
		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

// ReportUsage 报告使用情况
func (bd *BrightDataProvider) ReportUsage(proxy *ResidentialProxy, success bool, latency time.Duration) error {
	// Bright Data 通常不需要手动报告使用情况
	// 这里可以记录到本地日志或数据库
	return nil
}

// GetName 获取提供者名称
func (bd *BrightDataProvider) GetName() string {
	return "bright_data"
}

// GetCost 获取成本信息
func (bd *BrightDataProvider) GetCost() *ProxyCost {
	return &ProxyCost{
		PerRequest: 0.001, // $0.001 per request
		PerGB:      0.50,  // $0.50 per GB
		PerHour:    0.10,  // $0.10 per hour
		Currency:   "USD",
	}
}
