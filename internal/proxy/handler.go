package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"cdnproxy/internal/cache"
	"cdnproxy/internal/config"
	"cdnproxy/internal/metrics"
)

var (
	commonBrowserUAs = []string{"Chrome", "Chromium", "Firefox", "Safari", "Edg", "OPR"}
	ipRefererPattern = regexp.MustCompile(`^https?://(\d{1,3}\.){3}\d{1,3}(:\d+)?/`)
	localhostReferer = regexp.MustCompile(`^https?://(localhost|127.0.0.1|0.0.0.0)(:\d+)?/`)
)

type Handler struct {
	cfg              config.Config
	cache            *cache.DiskCache
	whitelistStore   WhitelistStore
	httpClient       *http.Client
	apiClient        *http.Client // API专用客户端（长超时）
	configStore      ConfigStore
	counterStore     CounterStore
	semaphore        chan struct{}            // 添加信号量
	bufferPool       sync.Pool                // 内存池
	wsSemaphore      chan struct{}            // WebSocket并发限制
	webpConverter    *WebPConverter           // WebP转换器
	residentialProxy *ResidentialProxyManager // 住宅IP代理管理器
	aiAPIProxy       *AIAPIProxy              // AI API代理处理器
}

// WhitelistStore 接口定义
type WhitelistStore interface {
	ContainsAllowedSuffix(ctx context.Context, host string) (bool, error)
}

// ConfigStore 接口定义
type ConfigStore interface {
	GetReferrerThreshold(ctx context.Context) (int64, error)
}

// CounterStore 接口定义
type CounterStore interface {
	IncrementReferrerCount(ctx context.Context, host string) (int64, error)
}

func NewHandler(cfg config.Config, diskCache *cache.DiskCache, whitelistStore WhitelistStore, configStore ConfigStore, counterStore CounterStore) http.Handler {
	// 优化 TCP 参数，适配跨境大文件传输
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 60 * time.Second,
		// 启用 TCP Fast Open（Linux 需要内核支持）
	}

	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          50,               // 降低到合理值
		MaxIdleConnsPerHost:   25,               // 降低到合理值
		MaxConnsPerHost:       100,              // 添加限制！
		IdleConnTimeout:       30 * time.Second, // 缩短空闲时间
		TLSHandshakeTimeout:   5 * time.Second,  // 缩短
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second, // 缩短
		// 启用 HTTP/2，对多路复用和头部压缩有帮助
		DisableCompression: true, // 禁用自动压缩，避免上游返回 gzip 导致问题，我们自己处理压缩
		// 降低缓冲区，减少内存使用
		WriteBufferSize: 32 * 1024, // 32KB 写缓冲
		ReadBufferSize:  32 * 1024, // 32KB 读缓冲
	}
	// 初始化住宅IP代理管理器
	residentialProxy := NewResidentialProxyManager()

	// 初始化AI API代理处理器
	aiAPIProxy := NewAIAPIProxy(residentialProxy)

	// 初始化WebP转换器
	webpConverter := NewWebPConverter()

	return &Handler{
		cfg:            cfg,
		cache:          diskCache,
		whitelistStore: whitelistStore,
		configStore:    configStore,
		counterStore:   counterStore,
		httpClient:     &http.Client{Transport: tr, Timeout: 30 * time.Second}, // CDN请求客户端
		apiClient: &http.Client{ // API请求专用客户端（长超时）
			Transport: tr,
			Timeout:   5 * time.Minute,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // 不自动跟随重定向
			},
		},
		semaphore:   make(chan struct{}, 50), // 最多50个并发
		wsSemaphore: make(chan struct{}, 10), // 最多10个WebSocket连接
		bufferPool: sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 64*1024) // 64KB 缓冲区
				return &buf
			},
		},
		webpConverter:    webpConverter,    // WebP转换器
		residentialProxy: residentialProxy, // 住宅IP代理管理器
		aiAPIProxy:       aiAPIProxy,       // AI API代理处理器
	}
}

// getBuffer 从内存池获取缓冲区
func (h *Handler) getBuffer() []byte {
	return *h.bufferPool.Get().(*[]byte)
}

