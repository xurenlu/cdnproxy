// Package admin 管理面板HTML模板
// 作者: rocky<m@some.im>

package admin

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

<div class="section">
  <h3>住宅IP代理管理</h3>
  <div class="proxy-stats">
    <div class="stat-item">
      <span class="stat-label">可用提供者:</span>
      <span class="stat-value" id="provider-count">-</span>
    </div>
    <div class="stat-item">
      <span class="stat-label">健康代理:</span>
      <span class="stat-value" id="healthy-proxies">-</span>
    </div>
    <div class="stat-item">
      <span class="stat-label">平均延迟:</span>
      <span class="stat-value" id="avg-latency">-</span>
    </div>
  </div>
  <div class="proxy-actions">
    <button onclick="refreshProxyStats()">刷新状态</button>
    <button onclick="showProxyProviders()">查看提供者</button>
  </div>
</div>
{{end}}`
