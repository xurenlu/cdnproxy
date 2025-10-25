package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// proxyAPIRequest 处理 API 请求（不缓存，支持 WebSocket/SSE/长连接）
func (h *Handler) proxyAPIRequest(w http.ResponseWriter, r *http.Request, upstreamURL string) {
	// 检查是否是 WebSocket 升级请求
	if isWebSocketRequest(r) {
		h.proxyWebSocket(w, r, upstreamURL)
		return
	}

	// 创建上游请求
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusInternalServerError)
		return
	}

	// 复制所有请求头（保持 API 请求的完整性）
	copyAllHeaders(req.Header, r.Header)

	// 确保有合适的 User-Agent
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "cdnproxy/2.0")
	}

	// 使用专门的 API HTTP 客户端（长超时，支持长连接）
	resp, err := h.apiClient.Do(req)
	if err != nil {
		log.Printf("API proxy error: %v", err)
		http.Error(w, "upstream API error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 检查是否是 SSE 响应
	contentType := resp.Header.Get("Content-Type")
	isSSE := strings.Contains(contentType, "text/event-stream")

	// 复制所有响应头
	copyAllHeaders(w.Header(), resp.Header)

	// 写入状态码
	w.WriteHeader(resp.StatusCode)

	// 如果是 SSE，使用流式传输
	if isSSE {
		h.streamSSE(w, resp.Body, r.Context())
		return
	}

	// 普通响应，直接复制
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("API proxy response copy error: %v", err)
	}
}

// proxyWebSocket 处理 WebSocket 连接
func (h *Handler) proxyWebSocket(w http.ResponseWriter, r *http.Request, upstreamURL string) {
	// 获取WebSocket信号量
	select {
	case h.wsSemaphore <- struct{}{}:
		defer func() { <-h.wsSemaphore }()
	case <-r.Context().Done():
		http.Error(w, "too many websocket connections", http.StatusServiceUnavailable)
		return
	}

	// 添加超时控制
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	// 将 http:// 或 https:// 替换为 ws:// 或 wss://
	wsURL := strings.Replace(upstreamURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)

	// 创建上游 WebSocket 请求
	upstreamReq, err := http.NewRequest(r.Method, wsURL, nil)
	if err != nil {
		http.Error(w, "failed to create websocket request", http.StatusInternalServerError)
		return
	}

	// 复制关键的 WebSocket 头
	copyWebSocketHeaders(upstreamReq.Header, r.Header)

	// 建立到上游的连接
	upstreamConn, err := h.dialUpstreamWithTimeout(ctx, wsURL, upstreamReq)
	if err != nil {
		log.Printf("WebSocket upstream dial error: %v", err)
		http.Error(w, "failed to connect to upstream", http.StatusBadGateway)
		return
	}
	defer upstreamConn.Close()

	// Hijack 客户端连接
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		log.Printf("WebSocket hijack error: %v", err)
		return
	}
	defer clientConn.Close()

	// 双向复制数据，带超时控制
	errChan := make(chan error, 2)

	// 客户端 -> 上游
	go func() {
		_, err := io.Copy(upstreamConn, clientBuf)
		errChan <- err
	}()

	// 上游 -> 客户端
	go func() {
		_, err := io.Copy(clientConn, upstreamConn)
		errChan <- err
	}()

	// 等待任一方向完成、出错或超时
	select {
	case err = <-errChan:
		if err != nil && err != io.EOF {
			log.Printf("WebSocket proxy error: %v", err)
		}
	case <-ctx.Done():
		log.Printf("WebSocket connection timeout")
	}
}

// dialUpstreamWithTimeout 建立到上游服务器的 TCP 连接并发送 HTTP 请求（带超时）
func (h *Handler) dialUpstreamWithTimeout(ctx context.Context, wsURL string, req *http.Request) (net.Conn, error) {
	// 解析 URL
	var host string
	var useTLS bool
	if strings.HasPrefix(wsURL, "wss://") {
		host = strings.TrimPrefix(wsURL, "wss://")
		useTLS = true
	} else {
		host = strings.TrimPrefix(wsURL, "ws://")
		useTLS = false
	}

	// 提取 host 和 path
	pathStart := strings.Index(host, "/")
	var path string
	if pathStart > 0 {
		path = host[pathStart:]
		host = host[:pathStart]
	} else {
		path = "/"
	}

	// 如果 host 没有端口，添加默认端口
	if !strings.Contains(host, ":") {
		if useTLS {
			host = host + ":443"
		} else {
			host = host + ":80"
		}
	}

	// 建立 TCP 连接（带超时）
	dialer := &net.Dialer{
		Timeout:   10 * time.Second, // 缩短连接超时
		KeepAlive: 30 * time.Second, // 缩短保活时间
	}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}

	// 如果是 wss，需要 TLS 包装
	if useTLS {
		tlsConn := wrapTLS(conn, strings.Split(host, ":")[0])
		if tlsConn == nil { // 添加nil检查
			return nil, fmt.Errorf("TLS handshake failed for host: %s", host)
		}
		conn = tlsConn
	}

	// 构建并发送 WebSocket 升级请求
	upgradeReq := "GET " + path + " HTTP/1.1\r\n"
	upgradeReq += "Host: " + strings.Split(host, ":")[0] + "\r\n"
	for k, vals := range req.Header {
		for _, v := range vals {
			upgradeReq += k + ": " + v + "\r\n"
		}
	}
	upgradeReq += "\r\n"

	_, err = conn.Write([]byte(upgradeReq))
	if err != nil {
		conn.Close()
		return nil, err
	}

	// 读取响应头，确认升级成功
	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}

	// 检查状态码是否为 101
	if !strings.Contains(statusLine, "101") {
		conn.Close()
		return nil, io.ErrUnexpectedEOF
	}

	// 跳过响应头
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	return conn, nil
}