// putBuffer 将缓冲区归还到内存池
func (h *Handler) putBuffer(buf []byte) {
	h.bufferPool.Put(&buf)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	success := false
	defer func() {
		duration := time.Since(start)
		metrics.GetGlobalMetrics().RecordRequest(success, duration)
		if duration > 5*time.Second {
			log.Printf("SLOW REQUEST: %s %s %s", r.Method, r.URL.Path, duration)
		}
	}()

	// 获取信号量
	select {
	case h.semaphore <- struct{}{}:
		defer func() { <-h.semaphore }()
	case <-r.Context().Done():
		http.Error(w, "service busy", http.StatusServiceUnavailable)
		return
	}

	// Incoming path is like: /cdn.jsdelivr.net/npm/bootstrap@... -> we must reconstruct the upstream URL
	upstreamURL, err := h.buildUpstreamURL(r)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// 检查是否是 API 域名（这些请求不缓存，支持 WebSocket/SSE/长连接）
	if h.isAPIDomain(upstreamURL) {
		// API 代理不需要访问控制检查（由上游 API 服务自己控制）
		// 使用住宅IP代理处理AI API请求
		if err := h.aiAPIProxy.ProxyAIRequest(w, r, upstreamURL); err != nil {
			// 如果住宅IP代理失败，回退到普通API代理
			h.proxyAPIRequest(w, r, upstreamURL)
		}
		success = true
		return
	}

	// Access control (仅对 CDN 代理进行访问控制)
	allowed, reason := h.isAccessAllowed(r)
	if !allowed {
		w.Header().Set("X-Blocked-Reason", reason)
		http.Error(w, "forbidden: "+reason, http.StatusForbidden)
		return
	}

	// Only cache GET/HEAD
	method := strings.ToUpper(r.Method)
	if method != http.MethodGet && method != http.MethodHead {
		h.proxyNoCache(w, r, upstreamURL)
		return
	}

	// 为缓存生成键
	key := buildCacheKey(method, upstreamURL)
	if e, _ := h.cache.Get(r.Context(), key); e != nil {
		metrics.GetGlobalMetrics().RecordCacheHit()
		// 设置优化的Cache-Control头
		contentType := e.ContentType
		if contentType == "" {
			contentType = e.Headers["Content-Type"]
		}
		cacheControl := cache.GetCacheControlByContentType(contentType)
		w.Header().Set("Cache-Control", cacheControl)

		// 复制其他响应头（排除需要动态设置的头）
		for k, v := range e.Headers {
			lowerK := strings.ToLower(k)
			// 避免覆盖我们需要动态设置的头
			if k != "Cache-Control" && lowerK != "content-encoding" && lowerK != "content-length" {
				w.Header().Set(k, v)
			}
		}

		// 根据客户端需求动态压缩缓存的数据
		body := e.Body
		if method == http.MethodGet && len(body) > 0 {
			// 检查是否是视频/音频文件，这些文件通常已经压缩，不需要再次压缩
			isVideoOrAudio := strings.Contains(strings.ToLower(contentType), "video/") ||
				strings.Contains(strings.ToLower(contentType), "audio/")

			if !isVideoOrAudio {
				acceptEncoding := r.Header.Get("Accept-Encoding")
				compressedBody, encoding := h.compressBody(body, acceptEncoding)
				if encoding != "" {
					w.Header().Set("Content-Encoding", encoding)
					w.Header().Set("Vary", "Accept-Encoding")
					body = compressedBody
				}
			}
		}

		w.WriteHeader(e.StatusCode)
		if method == http.MethodGet && len(body) > 0 {
			_, _ = w.Write(body)
		}
		success = true
		return
	}

	// 记录缓存未命中
	metrics.GetGlobalMetrics().RecordCacheMiss()

	// Fetch upstream
	req, err := http.NewRequestWithContext(r.Context(), method, upstreamURL, nil)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	// 优先使用客户端的 User-Agent，让 CDN 能正确识别
	clientUA := r.Header.Get("User-Agent")
	if clientUA != "" {
		req.Header.Set("User-Agent", clientUA)
	} else {
		// 模拟主流浏览器 UA，避免被限速
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	}

	// 转发客户端的 Range 请求到上游服务器
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	// 主动请求上游返回 gzip 压缩数据，加快下载速度
	// 因为 DisableCompression: true，Go 不会自动处理，我们手动解压（第 275-286 行）
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy headers
	headers := map[string]string{}
	contentType := ""
	contentLength := int64(0)
	contentEncoding := ""

	for k, vals := range resp.Header {
		// Filter hop-by-hop headers
		if isHopByHopHeader(k) {
			continue
		}
		lowerK := strings.ToLower(k)

		// 保存 Content-Length 用于判断文件大小
		if lowerK == "content-length" {
			if len(vals) > 0 {
				fmt.Sscanf(vals[0], "%d", &contentLength)
			}
		}

		// 保存 Content-Encoding，稍后需要手动解压
		if lowerK == "content-encoding" && len(vals) > 0 {
			contentEncoding = vals[0]
		}

		// 过滤压缩相关的头，因为我们会自己处理压缩
		// Content-Encoding: 我们需要手动解压上游的 gzip 数据
		// Content-Length: 压缩后长度会变化，由Go自动设置
		if lowerK == "content-encoding" || lowerK == "content-length" {
			continue
		}
		if len(vals) > 0 {
			w.Header()[k] = vals
			headers[k] = vals[0]
			if k == "Content-Type" {
				contentType = vals[0]
			}
		}
	}

	// 设置优化的Cache-Control头
	cacheControl := cache.GetCacheControlByContentType(contentType)
	w.Header().Set("Cache-Control", cacheControl)

	// 定义大文件阈值（1MB），大于此值的文件使用流式传输
	// 但视频和音频文件即使很大也要缓存，因为CDN缓存对媒体文件很重要
	const largeFileThreshold = 1 * 1024 * 1024
	const videoFileThreshold = 100 * 1024 * 1024 // 视频文件阈值：100MB

	var body []byte
	if method == http.MethodGet {
		// 检查是否是视频/音频文件
		isVideoOrAudio := strings.Contains(strings.ToLower(contentType), "video/") ||
			strings.Contains(strings.ToLower(contentType), "audio/")

		// 大文件直接流式传输，不缓存（但视频/音频文件例外）
		if contentLength > largeFileThreshold && !isVideoOrAudio {
			// 支持 Range 请求
			w.Header().Set("Accept-Ranges", "bytes")

			// 复制所有响应头（包括 Content-Range, Content-Length 等）
			for k, vals := range resp.Header {
				lowerK := strings.ToLower(k)
				// 对于大文件流式传输，保留 Content-Length 和 Content-Range
				if !isHopByHopHeader(k) && lowerK != "content-encoding" {
					w.Header()[k] = vals
				}
			}

			// 如果上游返回 206 Partial Content，我们也返回 206
			// 否则返回原始状态码
			w.WriteHeader(resp.StatusCode)

			// 直接流式传输，边下载边输出
			_, _ = io.Copy(w, resp.Body)
			return
		}

		// 小文件或视频/音频文件读入内存进行处理和缓存
		// 限制读取大小，防止OOM
		var readLimit int64 = largeFileThreshold
		if isVideoOrAudio {
			readLimit = videoFileThreshold // 视频文件允许更大的缓存
		}
		limitedReader := io.LimitReader(resp.Body, readLimit)
		var errCopy error
		body, errCopy = io.ReadAll(limitedReader)
		if errCopy != nil {
			// stream directly if readAll fails
			resp.Body.Close()
			req2, _ := http.NewRequestWithContext(r.Context(), method, upstreamURL, nil)
			req2.Header.Set("User-Agent", "cdnproxy/1.0")
			resp2, err2 := h.httpClient.Do(req2)
			if err2 != nil {
				http.Error(w, "upstream read error", http.StatusBadGateway)
				return
			}
			defer resp2.Body.Close()
			_, _ = io.Copy(w, resp2.Body)
			return
		}

		// 如果上游返回了 gzip 压缩的数据，手动解压
		// (因为我们设置了 DisableCompression: true，不会自动解压)
		if contentEncoding == "gzip" {
			gzReader, err := gzip.NewReader(bytes.NewReader(body))
			if err == nil {
				defer gzReader.Close() // 确保关闭
				decompressed, err := io.ReadAll(gzReader)
				if err == nil {
					body = decompressed
				}
			}
		}

		// 检查是否需要图片格式转换（只对小文件）
		userAgent := r.Header.Get("User-Agent")
		acceptHeader := r.Header.Get("Accept")
		if h.webpConverter.ShouldConvertToWebP(contentType, userAgent, acceptHeader) {
			// WebP转换器内部已经使用信号量限制并发数
			webpBody, err := h.webpConverter.ConvertToWebP(body, contentType)
			if err == nil && len(webpBody) > 0 {
				body = webpBody
				contentType = "image/webp"
				w.Header().Set("Content-Type", "image/webp")
			}
		}

		// 先保存未压缩的原始数据到缓存
		// DiskCache 使用文件过期时间，TTL 在文件系统级别处理
		_ = h.cache.Set(r.Context(), key, &cache.Entry{
			StatusCode:  resp.StatusCode,
			Headers:     headers,
			Body:        body, // 存储未压缩的原始数据
			StoredAt:    time.Now(),
			ContentType: contentType,
		}, h.cfg.CacheTTL)

		// 再根据客户端需求压缩响应
		acceptEncoding := r.Header.Get("Accept-Encoding")
		compressedBody, encoding := h.compressBody(body, acceptEncoding)
		if encoding != "" {
			w.Header().Set("Content-Encoding", encoding)
			w.Header().Set("Vary", "Accept-Encoding")
			body = compressedBody
		}

		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
	} else {
		w.WriteHeader(resp.StatusCode)
	}
}

