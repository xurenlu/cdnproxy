package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// 保存原始环境变量
	originalEnv := make(map[string]string)
	envVars := []string{
		"PORT", "REDIS_URL", "CACHE_TTL_SECONDS", "SESSION_TTL_SECONDS",
		"ADMIN_USERNAME", "ADMIN_PASSWORD", "WEBP_ENABLED",
		"IP_BAN_ENABLED", "IP_BAN_THRESHOLD", "IP_BAN_WINDOW_SEC", "IP_BAN_DURATION_SEC",
		"MAX_CONCURRENT_REQUESTS", "MAX_WEBSOCKET_CONNS",
		"LARGE_FILE_THRESHOLD", "VIDEO_FILE_THRESHOLD", "MAX_CACHE_FILE_SIZE",
		"API_DOMAINS", "LOOP_MAX", "LOOP_TIMEOUT",
	}
	for _, env := range envVars {
		originalEnv[env] = os.Getenv(env)
		os.Unsetenv(env)
	}

	// 测试完成后恢复环境变量
	defer func() {
		for env, val := range originalEnv {
			if val != "" {
				os.Setenv(env, val)
			}
		}
	}()

	tests := []struct {
		name        string
		setupEnv    func()
		wantErr     bool
		validateCfg func(*testing.T, Config)
	}{
		{
			name: "默认配置 - 无密码生成临时密码",
			setupEnv: func() {
				os.Setenv("PORT", "9090")
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg Config) {
				if cfg.Port != "9090" {
					t.Errorf("期望 Port = 9090, 实际 = %s", cfg.Port)
				}
				if cfg.AdminPassword == "" {
					t.Error("期望生成临时密码")
				}
				if cfg.MaxConcurrentRequests != 50 {
					t.Errorf("期望 MaxConcurrentRequests = 50, 实际 = %d", cfg.MaxConcurrentRequests)
				}
			},
		},
		{
			name: "自定义配置",
			setupEnv: func() {
				os.Setenv("PORT", "8080")
				os.Setenv("ADMIN_PASSWORD", "test12345")
				os.Setenv("MAX_CONCURRENT_REQUESTS", "100")
				os.Setenv("LARGE_FILE_THRESHOLD", "2097152") // 2MB
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg Config) {
				if cfg.AdminPassword != "test12345" {
					t.Errorf("期望 AdminPassword = test12345, 实际 = %s", cfg.AdminPassword)
				}
				if cfg.MaxConcurrentRequests != 100 {
					t.Errorf("期望 MaxConcurrentRequests = 100, 实际 = %d", cfg.MaxConcurrentRequests)
				}
				if cfg.LargeFileThreshold != 2097152 {
					t.Errorf("期望 LargeFileThreshold = 2097152, 实际 = %d", cfg.LargeFileThreshold)
				}
			},
		},
		{
			name: "密码太短",
			setupEnv: func() {
				os.Setenv("ADMIN_PASSWORD", "short")
			},
			wantErr: true,
		},
		{
			name: "无效的并发数",
			setupEnv: func() {
				os.Setenv("ADMIN_PASSWORD", "longenough123")
				os.Setenv("MAX_CONCURRENT_REQUESTS", "0")
			},
			wantErr: true,
		},
		{
			name: "LOOP_MAX / LOOP_TIMEOUT",
			setupEnv: func() {
				os.Setenv("ADMIN_PASSWORD", "longenough123")
				os.Setenv("LOOP_MAX", "100")
				os.Setenv("LOOP_TIMEOUT", "3600")
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg Config) {
				if cfg.LoopMax != 100 {
					t.Errorf("LoopMax = %d, want 100", cfg.LoopMax)
				}
				if !cfg.LoopTimeoutSet || cfg.LoopTimeoutSec != 3600 {
					t.Errorf("LoopTimeoutSet=%v LoopTimeoutSec=%d, want true 3600", cfg.LoopTimeoutSet, cfg.LoopTimeoutSec)
				}
			},
		},
		{
			name: "LOOP_MAX 无法解析时忽略不失败",
			setupEnv: func() {
				os.Setenv("ADMIN_PASSWORD", "longenough123")
				os.Setenv("LOOP_MAX", "notint")
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg Config) {
				if cfg.LoopMax != 0 {
					t.Errorf("LoopMax = %d, want 0 (ignored)", cfg.LoopMax)
				}
			},
		},
		{
			name: "LOOP_MAX 为 0 时忽略不失败",
			setupEnv: func() {
				os.Setenv("ADMIN_PASSWORD", "longenough123")
				os.Setenv("LOOP_MAX", "0")
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg Config) {
				if cfg.LoopMax != 0 {
					t.Errorf("LoopMax = %d, want 0 (ignored)", cfg.LoopMax)
				}
			},
		},
		{
			name: "LOOP_TIMEOUT 无法解析时忽略不失败",
			setupEnv: func() {
				os.Setenv("ADMIN_PASSWORD", "longenough123")
				os.Setenv("LOOP_TIMEOUT", "bad")
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg Config) {
				if cfg.LoopTimeoutSet {
					t.Error("LoopTimeoutSet should be false when value invalid")
				}
			},
		},
		{
			name: "LOOP_TIMEOUT 为 0 表示立即计时到期",
			setupEnv: func() {
				os.Setenv("ADMIN_PASSWORD", "longenough123")
				os.Setenv("LOOP_TIMEOUT", "0")
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg Config) {
				if !cfg.LoopTimeoutSet || cfg.LoopTimeoutSec != 0 {
					t.Errorf("LoopTimeoutSet=%v LoopTimeoutSec=%d, want true 0", cfg.LoopTimeoutSet, cfg.LoopTimeoutSec)
				}
			},
		},
		{
			name: "API_DOMAINS 逗号分隔",
			setupEnv: func() {
				os.Setenv("ADMIN_PASSWORD", "longenough123")
				os.Setenv("API_DOMAINS", "api.example.com,api.test.com")
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg Config) {
				hasExample := false
				hasTest := false
				for _, domain := range cfg.APIDomains {
					if domain == "api.example.com" {
						hasExample = true
					}
					if domain == "api.test.com" {
						hasTest = true
					}
				}
				if !hasExample {
					t.Error("期望包含 api.example.com")
				}
				if !hasTest {
					t.Error("期望包含 api.test.com")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清理环境变量
			for _, env := range envVars {
				os.Unsetenv(env)
			}
			// 设置测试环境
			tt.setupEnv()

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Error("期望返回错误，但没有")
				}
				return
			}
			if err != nil {
				t.Errorf("不期望返回错误: %v", err)
				return
			}
			if tt.validateCfg != nil {
				tt.validateCfg(t, cfg)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "有效配置",
			cfg: Config{
				MaxConcurrentRequests: 50,
				MaxWebSocketConns:     10,
				LargeFileThreshold:    1024,
				VideoFileThreshold:    1024,
				MaxCacheFileSize:      1024,
				IPBanThreshold:        10,
				IPBanWindowSec:        60,
				IPBanDuration:         300,
			},
			wantErr: false,
		},
		{
			name: "无效的并发数",
			cfg: Config{
				MaxConcurrentRequests: 0,
			},
			wantErr: true,
		},
		{
			name: "无效的文件大小阈值",
			cfg: Config{
				MaxConcurrentRequests: 50,
				LargeFileThreshold:    0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetenvBool(t *testing.T) {
	tests := []struct {
		key    string
		value  string
		def    bool
		want   bool
	}{
		{"TEST", "true", false, true},
		{"TEST", "True", false, true},
		{"TEST", "TRUE", false, true},
		{"TEST", "1", false, true},
		{"TEST", "yes", false, true},
		{"TEST", "on", false, true},
		{"TEST", "false", true, false},
		{"TEST", "0", true, false},
		{"TEST", "no", true, false},
		{"TEST", "off", true, false},
		{"TEST", "", true, true}, // 默认值
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
				defer os.Unsetenv(tt.key)
			}
			if got := getenvBool(tt.key, tt.def); got != tt.want {
				t.Errorf("getenvBool(%q, %v) = %v, want %v", tt.key, tt.def, got, tt.want)
			}
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		input    string
		sep      string
		expected []string
	}{
		{"a,b,c", ",", []string{"a", "b", "c"}},
		{" a , b , c ", ",", []string{"a", "b", "c"}},
		{"a, b, c", ",", []string{"a", "b", "c"}},
		{"a,,b", ",", []string{"a", "b"}},
		{"", ",", nil},
		{"single", ",", []string{"single"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitAndTrim(tt.input, tt.sep)
			if len(got) != len(tt.expected) {
				t.Errorf("splitAndTrim(%q, %q) 长度 = %d, 期望 %d", tt.input, tt.sep, len(got), len(tt.expected))
				return
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("splitAndTrim(%q, %q)[%d] = %q, 期望 %q", tt.input, tt.sep, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestGenerateRandomHex(t *testing.T) {
	hex1 := generateRandomHex(16)
	hex2 := generateRandomHex(16)

	if len(hex1) != 32 { // 16 字节 = 32 十六进制字符
		t.Errorf("generateRandomHex(16) 长度 = %d, 期望 32", len(hex1))
	}

	if hex1 == hex2 {
		t.Error("两次生成的随机数应该不同")
	}
}