// dialUpstream 建立到上游服务器的 TCP 连接并发送 HTTP 请求
func (h *Handler) dialUpstream(wsURL string, req *http.Request) (net.Conn, error) {
	// 解析 URL
	var host string
	var useTLS bool
	if strings.HasPrefix(wsURL, "wss://") {
		host = strings.TrimPrefix(wsURL, "wss://")
		useTLS = true
	} else {
		host = strings.TrimPrefix(wsURL, "ws://")
		useTLS = false
	}

	// 提取 host 和 path
	pathStart := strings.Index(host, "/")
	var path string
	if pathStart > 0 {
		path = host[pathStart:]
		host = host[:pathStart]
	} else {
		path = "/"
	}

	// 如果 host 没有端口，添加默认端口
	if !strings.Contains(host, ":") {
		if useTLS {
			host = host + ":443"
		} else {
			host = host + ":80"
		}
	}

	// 建立 TCP 连接
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 60 * time.Second,
	}
	conn, err := dialer.Dial("tcp", host)
	if err != nil {
		return nil, err
	}

	// 如果是 wss，需要 TLS 包装
	if useTLS {
		tlsConn := wrapTLS(conn, strings.Split(host, ":")[0])
		conn = tlsConn
	}

	// 构建并发送 WebSocket 升级请求
	upgradeReq := "GET " + path + " HTTP/1.1\r\n"
	upgradeReq += "Host: " + strings.Split(host, ":")[0] + "\r\n"
	for k, vals := range req.Header {
		for _, v := range vals {
			upgradeReq += k + ": " + v + "\r\n"
		}
	}
	upgradeReq += "\r\n"

	_, err = conn.Write([]byte(upgradeReq))
	if err != nil {
		conn.Close()
		return nil, err
	}

	// 读取响应头，确认升级成功
	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}

	// 检查状态码是否为 101
	if !strings.Contains(statusLine, "101") {
		conn.Close()
		return nil, io.ErrUnexpectedEOF
	}

	// 跳过响应头
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	return conn, nil
}

// streamSSE 流式传输 SSE 响应
func (h *Handler) streamSSE(w http.ResponseWriter, body io.Reader, reqCtx context.Context) {
	// 使用请求的context，并添加超时控制
	ctx, cancel := context.WithTimeout(reqCtx, 10*time.Minute)
	defer cancel()

	// 确保响应可以被 flush
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Println("streaming not supported")
		io.Copy(w, body)
		return
	}

	// 使用 bufio.Scanner 逐行读取
	scanner := bufio.NewScanner(body)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			log.Printf("SSE stream timeout")
			return
		default:
			line := scanner.Bytes()
			// 写入行
			_, err := w.Write(append(line, '\n'))
			if err != nil {
				log.Printf("SSE write error: %v", err)
				return
			}
			// 立即刷新，确保数据发送到客户端
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		log.Printf("SSE scan error: %v", err)
	}
}

// isWebSocketRequest 检查是否是 WebSocket 升级请求
func isWebSocketRequest(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Upgrade")) == "websocket" &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// copyAllHeaders 复制所有请求头（包括 hop-by-hop 头，API 代理需要完整转发）
func copyAllHeaders(dst, src http.Header) {
	for k, vals := range src {
		// 对于 API 代理，我们需要转发几乎所有头，但排除一些特殊的
		lowerK := strings.ToLower(k)
		// 跳过代理相关的内部头
		if lowerK == "x-forwarded-for" || lowerK == "x-real-ip" {
			continue
		}
		for _, v := range vals {
			dst.Add(k, v)
		}
	}
}

// copyWebSocketHeaders 复制 WebSocket 相关的请求头
func copyWebSocketHeaders(dst, src http.Header) {
	// 必需的 WebSocket 头
	wsHeaders := []string{
		"Upgrade",
		"Connection",
		"Sec-WebSocket-Key",
		"Sec-WebSocket-Version",
		"Sec-WebSocket-Protocol",
		"Sec-WebSocket-Extensions",
		"Origin",
		"User-Agent",
		"Cookie",
		"Authorization",
	}

	for _, header := range wsHeaders {
		if val := src.Get(header); val != "" {
			dst.Set(header, val)
		}
	}
}

// wrapTLS 包装 TLS 连接
func wrapTLS(conn net.Conn, serverName string) net.Conn {
	tlsConfig := &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: false, // 生产环境应该验证证书
		MinVersion:         tls.VersionTLS12,
	}
	tlsConn := tls.Client(conn, tlsConfig)

	// 执行握手
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("TLS handshake error: %v", err)
		conn.Close()
		return nil
	}

	return tlsConn
}