func (h *Handler) proxyNoCache(w http.ResponseWriter, r *http.Request, upstreamURL string) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
	if err != nil {
		http.Error(w, "request error", http.StatusInternalServerError)
		return
	}
	copyHeaders(req.Header, r.Header)

	// 使用客户端的 User-Agent 或模拟浏览器
	clientUA := r.Header.Get("User-Agent")
	if clientUA != "" {
		req.Header.Set("User-Agent", clientUA)
	} else {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (h *Handler) buildUpstreamURL(r *http.Request) (string, error) {
	// Expect URL path to start with /<host>/...
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		return "", errors.New("empty path")
	}
	// Support schemes in first segment, e.g. /https://example.com/... or default to https
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if r.URL.RawQuery != "" {
			return path + "?" + r.URL.RawQuery, nil
		}
		return path, nil
	}
	// Default https
	// First segment should be host
	slash := strings.IndexByte(path, '/')
	if slash == -1 {
		// no further path
		if r.URL.RawQuery != "" {
			return "https://" + path + "?" + r.URL.RawQuery, nil
		}
		return "https://" + path, nil
	}
	host := path[:slash]
	rest := path[slash:]
	// Validate host
	if _, err := url.Parse("https://" + host); err != nil {
		return "", err
	}
	if r.URL.RawQuery != "" {
		return "https://" + host + rest + "?" + r.URL.RawQuery, nil
	}
	return "https://" + host + rest, nil
}

