package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cdnproxy/internal/config"
	"cdnproxy/internal/storage"

	redis "github.com/redis/go-redis/v9"
)

const sessionCookieName = "cp_session"

// SessionStore 会话存储接口
type SessionStore interface {
	Set(token, value string) error
	Exists(token string) bool
	Delete(token string) error
}

type Server struct {
	cfg            config.Config
	sessionStore   SessionStore
	whitelistStore WhitelistStore
	configStore    ConfigStore
	tplLogin       *template.Template
	tplIndex       *template.Template
	proxyManager   ProxyManager // 代理管理器接口
}

// WhitelistStore 接口定义
type WhitelistStore interface {
	List(ctx context.Context) ([]string, error)
	Add(ctx context.Context, suffix string) error
	Remove(ctx context.Context, suffix string) error
}

// ConfigStore 接口定义
type ConfigStore interface {
	GetReferrerThreshold(ctx context.Context) (int64, error)
	SetReferrerThreshold(ctx context.Context, n int64) error
}

// ProxyManager 代理管理器接口
type ProxyManager interface {
	GetHealthStatus() map[string]interface{}
	GetProxyStats() map[string]interface{}
	GetProviderNames() []string
	GetProviderCount() int
}

func NewServer(cfg config.Config, redisClient *redis.Client, whitelistStore WhitelistStore, configStore ConfigStore) (*Server, error) {
	sessionStore := storage.NewRedisSessionStore(redisClient, cfg.SessionTTL)
	return NewServerWithSessionStore(cfg, whitelistStore, configStore, sessionStore)
}

func NewServerWithSessionStore(cfg config.Config, whitelistStore WhitelistStore, configStore ConfigStore, sessionStore SessionStore) (*Server, error) {
	s := &Server{
		cfg:            cfg,
		sessionStore:   sessionStore,
		whitelistStore: whitelistStore,
		configStore:    configStore,
	}
	s.tplLogin = template.Must(template.New("base_login").Parse(layoutHTML + loginHTML))
	s.tplIndex = template.Must(template.New("base_index").Parse(layoutHTML + indexHTML))
	return s, nil
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/login", s.handleLogin)
	mux.Handle("/admin/", s.authMiddleware(http.StripPrefix("/admin", s.adminMux())))
}

func (s *Server) adminMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/logout", s.handleLogout)
	mux.HandleFunc("/whitelist/add", s.handleWhitelistAdd)
	mux.HandleFunc("/whitelist/remove", s.handleWhitelistRemove)
	mux.HandleFunc("/config/update", s.handleConfigUpdate)
	mux.HandleFunc("/proxy/stats", s.handleProxyStats)
	mux.HandleFunc("/proxy/health", s.handleProxyHealth)
	mux.HandleFunc("/proxy/providers", s.handleProxyProviders)
	return mux
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		_ = s.tplLogin.ExecuteTemplate(w, "login", map[string]any{"Title": "Admin Login"})
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		username := strings.TrimSpace(r.Form.Get("username"))
		password := strings.TrimSpace(r.Form.Get("password"))
		if username == s.cfg.AdminUsername && password == s.cfg.AdminPassword {
			token := randomHex(16)
			if err := s.sessionStore.Set(token, "1"); err != nil {
				http.Error(w, "session error", http.StatusInternalServerError)
				return
			}
			cookie := &http.Cookie{
				Name:     sessionCookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Secure:   isRequestSecure(r),
				SameSite: http.SameSiteLaxMode,
				Expires:  time.Now().Add(s.cfg.SessionTTL),
			}
			http.SetCookie(w, cookie)
			http.Redirect(w, r, "/admin/", http.StatusSeeOther)
			return
		}
		_ = s.tplLogin.ExecuteTemplate(w, "login", map[string]any{"Title": "Admin Login", "Error": "用户名或密码错误"})
		return
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := getSessionCookie(r)
	if token != "" {
		_ = s.sessionStore.Delete(token)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", Expires: time.Unix(0, 0), MaxAge: -1})
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	suffixes, err := s.whitelistStore.List(ctx)
	if err != nil {
		http.Error(w, "failed to load whitelist", http.StatusInternalServerError)
		return
	}
	threshold, _ := s.configStore.GetReferrerThreshold(ctx)
	_ = s.tplIndex.ExecuteTemplate(w, "index", map[string]any{
		"Title":     "CDNProxy 管理",
		"Suffixes":  suffixes,
		"AdminUser": s.cfg.AdminUsername,
		"Threshold": threshold,
	})
}

