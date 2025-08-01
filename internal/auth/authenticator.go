package auth

import (
	"net/http"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
)

// Authenticator 代表认证器接口，定义HTTP请求认证的行为
type Authenticator interface {
	// Apply 将认证信息应用到HTTP请求中
	// req: 要应用认证的HTTP请求
	Apply(req *http.Request) error

	// Type 获取认证器类型
	Type() string
}

// AuthenticatorFactory 代表认证器工厂接口
type AuthenticatorFactory interface {
	// Create 根据配置创建认证器
	// authConfig: 认证配置信息
	Create(authConfig *config.AuthConfig) (Authenticator, error)
}
