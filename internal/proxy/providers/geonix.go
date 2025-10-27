// Package providers Geonix 住宅IP代理提供者
// 作者: rocky<m@some.im>

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GeonixProvider Geonix 住宅IP代理提供者
type GeonixProvider struct {
	APIKey     string
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// GeonixResponse Geonix API响应
type GeonixResponse struct {
	Status string `json:"status"`
	Data   struct {
		Proxies []GeonixProxy `json:"proxies"`
	} `json:"data"`
}

// GeonixProxy Geonix 代理信息
type GeonixProxy struct {
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

// NewGeonixProvider 创建 Geonix 提供者
func NewGeonixProvider(apiKey, username, password string) *GeonixProvider {
	return &GeonixProvider{
		APIKey:   apiKey,
		BaseURL:  "https://api.geonix.io",
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetProxy 获取一个可用的代理
func (gp *GeonixProvider) GetProxy(ctx context.Context) (*ResidentialProxy, error) {
	proxies, err := gp.GetProxyList(ctx)
	if err != nil {
		return nil, err
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("no proxies available")
	}

	// 选择最佳代理
	bestProxy := proxies[0]
	for _, proxy := range proxies {
		if proxy.Quality > bestProxy.Quality {
			bestProxy = proxy
		}
	}

	return bestProxy, nil
}

// GetProxyList 获取代理列表
func (gp *GeonixProvider) GetProxyList(ctx context.Context) ([]*ResidentialProxy, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", gp.BaseURL+"/proxy/list", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+gp.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := gp.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var apiResp GeonixResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	proxies := make([]*ResidentialProxy, 0, len(apiResp.Data.Proxies))
	for _, gProxy := range apiResp.Data.Proxies {
		proxy := &ResidentialProxy{
			ID:          gProxy.ID,
			IP:          gProxy.IP,
			Port:        gProxy.Port,
			Username:    gProxy.Username,
			Password:    gProxy.Password,
			Location:    gProxy.Location,
			Country:     gProxy.Country,
			City:        gProxy.City,
			ISP:         gProxy.ISP,
			Type:        gProxy.Type,
			Quality:     gProxy.Quality,
			SuccessRate: gProxy.SuccessRate,
			LastUsed:    time.Now(),
			Metadata: map[string]string{
				"provider": "geonix",
				"api_key":  gp.APIKey,
			},
		}
		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

// ReportUsage 报告使用情况
func (gp *GeonixProvider) ReportUsage(proxy *ResidentialProxy, success bool, latency time.Duration) error {
	return nil
}

// GetName 获取提供者名称
func (gp *GeonixProvider) GetName() string {
	return "geonix"
}

// GetCost 获取成本信息
func (gp *GeonixProvider) GetCost() *ProxyCost {
	return &ProxyCost{
		PerRequest: 0.0007, // $0.0007 per request
		PerGB:      0.35,   // $0.35 per GB
		PerHour:    0.07,   // $0.07 per hour
		Currency:   "USD",
	}
}
