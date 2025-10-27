package serverless

import (
	"context"
	"net/http"
)

// AliyunFCEvent 阿里云函数计算事件
type AliyunFCEvent struct {
	HTTPMethod      string            `json:"httpMethod"`
	Path            string            `json:"path"`
	Headers         map[string]string `json:"headers"`
	QueryParameters map[string]string `json:"queryParameters"`
	Body            string            `json:"body"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
}

// AliyunFCResponse 阿里云函数计算响应
type AliyunFCResponse struct {
	StatusCode      int               `json:"statusCode"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
}

// AliyunFCAdapter 阿里云函数计算适配器
type AliyunFCAdapter struct {
	*BaseAdapter
}

// NewAliyunFCAdapter 创建阿里云函数计算适配器
func NewAliyunFCAdapter(handler http.Handler) *AliyunFCAdapter {
	return &AliyunFCAdapter{
		BaseAdapter: NewBaseAdapter(handler),
	}
}

// Wrap 包装HTTP处理器
func (a *AliyunFCAdapter) Wrap(handler http.Handler) func(ctx context.Context, event *AliyunFCEvent) (*AliyunFCResponse, error) {
	return func(ctx context.Context, event *AliyunFCEvent) (*AliyunFCResponse, error) {
		// 转换事件为HTTP请求
		req := a.convertEventToRequest(ctx, event)

		// 创建响应写入器
		w := &ResponseWriter{}

		// 处理请求
		handler.ServeHTTP(w, req)

		// 转换响应
		response := &AliyunFCResponse{
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
func (e *AliyunFCEvent) GetMethod() string {
	return e.HTTPMethod
}

func (e *AliyunFCEvent) GetPath() string {
	return e.Path
}

func (e *AliyunFCEvent) GetHeaders() map[string]string {
	return e.Headers
}

func (e *AliyunFCEvent) GetBody() []byte {
	return []byte(e.Body)
}

func (e *AliyunFCEvent) GetQuery() map[string]string {
	return e.QueryParameters
}

// 阿里云函数计算入口点
func AliyunFCHandler(ctx context.Context, event *AliyunFCEvent) (*AliyunFCResponse, error) {
	// 这里需要在实际部署时注入handler
	// 为了演示，我们创建一个简单的处理器
	adapter := NewAliyunFCAdapter(nil)
	return adapter.Wrap(nil)(ctx, event)
}
