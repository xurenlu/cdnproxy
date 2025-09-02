package proxy

import (
	"errors"
	"fmt"
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
		for k, v := range e.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(e.StatusCode)
		if method == http.MethodGet && len(e.Body) > 0 {
			_, _ = w.Write(e.Body)
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
	for k, vals := range resp.Header {
		// Filter hop-by-hop headers
		if isHopByHopHeader(k) {
			continue
		}
		if len(vals) > 0 {
			w.Header()[k] = vals
			headers[k] = vals[0]
		}
	}
	w.WriteHeader(resp.StatusCode)

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
		_, _ = w.Write(body)
	}

	// Cache store
	_ = h.cache.Set(r.Context(), key, &cache.Entry{StatusCode: resp.StatusCode, Headers: headers, Body: body, StoredAt: time.Now()}, h.cfg.CacheTTL)
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
