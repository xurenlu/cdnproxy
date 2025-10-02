package proxy

import (
	"bytes"
	"compress/gzip"
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
	"cdnproxy/internal/storage"

	"github.com/chai2010/webp"
)

var (
	commonBrowserUAs = []string{"Chrome", "Chromium", "Firefox", "Safari", "Edg", "OPR"}
	ipRefererPattern = regexp.MustCompile(`^https?://(\d{1,3}\.){3}\d{1,3}(:\d+)?/`)
	localhostReferer = regexp.MustCompile(`^https?://(localhost|127.0.0.1|0.0.0.0)(:\d+)?/`)
)

type Handler struct {
	cfg            config.Config
	cache          *cache.Cache
	whitelistStore *storage.WhitelistStore
	httpClient     *http.Client
	configStore    *storage.ConfigStore
	counterStore   *storage.CounterStore
}

func NewHandler(cfg config.Config, cacheStore *cache.Cache, whitelistStore *storage.WhitelistStore, configStore *storage.ConfigStore, counterStore *storage.CounterStore) http.Handler {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          200,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &Handler{
		cfg:            cfg,
		cache:          cacheStore,
		whitelistStore: whitelistStore,
		configStore:    configStore,
		counterStore:   counterStore,
		httpClient:     &http.Client{Transport: tr, Timeout: 60 * time.Second},
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

	key := h.cache.BuildKey(method, upstreamURL)
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
	req.Header.Set("User-Agent", "cdnproxy/1.0")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy headers
	headers := map[string]string{}
	contentType := ""
	for k, vals := range resp.Header {
		// Filter hop-by-hop headers
		if isHopByHopHeader(k) {
			continue
		}
		// 过滤压缩相关的头，因为我们会自己处理压缩
		// Content-Encoding: Go的http.Client会自动解压，我们需要重新压缩
		// Content-Length: 压缩后长度会变化，由Go自动设置
		lowerK := strings.ToLower(k)
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

	var body []byte
	if method == http.MethodGet {
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

		// 检查是否需要图片格式转换
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
		// 根据内容类型确定TTL
		ttl := cache.GetTTLByContentType(contentType, h.cfg.CacheTTL)
		_ = h.cache.Set(r.Context(), key, &cache.Entry{
			StatusCode:  resp.StatusCode,
			Headers:     headers,
			Body:        body, // 存储未压缩的原始数据
			StoredAt:    time.Now(),
			ContentType: contentType,
		}, ttl)

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
	req.Header.Set("User-Agent", "cdnproxy/1.0")

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
