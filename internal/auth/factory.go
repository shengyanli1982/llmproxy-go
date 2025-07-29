package auth

import (
	"errors"
	"fmt"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
)

// 工厂相关错误定义
var (
	ErrInvalidAuthType   = errors.New("invalid auth type")
	ErrNilAuthConfig     = errors.New("auth config cannot be nil")
	ErrInvalidAuthConfig = errors.New("invalid auth config")
)

// defaultFactory 代表默认认证器工厂实现
type defaultFactory struct{}

// NewFactory 创建新的认证器工厂实例
func NewFactory() AuthenticatorFactory {
	return &defaultFactory{}
}

// Create 根据配置创建对应的认证器
// authConfig: 认证配置信息
func (f *defaultFactory) Create(authConfig *config.AuthConfig) (Authenticator, error) {
	if authConfig == nil {
		return nil, ErrNilAuthConfig
	}

	switch authConfig.Type {
	case "none", "":
		// 默认使用无认证
		return NewNoneAuthenticator(), nil

	case "bearer":
		if authConfig.Token == "" {
			return nil, fmt.Errorf("%w: bearer token is required", ErrInvalidAuthConfig)
		}
		return NewBearerAuthenticator(authConfig.Token)

	case "basic":
		if authConfig.Username == "" || authConfig.Password == "" {
			return nil, fmt.Errorf("%w: username and password are required for basic auth", ErrInvalidAuthConfig)
		}
		return NewBasicAuthenticator(authConfig.Username, authConfig.Password)

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidAuthType, authConfig.Type)
	}
}

// CreateFromConfig 从上游配置创建认证器的便捷方法
// upstreamConfig: 上游配置
func CreateFromConfig(upstreamConfig *config.UpstreamConfig) (Authenticator, error) {
	if upstreamConfig == nil {
		return nil, errors.New("upstream config cannot be nil")
	}

	// 如果没有认证配置，使用默认的无认证
	if upstreamConfig.Auth == nil {
		return NewNoneAuthenticator(), nil
	}

	factory := NewFactory()
	return factory.Create(upstreamConfig.Auth)
}