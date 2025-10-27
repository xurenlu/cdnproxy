package serverless

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Event 云函数事件接口
type Event interface {
	GetMethod() string
	GetPath() string
	GetHeaders() map[string]string
	GetBody() []byte
	GetQuery() map[string]string
}

// Response 云函数响应接口
type Response interface {
	SetStatusCode(code int)
	SetHeaders(headers map[string]string)
	SetBody(body []byte)
	GetStatusCode() int
	GetHeaders() map[string]string
	GetBody() []byte
}

// Adapter 云函数适配器接口
type Adapter interface {
	Wrap(handler http.Handler) func(ctx context.Context, event Event) (Response, error)
}

// BaseAdapter 基础适配器
type BaseAdapter struct {
	handler http.Handler
}

// NewBaseAdapter 创建基础适配器
func NewBaseAdapter(handler http.Handler) *BaseAdapter {
	return &BaseAdapter{handler: handler}
}

// convertEventToRequest 将云函数事件转换为HTTP请求
func (a *BaseAdapter) convertEventToRequest(ctx context.Context, event Event) *http.Request {
	// 构建URL
	u := &url.URL{
		Path:     event.GetPath(),
		RawQuery: a.buildQueryString(event.GetQuery()),
	}

	// 创建HTTP请求
	req, _ := http.NewRequestWithContext(ctx, event.GetMethod(), u.String(), strings.NewReader(string(event.GetBody())))

	// 设置请求头
	for key, value := range event.GetHeaders() {
		req.Header.Set(key, value)
	}

	return req
}

// convertResponseToEvent 将HTTP响应转换为云函数事件
func (a *BaseAdapter) convertResponseToEvent(w *ResponseWriter) Response {
	return &BaseResponse{
		StatusCode: w.statusCode,
		Headers:    w.headers,
		Body:       w.body,
	}
}

// buildQueryString 构建查询字符串
func (a *BaseAdapter) buildQueryString(query map[string]string) string {
	if len(query) == 0 {
		return ""
	}

	var parts []string
	for key, value := range query {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, "&")
}

// ResponseWriter 响应写入器
type ResponseWriter struct {
	statusCode int
	headers    map[string]string
	body       []byte
}

func (w *ResponseWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(map[string]string)
	}
	// 返回一个临时的Header，用于设置响应头
	return make(http.Header)
}

func (w *ResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

func (w *ResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// BaseResponse 基础响应实现
type BaseResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
}

func (r *BaseResponse) SetStatusCode(code int) {
	r.StatusCode = code
}

func (r *BaseResponse) SetHeaders(headers map[string]string) {
	r.Headers = headers
}

func (r *BaseResponse) SetBody(body []byte) {
	r.Body = body
}

func (r *BaseResponse) GetStatusCode() int {
	return r.StatusCode
}

func (r *BaseResponse) GetHeaders() map[string]string {
	return r.Headers
}

func (r *BaseResponse) GetBody() []byte {
	return r.Body
}
