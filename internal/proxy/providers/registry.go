// Package providers 住宅IP代理提供者注册表
// 作者: rocky<m@some.im>

package providers

import (
	"os"
)

// ProviderRegistry 提供者注册表
type ProviderRegistry struct {
	providers map[string]ResidentialProxyProvider
}

// NewProviderRegistry 创建提供者注册表
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]ResidentialProxyProvider),
	}
}

// RegisterAllProviders 注册所有可用的提供者
func (pr *ProviderRegistry) RegisterAllProviders() {
	// 注册 Bright Data
	if apiKey := os.Getenv("BRIGHT_DATA_API_KEY"); apiKey != "" {
		username := os.Getenv("BRIGHT_DATA_USERNAME")
		password := os.Getenv("BRIGHT_DATA_PASSWORD")
		if username != "" && password != "" {
			pr.providers["bright_data"] = NewBrightDataProvider(apiKey, username, password)
		}
	}

	// 注册 Smartproxy
	if apiKey := os.Getenv("SMARTPROXY_API_KEY"); apiKey != "" {
		username := os.Getenv("SMARTPROXY_USERNAME")
		password := os.Getenv("SMARTPROXY_PASSWORD")
		if username != "" && password != "" {
			pr.providers["smartproxy"] = NewSmartproxyProvider(apiKey, username, password)
		}
	}

	// 注册 NetNut
	if apiKey := os.Getenv("NETNUT_API_KEY"); apiKey != "" {
		username := os.Getenv("NETNUT_USERNAME")
		password := os.Getenv("NETNUT_PASSWORD")
		if username != "" && password != "" {
			pr.providers["netnut"] = NewNetNutProvider(apiKey, username, password)
		}
	}

	// 注册 Oxylabs
	if apiKey := os.Getenv("OXYLABS_API_KEY"); apiKey != "" {
		username := os.Getenv("OXYLABS_USERNAME")
		password := os.Getenv("OXYLABS_PASSWORD")
		if username != "" && password != "" {
			pr.providers["oxylabs"] = NewOxylabsProvider(apiKey, username, password)
		}
	}

	// 注册 Proxy-Seller
	if apiKey := os.Getenv("PROXY_SELLER_API_KEY"); apiKey != "" {
		username := os.Getenv("PROXY_SELLER_USERNAME")
		password := os.Getenv("PROXY_SELLER_PASSWORD")
		if username != "" && password != "" {
			pr.providers["proxy_seller"] = NewProxySellerProvider(apiKey, username, password)
		}
	}

	// 注册 Youproxy
	if apiKey := os.Getenv("YOUPROXY_API_KEY"); apiKey != "" {
		username := os.Getenv("YOUPROXY_USERNAME")
		password := os.Getenv("YOUPROXY_PASSWORD")
		if username != "" && password != "" {
			pr.providers["youproxy"] = NewYouproxyProvider(apiKey, username, password)
		}
	}

	// 注册 Geonix
	if apiKey := os.Getenv("GEONIX_API_KEY"); apiKey != "" {
		username := os.Getenv("GEONIX_USERNAME")
		password := os.Getenv("GEONIX_PASSWORD")
		if username != "" && password != "" {
			pr.providers["geonix"] = NewGeonixProvider(apiKey, username, password)
		}
	}

	// 注册 IPBurger
	if apiKey := os.Getenv("IPBURGER_API_KEY"); apiKey != "" {
		username := os.Getenv("IPBURGER_USERNAME")
		password := os.Getenv("IPBURGER_PASSWORD")
		if username != "" && password != "" {
			pr.providers["ipburger"] = NewIPBurgerProvider(apiKey, username, password)
		}
	}
}

// GetProviders 获取所有已注册的提供者
func (pr *ProviderRegistry) GetProviders() map[string]ResidentialProxyProvider {
	return pr.providers
}

// GetProvider 获取指定名称的提供者
func (pr *ProviderRegistry) GetProvider(name string) (ResidentialProxyProvider, bool) {
	provider, exists := pr.providers[name]
	return provider, exists
}

// GetProviderNames 获取所有提供者名称
func (pr *ProviderRegistry) GetProviderNames() []string {
	names := make([]string, 0, len(pr.providers))
	for name := range pr.providers {
		names = append(names, name)
	}
	return names
}

// GetProviderCount 获取提供者数量
func (pr *ProviderRegistry) GetProviderCount() int {
	return len(pr.providers)
}

// IsProviderAvailable 检查提供者是否可用
func (pr *ProviderRegistry) IsProviderAvailable(name string) bool {
	_, exists := pr.providers[name]
	return exists
}
