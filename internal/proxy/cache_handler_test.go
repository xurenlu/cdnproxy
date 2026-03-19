package proxy

import (
	"cdnproxy/internal/cache"
	"cdnproxy/internal/config"
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

// MockCache 模拟缓存实现
type MockCache struct {
	data map[string]*cache.Entry
}

func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string]*cache.Entry),
	}
}

func (m *MockCache) Get(ctx context.Context, key string) (*cache.Entry, error) {
	return m.data[key], nil
}

func (m *MockCache) Set(ctx context.Context, key string, entry *cache.Entry, ttl time.Duration) error {
	m.data[key] = entry
	return nil
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func TestBuildCacheKey(t *testing.T) {
	tests := []struct {
		name   string
		method string
		url    string
	}{
		{"GET 请求", "GET", "https://example.com/file.js"},
		{"POST 请求", "POST", "https://example.com/api"},
		{"带查询参数", "GET", "https://example.com/file.js?v=1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := buildCacheKey(tt.method, tt.url)
			if key == "" {
				t.Error("buildCacheKey 返回空字符串")
			}
			if len(key) < 10 { // SHA256 哈希应该更长
				t.Errorf("buildCacheKey 返回的键太短: %s", key)
			}
		})
	}
}

func TestFirstSegmentLooksLikeDomain(t *testing.T) {
	cfg := config.Config{}
	h := &Handler{cfg: cfg}

	tests := []struct {
		path     string
		expected bool
	}{
		{"/cdn.jsdelivr.net/npm/vue", true},
		{"/example.com", true},
		{"/localhost", true},
		{"/sub.example.com/path", true},
		{"/favicon.ico", false},
		{"/robots.txt", false},
		{"/style.css", false},
		{"/app.js", false},
		{"/image.png", false},
		{"/", false},
		{"", false},
		{"/https://example.com/path", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := h.firstSegmentLooksLikeDomain(tt.path)
			if got != tt.expected {
				t.Errorf("firstSegmentLooksLikeDomain(%q) = %v, 期望 %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestBuildUpstreamURL(t *testing.T) {
	cfg := config.Config{}
	h := &Handler{cfg: cfg}

	tests := []struct {
		path       string
		rawQuery   string
		expected   string
		expectErr  bool
	}{
		{"/example.com/path", "", "https://example.com/path", false},
		{"/example.com", "a=1&b=2", "https://example.com?a=1&b=2", false},
		{"/https://example.com/path", "", "https://example.com/path", false},
		{"/http://example.com/path", "", "http://example.com/path", false},
		{"", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://localhost"+tt.path, nil)
			if tt.rawQuery != "" {
				req.URL.RawQuery = tt.rawQuery
			}

			got, err := h.buildUpstreamURL(req)
			if tt.expectErr {
				if err == nil {
					t.Error("期望返回错误，但没有")
				}
				return
			}
			if err != nil {
				t.Errorf("不期望返回错误: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("buildUpstreamURL() = %q, 期望 %q", got, tt.expected)
			}
		})
	}
}

func TestIsHopByHopHeader(t *testing.T) {
	tests := []struct {
		header   string
		expected bool
	}{
		{"Connection", true},
		{"connection", true},
		{"Keep-Alive", true},
		{"Proxy-Authenticate", true},
		{"Content-Type", false},
		{"Cache-Control", false},
		{"Content-Length", false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			got := isHopByHopHeader(tt.header)
			if got != tt.expected {
				t.Errorf("isHopByHopHeader(%q) = %v, 期望 %v", tt.header, got, tt.expected)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s        string
		subs     []string
		expected bool
	}{
		{"Mozilla/5.0 Chrome", []string{"Chrome", "Firefox"}, true},
		{"Mozilla/5.0 Safari", []string{"Chrome", "Firefox"}, false},
		{"", []string{"Chrome"}, false},
		{"Chrome", []string{}, false},
		{"chrome", []string{"Chrome"}, true}, // 大小写不敏感
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := containsAny(tt.s, tt.subs)
			if got != tt.expected {
				t.Errorf("containsAny(%q, %v) = %v, 期望 %v", tt.s, tt.subs, got, tt.expected)
			}
		})
	}
}

func TestCompressBody(t *testing.T) {
	tests := []struct {
		name           string
		body           []byte
		acceptEncoding string
		expectCompress bool
	}{
		{
			name:           "大文件支持gzip",
			body:           make([]byte, 2048), // 2KB
			acceptEncoding: "gzip",
			expectCompress: true,
		},
		{
			name:           "小文件不压缩",
			body:           make([]byte, 512), // 512B
			acceptEncoding: "gzip",
			expectCompress: false,
		},
		{
			name:           "不支持gzip",
			body:           make([]byte, 2048),
			acceptEncoding: "identity",
			expectCompress: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, encoding := compressBody(tt.body, tt.acceptEncoding)
			if tt.expectCompress && encoding != "gzip" {
				t.Errorf("期望使用 gzip 压缩，实际编码: %s", encoding)
			}
			if !tt.expectCompress && encoding != "" {
				t.Errorf("期望不压缩，实际编码: %s", encoding)
			}
			if tt.expectCompress && len(got) >= len(tt.body) {
				t.Error("压缩后应该比原始数据小")
			}
		})
	}
}

func TestIsAPIDomain(t *testing.T) {
	cfg := config.Config{
		APIDomains: []string{
			"api.openai.com",
			"api.anthropic.com",
			"poe.com",
		},
	}
	h := &Handler{cfg: cfg}

	tests := []struct {
		url      string
		expected bool
	}{
		{"https://api.openai.com/v1/chat", true},
		{"http://api.anthropic.com/v1/messages", true},
		{"https://poe.com/api", true},
		{"https://cdn.jsdelivr.net/npm/vue", false},
		{"https://example.com/api", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := h.isAPIDomain(tt.url)
			if got != tt.expected {
				t.Errorf("isAPIDomain(%q) = %v, 期望 %v", tt.url, got, tt.expected)
			}
		})
	}
}

func TestIsWebSocketRequest(t *testing.T) {
	tests := []struct {
		name         string
		upgrade      string
		connection   string
		expected     bool
	}{
		{"WebSocket请求", "websocket", "upgrade", true},
		{"WebSocket大写", "WebSocket", "Upgrade", true},
		{"普通请求", "", "", false},
		{"部分匹配", "websocket", "keep-alive", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			if tt.upgrade != "" {
				req.Header.Set("Upgrade", tt.upgrade)
			}
			if tt.connection != "" {
				req.Header.Set("Connection", tt.connection)
			}

			got := isWebSocketRequest(req)
			if got != tt.expected {
				t.Errorf("isWebSocketRequest() = %v, 期望 %v", got, tt.expected)
			}
		})
	}
}
