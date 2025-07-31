package config

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientConfig_Validation(t *testing.T) {
	validator := validator.New()

	tests := []struct {
		name    string
		config  HTTPClientConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields",
			config: HTTPClientConfig{
				Agent:     "LLMProxy/1.0",
				KeepAlive: 60000,
				Connect: &ConnectConfig{
					IdleTotal:   100,
					IdlePerHost: 10,
					MaxPerHost:  50,
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with zero values (use defaults)",
			config: HTTPClientConfig{
				Agent:     "LLMProxy/1.0",
				KeepAlive: 0, // 0 means disable keepalive
				Connect: &ConnectConfig{
					IdleTotal:   0, // 0 means use default
					IdlePerHost: 0, // 0 means use default
					MaxPerHost:  0, // 0 means use default
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with maximum values",
			config: HTTPClientConfig{
				Agent:     "LLMProxy/1.0",
				KeepAlive: 600000,
				Connect: &ConnectConfig{
					IdleTotal:   1000,
					IdlePerHost: 100,
					MaxPerHost:  500,
				},
			},
			wantErr: false,
		},
		{
			name: "valid config without connect section",
			config: HTTPClientConfig{
				Agent:     "LLMProxy/1.0",
				KeepAlive: 60000,
				Connect:   nil, // omit connect config
			},
			wantErr: false,
		},
		{
			name: "invalid keepalive - too high",
			config: HTTPClientConfig{
				Agent:     "LLMProxy/1.0",
				KeepAlive: 600001,
			},
			wantErr: true,
			errMsg:  "KeepAlive",
		},
		{
			name: "invalid keepalive - negative",
			config: HTTPClientConfig{
				Agent:     "LLMProxy/1.0",
				KeepAlive: -1,
			},
			wantErr: true,
			errMsg:  "KeepAlive",
		},
		{
			name: "invalid IdleTotal - too high",
			config: HTTPClientConfig{
				Agent: "LLMProxy/1.0",
				Connect: &ConnectConfig{
					IdleTotal: 1001,
				},
			},
			wantErr: true,
			errMsg:  "IdleTotal",
		},
		{
			name: "invalid IdlePerHost - too high",
			config: HTTPClientConfig{
				Agent: "LLMProxy/1.0",
				Connect: &ConnectConfig{
					IdlePerHost: 101,
				},
			},
			wantErr: true,
			errMsg:  "IdlePerHost",
		},
		{
			name: "invalid MaxPerHost - too high",
			config: HTTPClientConfig{
				Agent: "LLMProxy/1.0",
				Connect: &ConnectConfig{
					MaxPerHost: 501,
				},
			},
			wantErr: true,
			errMsg:  "MaxPerHost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Struct(&tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConnectConfig_Validation(t *testing.T) {
	validator := validator.New()

	tests := []struct {
		name    string
		config  ConnectConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields",
			config: ConnectConfig{
				IdleTotal:   100,
				IdlePerHost: 10,
				MaxPerHost:  50,
			},
			wantErr: false,
		},
		{
			name: "valid config with zero values",
			config: ConnectConfig{
				IdleTotal:   0,
				IdlePerHost: 0,
				MaxPerHost:  0,
			},
			wantErr: false,
		},
		{
			name: "valid config with maximum values",
			config: ConnectConfig{
				IdleTotal:   1000,
				IdlePerHost: 100,
				MaxPerHost:  500,
			},
			wantErr: false,
		},
		{
			name: "invalid IdleTotal - too high",
			config: ConnectConfig{
				IdleTotal: 1001,
			},
			wantErr: true,
			errMsg:  "IdleTotal",
		},
		{
			name: "invalid IdlePerHost - too high",
			config: ConnectConfig{
				IdlePerHost: 101,
			},
			wantErr: true,
			errMsg:  "IdlePerHost",
		},
		{
			name: "invalid MaxPerHost - too high",
			config: ConnectConfig{
				MaxPerHost: 501,
			},
			wantErr: true,
			errMsg:  "MaxPerHost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Struct(&tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPClientConfig_YAMLTags(t *testing.T) {
	// 测试YAML标签是否正确设置
	config := HTTPClientConfig{
		Agent:     "test-agent",
		KeepAlive: 30,
		Connect: &ConnectConfig{
			IdleTotal:   50,
			IdlePerHost: 5,
			MaxPerHost:  25,
		},
	}

	// 这个测试主要是确保结构体可以正常创建和使用
	assert.Equal(t, "test-agent", config.Agent)
	assert.Equal(t, 30, config.KeepAlive)
	assert.NotNil(t, config.Connect)
	assert.Equal(t, 50, config.Connect.IdleTotal)
	assert.Equal(t, 5, config.Connect.IdlePerHost)
	assert.Equal(t, 25, config.Connect.MaxPerHost)
}

func TestAuthConfig_ConditionalValidation(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create configuration manager: %v", err)
	}

	tests := []struct {
		name    string
		config  AuthConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid bearer auth",
			config: AuthConfig{
				Type:  "bearer",
				Token: "valid-token",
			},
			wantErr: false,
		},
		{
			name: "invalid bearer auth - missing token",
			config: AuthConfig{
				Type: "bearer",
			},
			wantErr: true,
			errMsg:  "Token",
		},
		{
			name: "valid basic auth",
			config: AuthConfig{
				Type:     "basic",
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
		{
			name: "invalid basic auth - missing username",
			config: AuthConfig{
				Type:     "basic",
				Password: "pass",
			},
			wantErr: true,
			errMsg:  "Username",
		},
		{
			name: "invalid basic auth - missing password",
			config: AuthConfig{
				Type:     "basic",
				Username: "user",
			},
			wantErr: true,
			errMsg:  "Password",
		},
		{
			name: "valid none auth",
			config: AuthConfig{
				Type: "none",
			},
			wantErr: false,
		},
		{
			name: "valid empty auth (defaults to none)",
			config: AuthConfig{
				Type: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validator.Struct(&tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHeaderOpConfig_ConditionalValidation(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create configuration manager: %v", err)
	}

	tests := []struct {
		name    string
		config  HeaderOpConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid insert operation",
			config: HeaderOpConfig{
				Op:    "insert",
				Key:   "X-Custom-Header",
				Value: "custom-value",
			},
			wantErr: false,
		},
		{
			name: "invalid insert operation - missing value",
			config: HeaderOpConfig{
				Op:  "insert",
				Key: "X-Custom-Header",
			},
			wantErr: true,
			errMsg:  "Value",
		},
		{
			name: "valid replace operation",
			config: HeaderOpConfig{
				Op:    "replace",
				Key:   "X-Custom-Header",
				Value: "new-value",
			},
			wantErr: false,
		},
		{
			name: "invalid replace operation - missing value",
			config: HeaderOpConfig{
				Op:  "replace",
				Key: "X-Custom-Header",
			},
			wantErr: true,
			errMsg:  "Value",
		},
		{
			name: "valid remove operation",
			config: HeaderOpConfig{
				Op:  "remove",
				Key: "X-Custom-Header",
			},
			wantErr: false,
		},
		{
			name: "valid remove operation with value (ignored)",
			config: HeaderOpConfig{
				Op:    "remove",
				Key:   "X-Custom-Header",
				Value: "ignored-value",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validator.Struct(&tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpstreamConfig_HTTPURLValidation(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create configuration manager: %v", err)
	}

	tests := []struct {
		name    string
		config  UpstreamConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid http URL",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "http://example.com",
			},
			wantErr: false,
		},
		{
			name: "valid https URL",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "https://example.com",
			},
			wantErr: false,
		},
		{
			name: "valid https URL with port and path",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "https://api.example.com:8080/v1/chat/completions",
			},
			wantErr: false,
		},
		{
			name: "valid http URL with credentials",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "http://user:pass@example.com:3000",
			},
			wantErr: false,
		},
		{
			name: "valid https URL with IPv6",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "https://[::1]:8080",
			},
			wantErr: false,
		},
		{
			name: "valid HTTP URL (uppercase)",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "HTTP://EXAMPLE.COM",
			},
			wantErr: false,
		},
		{
			name: "invalid ftp protocol",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "ftp://example.com",
			},
			wantErr: true,
			errMsg:  "http_url",
		},
		{
			name: "invalid file protocol",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "file:///path/to/file",
			},
			wantErr: true,
			errMsg:  "http_url",
		},
		{
			name: "missing protocol",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "example.com",
			},
			wantErr: true,
			errMsg:  "http_url",
		},
		{
			name: "empty URL",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "",
			},
			wantErr: true,
			errMsg:  "required",
		},
		{
			name: "URL without host",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "http://",
			},
			wantErr: true,
			errMsg:  "http_url",
		},
		{
			name: "URL without host but with path",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "https:///path",
			},
			wantErr: true,
			errMsg:  "http_url",
		},
		{
			name: "invalid URL format",
			config: UpstreamConfig{
				Name: "test-upstream",
				URL:  "http://invalid url with spaces",
			},
			wantErr: true,
			errMsg:  "http_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validator.Struct(&tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProxyConfig_HTTPURLValidation(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create configuration manager: %v", err)
	}

	tests := []struct {
		name    string
		config  ProxyConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid http proxy URL",
			config: ProxyConfig{
				URL: "http://proxy.example.com:8080",
			},
			wantErr: false,
		},
		{
			name: "valid https proxy URL",
			config: ProxyConfig{
				URL: "https://proxy.example.com:8080",
			},
			wantErr: false,
		},
		{
			name: "valid proxy URL with credentials",
			config: ProxyConfig{
				URL: "http://user:pass@proxy.example.com:8080",
			},
			wantErr: false,
		},
		{
			name: "invalid socks5 protocol",
			config: ProxyConfig{
				URL: "socks5://proxy.example.com:1080",
			},
			wantErr: true,
			errMsg:  "http_url",
		},
		{
			name: "missing protocol",
			config: ProxyConfig{
				URL: "proxy.example.com:8080",
			},
			wantErr: true,
			errMsg:  "http_url",
		},
		{
			name: "empty URL",
			config: ProxyConfig{
				URL: "",
			},
			wantErr: true,
			errMsg:  "required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validator.Struct(&tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBalanceConfig_Validation(t *testing.T) {
	validator := validator.New()

	tests := []struct {
		name    string
		config  BalanceConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid roundrobin strategy",
			config: BalanceConfig{
				Strategy: "roundrobin",
			},
			wantErr: false,
		},
		{
			name: "valid weighted_roundrobin strategy",
			config: BalanceConfig{
				Strategy: "weighted_roundrobin",
			},
			wantErr: false,
		},
		{
			name: "valid random strategy",
			config: BalanceConfig{
				Strategy: "random",
			},
			wantErr: false,
		},
		{
			name: "valid failover strategy",
			config: BalanceConfig{
				Strategy: "failover",
			},
			wantErr: false,
		},
		{
			name: "invalid response_aware strategy (removed)",
			config: BalanceConfig{
				Strategy: "response_aware",
			},
			wantErr: true,
			errMsg:  "Strategy",
		},
		{
			name: "invalid unknown strategy",
			config: BalanceConfig{
				Strategy: "unknown",
			},
			wantErr: true,
			errMsg:  "Strategy",
		},
		{
			name: "empty strategy (invalid, must specify strategy)",
			config: BalanceConfig{
				Strategy: "",
			},
			wantErr: true,
			errMsg:  "Strategy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Struct(&tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBreakerConfig_OptionalValidation(t *testing.T) {
	validator := validator.New()

	tests := []struct {
		name    string
		config  BreakerConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields",
			config: BreakerConfig{
				Threshold:   0.5,
				Cooldown:    30000,
				MaxRequests: 3,
				Interval:    10000,
			},
			wantErr: false,
		},
		{
			name: "valid config with zero values (use defaults)",
			config: BreakerConfig{
				Threshold:   0,
				Cooldown:    0,
				MaxRequests: 0,
				Interval:    0,
			},
			wantErr: false,
		},
		{
			name: "invalid threshold - too low",
			config: BreakerConfig{
				Threshold: 0.005,
				Cooldown:  30000,
			},
			wantErr: true,
			errMsg:  "Threshold",
		},
		{
			name: "invalid threshold - too high",
			config: BreakerConfig{
				Threshold: 1.5,
				Cooldown:  30000,
			},
			wantErr: true,
			errMsg:  "Threshold",
		},
		{
			name: "invalid cooldown - too high",
			config: BreakerConfig{
				Threshold: 0.5,
				Cooldown:  3600001,
			},
			wantErr: true,
			errMsg:  "Cooldown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Struct(&tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
