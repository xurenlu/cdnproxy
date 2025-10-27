package serverless

import (
	"context"
	"net/http"
)

// TencentCloudEvent 腾讯云函数事件
type TencentCloudEvent struct {
	HTTPMethod      string            `json:"httpMethod"`
	Path            string            `json:"path"`
	Headers         map[string]string `json:"headers"`
	QueryString     map[string]string `json:"queryString"`
	Body            string            `json:"body"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
}

// TencentCloudResponse 腾讯云函数响应
type TencentCloudResponse struct {
	StatusCode      int               `json:"statusCode"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
}

// TencentCloudAdapter 腾讯云函数适配器
type TencentCloudAdapter struct {
	*BaseAdapter
}

// NewTencentCloudAdapter 创建腾讯云函数适配器
func NewTencentCloudAdapter(handler http.Handler) *TencentCloudAdapter {
	return &TencentCloudAdapter{
		BaseAdapter: NewBaseAdapter(handler),
	}
}

// Wrap 包装HTTP处理器
func (a *TencentCloudAdapter) Wrap(handler http.Handler) func(ctx context.Context, event *TencentCloudEvent) (*TencentCloudResponse, error) {
	return func(ctx context.Context, event *TencentCloudEvent) (*TencentCloudResponse, error) {
		// 转换事件为HTTP请求
		req := a.convertEventToRequest(ctx, event)
		
		// 创建响应写入器
		w := &ResponseWriter{}
		
		// 处理请求
		handler.ServeHTTP(w, req)
		
		// 转换响应
		response := &TencentCloudResponse{
			StatusCode:      w.statusCode,
			Headers:         w.headers,
			Body:            string(w.body),
			IsBase64Encoded: false,
		}
		
		// 设置默认状态码
		if response.StatusCode == 0 {
			response.StatusCode = 200
		}
		
		// 设置默认响应头
		if response.Headers == nil {
			response.Headers = make(map[string]string)
		}
		response.Headers["Content-Type"] = "application/json"
		
		return response, nil
	}
}

// 实现Event接口
func (e *TencentCloudEvent) GetMethod() string {
	return e.HTTPMethod
}

func (e *TencentCloudEvent) GetPath() string {
	return e.Path
}

func (e *TencentCloudEvent) GetHeaders() map[string]string {
	return e.Headers
}

func (e *TencentCloudEvent) GetBody() []byte {
	return []byte(e.Body)
}

func (e *TencentCloudEvent) GetQuery() map[string]string {
	return e.QueryString
}

// 腾讯云函数入口点
func TencentCloudHandler(ctx context.Context, event *TencentCloudEvent) (*TencentCloudResponse, error) {
	// 这里需要在实际部署时注入handler
	// 为了演示，我们创建一个简单的处理器
	adapter := NewTencentCloudAdapter(nil)
	return adapter.Wrap(nil)(ctx, event)
}
