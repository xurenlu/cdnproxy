#!/usr/bin/env ruby
# frozen_string_literal: true

# 本地验证 LOOP_MAX / LOOP_TIMEOUT：启动 cdnproxy 子进程，用 HTTP 请求 /healthz 观察优雅退出。
# 用法：
#   cd 项目根目录 && go build -o cdnproxy-test .
#   REDIS_URL=redis://127.0.0.1:6379/0 ADMIN_PASSWORD=yourpass ruby scripts/test_loop_lifecycle.rb ./cdnproxy-test
#
# 未传二进制路径时默认项目根目录下的 cdnproxy-test。

require "net/http"
require "timeout"
require "socket"

ROOT = File.expand_path("..", __dir__)
BINARY = ARGV[0] || File.join(ROOT, "cdnproxy-test")

# 仅探测 TCP 监听，不发送 HTTP（否则会占用 LOOP_MAX 计数）
def wait_tcp_ready(host, port, timeout_sec: 15)
  deadline = Time.now + timeout_sec
  loop do
    begin
      Socket.tcp(host, port, connect_timeout: 1).close
      return true
    rescue Errno::ECONNREFUSED, Errno::EHOSTUNREACH, SocketError, IOError, SystemCallError
      # retry
    end
    raise "server TCP not ready within #{timeout_sec}s" if Time.now > deadline

    sleep 0.05
  end
end

def http_get_healthz(host, port)
  Net::HTTP.start(host, port, read_timeout: 5, open_timeout: 2) do |http|
    http.get("/healthz")
  end
end

def run_subprocess(extra_env, log_path)
  # 不用整份 ENV，避免本机已 export 的 LOOP_* 泄漏到子进程
  child = {
    "PATH" => ENV.fetch("PATH", "/usr/bin:/bin"),
    "HOME" => ENV["HOME"],
    "LANG" => ENV["LANG"],
    "TZ" => ENV["TZ"]
  }.compact.merge(extra_env)
  log = File.open(log_path, "w")
  pid = spawn(child, BINARY, out: log, err: log)
  [pid, log]
end

def kill_tree(pid)
  Process.kill("TERM", pid)
rescue Errno::ESRCH
  # already gone
end

def wait_pid(pid, timeout_sec: 30)
  Timeout.timeout(timeout_sec) { Process.wait2(pid) }
rescue Timeout::Error
  kill_tree(pid)
  Process.wait2(pid) rescue nil
  raise
end

abort "找不到可执行文件: #{BINARY}（请先 go build -o cdnproxy-test .）" unless File.executable?(BINARY)

host = "127.0.0.1"
port = (ENV["LOOP_TEST_PORT1"] || 28_180).to_i
redis_url = ENV["REDIS_URL"] || "redis://127.0.0.1:6379/0"
admin_pass = ENV["ADMIN_PASSWORD"] || "looptest_pass_12345"

base_env = {
  "PORT" => port.to_s,
  "REDIS_URL" => redis_url,
  "ADMIN_PASSWORD" => admin_pass,
  "IP_BAN_ENABLED" => "false"
}

puts "== 1) LOOP_MAX：第 N 次请求完成后进程应退出，第 N+1 次连接应失败 =="
tmpdir = File.join(ROOT, "tmp")
Dir.mkdir(tmpdir) unless Dir.exist?(tmpdir)
log1 = File.join(tmpdir, "loop-max.log")
File.delete(log1) if File.exist?(log1)

env1 = base_env.merge("LOOP_MAX" => "3")
env1.delete("LOOP_TIMEOUT")

pid1, log_io = run_subprocess(env1, log1)
log_io.close

begin
  wait_tcp_ready(host, port)
  3.times do |i|
    res = http_get_healthz(host, port)
    puts "  请求 #{i + 1}/3: HTTP #{res.code}"
    abort "unexpected status" unless res.code.to_i == 200
  end
  sleep 0.3
  begin
    http_get_healthz(host, port)
    abort "FAIL: 预期第 4 次请求前服务已退出"
  rescue Errno::ECONNREFUSED, Errno::EHOSTUNREACH, Net::OpenTimeout, EOFError
    puts "  第 4 次: 连接被拒绝或超时（符合预期）"
  end
  _status = wait_pid(pid1, timeout_sec: 45)
  puts "  子进程已退出: #{$?.inspect}"
rescue StandardError => e
  warn "  错误: #{e.full_message}"
  kill_tree(pid1)
  wait_pid(pid1, timeout_sec: 5) rescue nil
  raise
end

puts File.read(log1).lines.last(8).map { |l| "  [log] #{l}" }.join

puts
puts "== 2) LOOP_TIMEOUT：约 2 秒后应优雅退出 =="
port2 = (ENV["LOOP_TEST_PORT2"] || 28_181).to_i
log2 = File.join(tmpdir, "loop-timeout.log")
File.delete(log2) if File.exist?(log2)

env2 = base_env.merge("PORT" => port2.to_s, "LOOP_TIMEOUT" => "2")
env2.delete("LOOP_MAX")

pid2, log_io2 = run_subprocess(env2, log2)
log_io2.close

t0 = Process.clock_gettime(Process::CLOCK_MONOTONIC)
begin
  wait_tcp_ready(host, port2)
  http_get_healthz(host, port2)
  puts "  首次 /healthz 成功"
  sleep 0.2
  loop do
    begin
      http_get_healthz(host, port2)
    rescue Errno::ECONNREFUSED, Errno::EHOSTUNREACH, Net::OpenTimeout, EOFError
      break
    end
    sleep 0.05
  end
  elapsed = Process.clock_gettime(Process::CLOCK_MONOTONIC) - t0
  puts "  服务不可达，耗时约 #{elapsed.round(2)}s（期望 >= 1.8s）"
  abort "FAIL: 超时退出太慢" if elapsed < 1.8
  abort "FAIL: 超时退出太快（可能未启动）" if elapsed > 10

  wait_pid(pid2, timeout_sec: 45)
  puts "  子进程已退出: #{$?.inspect}"
rescue StandardError => e
  warn "  错误: #{e.full_message}"
  kill_tree(pid2)
  wait_pid(pid2, timeout_sec: 5) rescue nil
  raise
end

puts File.read(log2).lines.last(8).map { |l| "  [log] #{l}" }.join

puts
puts "== 全部通过 =="
