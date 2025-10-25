package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cdnproxy/internal/config"
	"cdnproxy/internal/storage"
)

const sessionCookieName = "cp_session"

type Server struct {
	cfg            config.Config
	sessionStore   *storage.FileSessionStore
	whitelistStore WhitelistStore
	configStore    ConfigStore
	tplLogin       *template.Template
	tplIndex       *template.Template
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

func NewServer(cfg config.Config, whitelistStore WhitelistStore, configStore ConfigStore) (*Server, error) {
	sessionStore, err := storage.NewFileSessionStore(cfg.DataDir, cfg.SessionTTL)
	if err != nil {
		return nil, err
	}

	return NewServerWithSessionStore(cfg, whitelistStore, configStore, sessionStore)
}

func NewServerWithSessionStore(cfg config.Config, whitelistStore WhitelistStore, configStore ConfigStore, sessionStore *storage.FileSessionStore) (*Server, error) {
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

const layoutHTML = `{{define "layout"}}
<!doctype html>
<html lang="zh-cn">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>{{.Title}}</title>
  <style>
    body{font-family: system-ui, -apple-system, Segoe UI, Roboto, Arial; max-width: 820px; margin: 40px auto; padding: 0 16px;}
    header{display:flex;justify-content:space-between;align-items:center;margin-bottom:24px}
    table{border-collapse:collapse;width:100%}
    th,td{border:1px solid #ddd;padding:8px}
    th{background:#f7f7f7;text-align:left}
    form.inline{display:inline}
    input[type=text],input[type=password]{padding:8px;border:1px solid #ccc;border-radius:4px;width:100%}
    button{padding:8px 12px;border:1px solid #333;border-radius:4px;background:#333;color:#fff;cursor:pointer}
    button.secondary{background:#fff;color:#333}
    .error{color:#c00;margin:8px 0}
    .card{border:1px solid #eee;border-radius:8px;padding:16px;margin:12px 0}
  </style>
  </head>
  <body>
    {{template "content" .}}
  </body>
</html>
{{end}}`

const loginHTML = `{{define "login"}}{{template "layout" .}}{{end}}{{define "content"}}
<h1>登录 CDNProxy 管理</h1>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
<form method="post" action="/admin/login" class="card" style="max-width:420px">
  <div style="margin-bottom:8px">
    <label>用户名</label>
    <input name="username" placeholder="用户名" required />
  </div>
  <div style="margin-bottom:8px">
    <label>密码</label>
    <input name="password" type="password" placeholder="密码" required />
  </div>
  <button type="submit">登录</button>
  <p style="margin-top:8px;color:#666">请使用管理员提供的账号登录。</p>
{{end}}`

const indexHTML = `{{define "index"}}{{template "layout" .}}{{end}}{{define "content"}}
<header>
  <h1>CDNProxy 管理</h1>
  <form method="post" action="/admin/logout">
    <button class="secondary">登出 {{.AdminUser}}</button>
  </form>
  </header>

<div class="card">
  <h3>访问阈值配置</h3>
  <form method="post" action="/admin/config/update" style="margin:12px 0">
    <label>单一 Referer 主机名最近1天最多次数（超过需白名单）</label>
    <input name="threshold" type="number" min="1" value="{{.Threshold}}" />
    <button type="submit">保存</button>
  </form>
  <p style="color:#666">当前值：{{.Threshold}}。默认 1000。</p>
  <p style="color:#666">说明：Referer 为 IP/localhost 或非常见浏览器 UA 始终放行。</p>
  </div>

<div class="card">
  <h3>白名单后缀</h3>
  <form method="post" action="/admin/whitelist/add" style="margin:12px 0">
    <input name="suffix" placeholder="例如：example.com 或 .example.com" />
    <button type="submit">添加</button>
  </form>
  <table>
    <thead><tr><th>后缀</th><th style="width:160px">操作</th></tr></thead>
    <tbody>
      {{range .Suffixes}}
      <tr>
        <td>{{.}}</td>
        <td>
          <form method="post" action="/admin/whitelist/remove" class="inline">
            <input type="hidden" name="suffix" value="{{.}}" />
            <button class="secondary" type="submit">删除</button>
          </form>
        </td>
      </tr>
      {{else}}
      <tr><td colspan="2" style="color:#666">暂无白名单后缀</td></tr>
      {{end}}
    </tbody>
  </table>
  <p style="color:#666;margin-top:8px">当 Referer 为域名且其后缀在此列表中时允许访问。</p>
</div>
{{end}}`