func (s *Server) handleWhitelistAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	suffix := strings.TrimSpace(r.Form.Get("suffix"))
	if suffix != "" {
		_ = s.whitelistStore.Add(r.Context(), strings.ToLower(suffix))
	}
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func (s *Server) handleWhitelistRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	suffix := strings.TrimSpace(r.Form.Get("suffix"))
	if suffix != "" {
		_ = s.whitelistStore.Remove(r.Context(), strings.ToLower(suffix))
	}
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow direct access to /login when strip prefix is not applied
		if strings.HasPrefix(r.URL.Path, "/login") {
			next.ServeHTTP(w, r)
			return
		}
		token := getSessionCookie(r)
		if token == "" {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		if !s.sessionStore.Exists(token) {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	val := strings.TrimSpace(r.Form.Get("threshold"))
	if val == "" {
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
		return
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		http.Error(w, "invalid threshold", http.StatusBadRequest)
		return
	}
	if err := s.configStore.SetReferrerThreshold(r.Context(), n); err != nil {
		http.Error(w, "save error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func getSessionCookie(r *http.Request) string {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func isRequestSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	// Respect common reverse proxy headers
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return false
}

// 模板常量已移至 templates.go

// 代理管理处理函数
func (s *Server) handleProxyStats(w http.ResponseWriter, r *http.Request) {
	if s.proxyManager == nil {
		http.Error(w, "代理管理器未初始化", http.StatusInternalServerError)
		return
	}

	stats := s.proxyManager.GetProxyStats()
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{
		"provider_count": %d,
		"healthy_proxies": %v,
		"unhealthy_proxies": %v,
		"average_latency": %v
	}`,
		s.proxyManager.GetProviderCount(),
		stats["healthy_proxies"],
		stats["unhealthy_proxies"],
		stats["average_latency"])))
}

func (s *Server) handleProxyHealth(w http.ResponseWriter, r *http.Request) {
	if s.proxyManager == nil {
		http.Error(w, "代理管理器未初始化", http.StatusInternalServerError)
		return
	}

	health := s.proxyManager.GetHealthStatus()
	w.Header().Set("Content-Type", "application/json")

	// 简化健康状态输出
	healthyCount := 0
	totalCount := len(health)
	for _, result := range health {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if isHealthy, exists := resultMap["is_healthy"]; exists {
				if healthy, ok := isHealthy.(bool); ok && healthy {
					healthyCount++
				}
			}
		}
	}

	w.Write([]byte(fmt.Sprintf(`{
		"total_proxies": %d,
		"healthy_proxies": %d,
		"health_rate": %.2f
	}`, totalCount, healthyCount, float64(healthyCount)/float64(totalCount))))
}

func (s *Server) handleProxyProviders(w http.ResponseWriter, r *http.Request) {
	if s.proxyManager == nil {
		http.Error(w, "代理管理器未初始化", http.StatusInternalServerError)
		return
	}

	providers := s.proxyManager.GetProviderNames()
	w.Header().Set("Content-Type", "application/json")

	providersJSON := "["
	for i, provider := range providers {
		if i > 0 {
			providersJSON += ","
		}
		providersJSON += fmt.Sprintf(`"%s"`, provider)
	}
	providersJSON += "]"

	w.Write([]byte(fmt.Sprintf(`{
		"providers": %s,
		"count": %d
	}`, providersJSON, len(providers))))
}
