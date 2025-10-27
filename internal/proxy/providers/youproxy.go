// Package providers Youproxy 住宅IP代理提供者
// 作者: rocky<m@some.im>

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// YouproxyProvider Youproxy 住宅IP代理提供者
type YouproxyProvider struct {
	APIKey     string
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// YouproxyResponse Youproxy API响应
type YouproxyResponse struct {
	Status string `json:"status"`
	Data   struct {
		Proxies []YouproxyProxy `json:"proxies"`
	} `json:"data"`
}

// YouproxyProxy Youproxy 代理信息
type YouproxyProxy struct {
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

// NewYouproxyProvider 创建 Youproxy 提供者
func NewYouproxyProvider(apiKey, username, password string) *YouproxyProvider {
	return &YouproxyProvider{
		APIKey:   apiKey,
		BaseURL:  "https://api.youproxy.com",
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetProxy 获取一个可用的代理
func (yp *YouproxyProvider) GetProxy(ctx context.Context) (*ResidentialProxy, error) {
	proxies, err := yp.GetProxyList(ctx)
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
func (yp *YouproxyProvider) GetProxyList(ctx context.Context) ([]*ResidentialProxy, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", yp.BaseURL+"/proxy/list", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+yp.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := yp.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var apiResp YouproxyResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	proxies := make([]*ResidentialProxy, 0, len(apiResp.Data.Proxies))
	for _, ypProxy := range apiResp.Data.Proxies {
		proxy := &ResidentialProxy{
			ID:          ypProxy.ID,
			IP:          ypProxy.IP,
			Port:        ypProxy.Port,
			Username:    ypProxy.Username,
			Password:    ypProxy.Password,
			Location:    ypProxy.Location,
			Country:     ypProxy.Country,
			City:        ypProxy.City,
			ISP:         ypProxy.ISP,
			Type:        ypProxy.Type,
			Quality:     ypProxy.Quality,
			SuccessRate: ypProxy.SuccessRate,
			LastUsed:    time.Now(),
			Metadata: map[string]string{
				"provider": "youproxy",
				"api_key":  yp.APIKey,
			},
		}
		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

// ReportUsage 报告使用情况
func (yp *YouproxyProvider) ReportUsage(proxy *ResidentialProxy, success bool, latency time.Duration) error {
	// Youproxy 通常不需要手动报告使用情况
	return nil
}

// GetName 获取提供者名称
func (yp *YouproxyProvider) GetName() string {
	return "youproxy"
}

// GetCost 获取成本信息
func (yp *YouproxyProvider) GetCost() *ProxyCost {
	return &ProxyCost{
		PerRequest: 0.0006, // $0.0006 per request
		PerGB:      0.25,   // $0.25 per GB
		PerHour:    0.06,   // $0.06 per hour
		Currency:   "USD",
	}
}
