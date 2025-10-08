package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"cdnproxy/internal/cache"
	"cdnproxy/internal/config"

	"github.com/chai2010/webp"
)

var (
	commonBrowserUAs = []string{"Chrome", "Chromium", "Firefox", "Safari", "Edg", "OPR"}
	ipRefererPattern = regexp.MustCompile(`^https?://(\d{1,3}\.){3}\d{1,3}(:\d+)?/`)
	localhostReferer = regexp.MustCompile(`^https?://(localhost|127.0.0.1|0.0.0.0)(:\d+)?/`)
)

type Handler struct {
	cfg            config.Config
	cache          *cache.DiskCache
	whitelistStore WhitelistStore
	httpClient     *http.Client
	configStore    ConfigStore
	counterStore   CounterStore
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
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   100,  // 增加每个主机的连接池大小
		MaxConnsPerHost:       0,    // 0 表示不限制
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		// 启用 HTTP/2，对多路复用和头部压缩有帮助
		DisableCompression:    false, // 允许自动解压缩
		// 增大缓冲区，适配跨境高延迟网络
		WriteBufferSize:       64 * 1024, // 64KB 写缓冲
		ReadBufferSize:        64 * 1024, // 64KB 读缓冲
	}
	return &Handler{
		cfg:            cfg,
		cache:          diskCache,
		whitelistStore: whitelistStore,
		configStore:    configStore,
		counterStore:   counterStore,
		httpClient:     &http.Client{Transport: tr, Timeout: 0}, // 使用0表示无超时，让流式传输不受限制
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Incoming path is like: /cdn.jsdelivr.net/npm/bootstrap@... -> we must reconstruct the upstream URL
	upstreamURL, err := h.buildUpstreamURL(r)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Access control
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
			acceptEncoding := r.Header.Get("Accept-Encoding")
			compressedBody, encoding := h.compressBody(body, acceptEncoding)
			if encoding != "" {
				w.Header().Set("Content-Encoding", encoding)
				w.Header().Set("Vary", "Accept-Encoding")
				body = compressedBody
			}
		}

		w.WriteHeader(e.StatusCode)
		if method == http.MethodGet && len(body) > 0 {
			_, _ = w.Write(body)
		}
		return
	}

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
	
	// 不转发 Accept-Encoding 头，让上游返回未压缩的数据
	// 我们会在本地根据客户端需求进行压缩（第 293-299 行）
	// 这样可以避免 Go http.Client 自动解压缩导致的问题

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

		// 过滤压缩相关的头，因为我们会自己处理压缩
		// Content-Encoding: Go的http.Client会自动解压，我们需要重新压缩
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

	// 定义大文件阈值（5MB），大于此值的文件使用流式传输
	const largeFileThreshold = 5 * 1024 * 1024

	var body []byte
	if method == http.MethodGet {
		// 大文件直接流式传输，不缓存
		if contentLength > largeFileThreshold {
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

		// 小文件才读入内存进行处理和缓存
		var errCopy error
		body, errCopy = io.ReadAll(resp.Body)
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

		// 检查是否需要图片格式转换（只对小文件）
		userAgent := r.Header.Get("User-Agent")
		acceptHeader := r.Header.Get("Accept")
		if h.shouldConvertToWebP(contentType, userAgent, acceptHeader) {
			webpBody, err := h.convertToWebP(body, contentType)
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

// shouldConvertToWebP 判断是否应该转换为WebP格式
func (h *Handler) shouldConvertToWebP(contentType, userAgent, acceptHeader string) bool {
	// 只处理图片类型
	if !strings.HasPrefix(contentType, "image/") {
		return false
	}

	// 检查User-Agent是否支持WebP（主要判断依据）
	userAgent = strings.ToLower(userAgent)

	// Chrome/Chromium 23+ 支持WebP
	if strings.Contains(userAgent, "chrome") && !strings.Contains(userAgent, "edg") {
		return true
	}

	// Firefox 65+ 支持WebP (2019年1月发布)
	if strings.Contains(userAgent, "firefox") {
		// 提取Firefox版本号进行更精确的判断
		if version := h.extractFirefoxVersion(userAgent); version >= 65 {
			return true
		}
	}

	// Safari 14+ 支持WebP (2020年9月发布)
	if strings.Contains(userAgent, "safari") && !strings.Contains(userAgent, "chrome") {
		// 提取Safari版本号
		if version := h.extractSafariVersion(userAgent); version >= 14 {
			return true
		}
	}

	// Edge 18+ 支持WebP (2018年10月发布)
	if strings.Contains(userAgent, "edg") {
		return true
	}

	// Opera 12+ 支持WebP
	if strings.Contains(userAgent, "opr") || strings.Contains(userAgent, "opera") {
		return true
	}

	// 如果User-Agent不支持，但Accept头明确支持WebP，也可以转换
	// 这适用于一些API客户端或特殊工具
	if strings.Contains(acceptHeader, "image/webp") {
		return true
	}

	return false
}

// extractFirefoxVersion 提取Firefox版本号
func (h *Handler) extractFirefoxVersion(userAgent string) int {
	// 查找 "Firefox/" 后的版本号
	start := strings.Index(userAgent, "firefox/")
	if start == -1 {
		return 0
	}
	start += 8 // "firefox/".length

	// 找到版本号结束位置
	end := start
	for end < len(userAgent) && (userAgent[end] >= '0' && userAgent[end] <= '9' || userAgent[end] == '.') {
		end++
	}

	// 解析主版本号
	versionStr := userAgent[start:end]
	if dotIndex := strings.Index(versionStr, "."); dotIndex != -1 {
		versionStr = versionStr[:dotIndex]
	}

	// 转换为整数
	var version int
	fmt.Sscanf(versionStr, "%d", &version)
	return version
}

// extractSafariVersion 提取Safari版本号
func (h *Handler) extractSafariVersion(userAgent string) int {
	// 查找 "Version/" 后的版本号
	start := strings.Index(userAgent, "version/")
	if start == -1 {
		return 0
	}
	start += 8 // "version/".length

	// 找到版本号结束位置
	end := start
	for end < len(userAgent) && (userAgent[end] >= '0' && userAgent[end] <= '9' || userAgent[end] == '.') {
		end++
	}

	// 解析主版本号
	versionStr := userAgent[start:end]
	if dotIndex := strings.Index(versionStr, "."); dotIndex != -1 {
		versionStr = versionStr[:dotIndex]
	}

	// 转换为整数
	var version int
	fmt.Sscanf(versionStr, "%d", &version)
	return version
}

// buildCacheKey 生成缓存键
func buildCacheKey(method, upstreamURL string) string {
	h := sha256.Sum256([]byte(method + " " + upstreamURL))
	return "cache:v3:" + hex.EncodeToString(h[:])
}

// convertToWebP 将图片转换为WebP格式
func (h *Handler) convertToWebP(body []byte, contentType string) ([]byte, error) {
	// 只处理支持的图片格式
	if !strings.Contains(contentType, "image/jpeg") &&
		!strings.Contains(contentType, "image/png") &&
		!strings.Contains(contentType, "image/gif") {
		return nil, fmt.Errorf("unsupported image type: %s", contentType)
	}

	// 解码图片
	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %v", err)
	}

	// 转换为WebP格式
	var buf bytes.Buffer

	// 设置WebP编码选项
	options := &webp.Options{
		Lossless: false, // 使用有损压缩以获得更好的压缩率
		Quality:  80,    // 质量设置为80，平衡文件大小和图片质量
	}

	// 编码为WebP
	if err := webp.Encode(&buf, img, options); err != nil {
		return nil, fmt.Errorf("failed to encode WebP: %v", err)
	}

	webpData := buf.Bytes()

	// 只有WebP版本确实更小才返回WebP
	if len(webpData) < len(body) {
		return webpData, nil
	}

	// 如果WebP版本更大，返回原始图片
	return body, nil
}
