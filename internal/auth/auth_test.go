package auth

import (
	"net/http"
	"testing"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBearerAuthenticator 测试Bearer Token认证
func TestBearerAuthenticator(t *testing.T) {
	// 测试有效token
	auth, err := NewBearerAuthenticator("test-token")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	err = auth.Apply(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "Bearer test-token"
	if got := req.Header.Get("Authorization"); got != expected {
		t.Errorf("Expected %s, got %s", expected, got)
	}

	// 测试空token
	_, err = NewBearerAuthenticator("")
	if err != ErrEmptyToken {
		t.Errorf("Expected ErrEmptyToken, got %v", err)
	}
}

// TestBearerAuthenticator_Apply_NilRequest 测试Bearer认证器处理nil请求
func TestBearerAuthenticator_Apply_NilRequest(t *testing.T) {
	auth, err := NewBearerAuthenticator("test-token")
	require.NoError(t, err)

	err = auth.Apply(nil)
	assert.Error(t, err)
}

// TestBasicAuthenticator 测试Basic Auth认证
func TestBasicAuthenticator(t *testing.T) {
	// 测试有效凭据
	auth, err := NewBasicAuthenticator("user", "pass")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	err = auth.Apply(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "Basic dXNlcjpwYXNz" // base64("user:pass")
	if got := req.Header.Get("Authorization"); got != expected {
		t.Errorf("Expected %s, got %s", expected, got)
	}

	// 测试空用户名
	_, err = NewBasicAuthenticator("", "pass")
	if err != ErrEmptyUsername {
		t.Errorf("Expected ErrEmptyUsername, got %v", err)
	}

	// 测试空密码
	_, err = NewBasicAuthenticator("user", "")
	if err != ErrEmptyPassword {
		t.Errorf("Expected ErrEmptyPassword, got %v", err)
	}
}

// TestBasicAuthenticator_Apply_NilRequest 测试Basic认证器处理nil请求
func TestBasicAuthenticator_Apply_NilRequest(t *testing.T) {
	auth, err := NewBasicAuthenticator("user", "pass")
	require.NoError(t, err)

	err = auth.Apply(nil)
	assert.Error(t, err)
}

// TestNoneAuthenticator 测试无认证
func TestNoneAuthenticator(t *testing.T) {
	auth := NewNoneAuthenticator()

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	err := auth.Apply(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// 确认没有设置Authorization头部
	if got := req.Header.Get("Authorization"); got != "" {
		t.Errorf("Expected empty Authorization header, got %s", got)
	}
}

// TestFactory 测试认证器工厂
func TestFactory(t *testing.T) {
	factory := NewFactory()

	// 测试Bearer认证
	bearerConfig := &config.AuthConfig{
		Type:  "bearer",
		Token: "test-token",
	}
	auth, err := factory.Create(bearerConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if auth.Type() != "bearer" {
		t.Errorf("Expected bearer type, got %s", auth.Type())
	}

	// 测试Basic认证
	basicConfig := &config.AuthConfig{
		Type:     "basic",
		Username: "user",
		Password: "pass",
	}
	auth, err = factory.Create(basicConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if auth.Type() != "basic" {
		t.Errorf("Expected basic type, got %s", auth.Type())
	}

	// 测试无认证
	noneConfig := &config.AuthConfig{
		Type: "none",
	}
	auth, err = factory.Create(noneConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if auth.Type() != "none" {
		t.Errorf("Expected none type, got %s", auth.Type())
	}

	// 测试无效类型
	invalidConfig := &config.AuthConfig{
		Type: "invalid",
	}
	_, err = factory.Create(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid auth type")
	}
}

// TestFactory_Create_ValidationErrors 测试工厂创建时的验证错误
func TestFactory_Create_ValidationErrors(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name      string
		config    *config.AuthConfig
		wantError string
	}{
		{
			name:      "nil config",
			config:    nil,
			wantError: "auth config cannot be nil",
		},
		{
			name: "bearer without token",
			config: &config.AuthConfig{
				Type:  "bearer",
				Token: "",
			},
			wantError: "invalid auth config: bearer token is required",
		},
		{
			name: "basic without username",
			config: &config.AuthConfig{
				Type:     "basic",
				Username: "",
				Password: "pass",
			},
			wantError: "invalid auth config: username and password are required for basic auth",
		},
		{
			name: "basic without password",
			config: &config.AuthConfig{
				Type:     "basic",
				Username: "user",
				Password: "",
			},
			wantError: "invalid auth config: username and password are required for basic auth",
		},
		{
			name: "empty type should default to none",
			config: &config.AuthConfig{
				Type: "",
			},
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := factory.Create(tt.config)

			if tt.wantError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				assert.Nil(t, auth)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, auth)
				assert.Equal(t, "none", auth.Type())
			}
		})
	}
}

// TestCreateFromConfig 测试从上游配置创建认证器
func TestCreateFromConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.UpstreamConfig
		wantType  string
		wantError bool
	}{
		{
			name: "config with bearer auth",
			config: &config.UpstreamConfig{
				Auth: &config.AuthConfig{
					Type:  "bearer",
					Token: "YOUR_OPENAI_API_KEY_HERE", // from config.default.yaml
				},
			},
			wantType:  "bearer",
			wantError: false,
		},
		{
			name: "config with basic auth",
			config: &config.UpstreamConfig{
				Auth: &config.AuthConfig{
					Type:     "basic",
					Username: "service_user", // from config.default.yaml
					Password: "service_pass",
				},
			},
			wantType:  "basic",
			wantError: false,
		},
		{
			name: "config with none auth",
			config: &config.UpstreamConfig{
				Auth: &config.AuthConfig{
					Type: "none",
				},
			},
			wantType:  "none",
			wantError: false,
		},
		{
			name: "config with nil auth - should default to none",
			config: &config.UpstreamConfig{
				Auth: nil,
			},
			wantType:  "none",
			wantError: false,
		},
		{
			name:      "nil config",
			config:    nil,
			wantType:  "",
			wantError: true,
		},
		{
			name: "invalid auth type in upstream config",
			config: &config.UpstreamConfig{
				Auth: &config.AuthConfig{
					Type: "invalid",
				},
			},
			wantType:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := CreateFromConfig(tt.config)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, auth)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, auth)
				assert.Equal(t, tt.wantType, auth.Type())
			}
		})
	}
}

// TestConfigDefaultYamlAuthTypes 测试config.default.yaml中的认证类型
func TestConfigDefaultYamlAuthTypes(t *testing.T) {
	factory := NewFactory()

	// Test auth types from config.default.yaml
	authTypes := []string{"bearer", "basic", "none"}

	for _, authType := range authTypes {
		t.Run("auth_type_"+authType, func(t *testing.T) {
			var authConfig *config.AuthConfig

			switch authType {
			case "bearer":
				authConfig = &config.AuthConfig{
					Type:  "bearer",
					Token: "YOUR_OPENAI_API_KEY_HERE", // from config.default.yaml
				}
			case "basic":
				authConfig = &config.AuthConfig{
					Type:     "basic",
					Username: "service_user", // from config.default.yaml
					Password: "service_pass",
				}
			case "none":
				authConfig = &config.AuthConfig{
					Type: "none",
				}
			}

			auth, err := factory.Create(authConfig)
			assert.NoError(t, err)
			assert.NotNil(t, auth)
			assert.Equal(t, authType, auth.Type())
		})
	}
}
