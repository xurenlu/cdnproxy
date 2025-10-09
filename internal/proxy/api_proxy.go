package proxy

import (
	"bufio"
	"crypto/tls"
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

	// 使用专门的 HTTP 客户端（无超时限制，支持长连接）
	client := &http.Client{
		Transport: h.httpClient.Transport,
		Timeout:   0, // 无超时限制
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不自动跟随重定向
		},
	}

	resp, err := client.Do(req)
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
		h.streamSSE(w, resp.Body)
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
	upstreamConn, err := h.dialUpstream(wsURL, upstreamReq)
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

	// 双向复制数据
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

	// 等待任一方向完成或出错
	err = <-errChan
	if err != nil && err != io.EOF {
		log.Printf("WebSocket proxy error: %v", err)
	}
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
func (h *Handler) streamSSE(w http.ResponseWriter, body io.Reader) {
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