func (h *Handler) isAccessAllowed(r *http.Request) (bool, string) {
	ref := r.Referer()

	// No referer -> allow
	if ref == "" {
		return true, "no referer"
	}

	// If referer is IP or localhost -> allow
	if ipRefererPattern.MatchString(ref) || localhostReferer.MatchString(ref) {
		return true, "ip/localhost referer"
	}

	u, err := url.Parse(ref)
	if err != nil || u.Hostname() == "" {
		// Malformed or empty host in referer -> allow
		return true, "invalid referer host"
	}
	host := u.Hostname()
	// If hostname parses as an IP -> allow
	if net.ParseIP(host) != nil {
		return true, "ip referer host"
	}
	if strings.EqualFold(host, "localhost") {
		return true, "localhost referer host"
	}

	// Domain referer: check threshold, then whitelist
	var threshold int64 = 1000
	if h.configStore != nil {
		if tv, err2 := h.configStore.GetReferrerThreshold(r.Context()); err2 == nil {
			threshold = tv
		}
	}
	var n int64 = -1
	if h.counterStore != nil {
		if v, err2 := h.counterStore.IncrementReferrerCount(r.Context(), host); err2 == nil {
			n = v
		}
	}
	if n >= 0 && n <= threshold {
		return true, fmt.Sprintf("under threshold (%d <= %d) for %s", n, threshold, host)
	}

	// Over threshold: require whitelist
	if allowed, err := h.whitelistStore.ContainsAllowedSuffix(r.Context(), host); err == nil && allowed {
		return true, "whitelist suffix"
	}

	return false, fmt.Sprintf("ref domain over threshold and not whitelisted: host=%s count=%d threshold=%d", host, n, threshold)
}

func isHopByHopHeader(k string) bool {
	switch strings.ToLower(k) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	}
	return false
}

func copyHeaders(dst, src http.Header) {
	for k, vals := range src {
		if isHopByHopHeader(k) {
			continue
		}
		dst[k] = vals
	}
}

func containsAny(s string, subs []string) bool {
	s = strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(s, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// isAPIDomain 检查 URL 是否属于 API 域名
func (h *Handler) isAPIDomain(upstreamURL string) bool {
	urlLower := strings.ToLower(upstreamURL)

	// 检查是否匹配任何配置的 API 域名
	for _, domain := range h.cfg.APIDomains {
		domainLower := strings.ToLower(domain)
		// 检查 https://domain 或 http://domain
		if strings.Contains(urlLower, "://"+domainLower+"/") ||
			strings.Contains(urlLower, "://"+domainLower+"?") ||
			strings.HasSuffix(urlLower, "://"+domainLower) {
			return true
		}
	}

	return false
}

// compressBody 根据Accept-Encoding头压缩响应体
func (h *Handler) compressBody(body []byte, acceptEncoding string) ([]byte, string) {
	// 如果响应体太小，不进行压缩
	if len(body) < 1024 {
		return body, ""
	}

	// 检查是否已经压缩过
	if strings.Contains(acceptEncoding, "gzip") {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(body); err == nil {
			if err := gz.Close(); err == nil {
				// 只有压缩后确实更小才使用压缩
				if buf.Len() < len(body) {
					return buf.Bytes(), "gzip"
				}
			}
		}
	}

	return body, ""
}

// buildCacheKey 生成缓存键
func buildCacheKey(method, upstreamURL string) string {
	h := sha256.Sum256([]byte(method + " " + upstreamURL))
	return "cache:v3:" + hex.EncodeToString(h[:])
}
