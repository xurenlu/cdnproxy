package proxy

import (
	"net/http"
	"strings"
)

// isAccessAllowed 检查请求是否被允许访问
func (h *Handler) isAccessAllowed(r *http.Request) (bool, string) {
	// 移除所有限制，允许所有访问
	return true, "access allowed"
}

// isHopByHopHeader 判断是否是 hop-by-hop 头
func isHopByHopHeader(k string) bool {
	switch strings.ToLower(k) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	}
	return false
}

// copyHeaders 复制 HTTP 头（过滤 hop-by-hop 头）
func copyHeaders(dst, src http.Header) {
	for k, vals := range src {
		if isHopByHopHeader(k) {
			continue
		}
		dst[k] = vals
	}
}

// containsAny 检查字符串是否包含任意子串
func containsAny(s string, subs []string) bool {
	s = strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(s, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
