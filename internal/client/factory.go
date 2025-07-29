package client

import (
	"errors"
)

// 工厂相关错误定义
var (
	ErrNilConfig = errors.New("client config cannot be nil")
)

// clientFactory 代表HTTP客户端工厂实现
type clientFactory struct{}

// NewFactory 创建新的HTTP客户端工厂实例
func NewFactory() HTTPClientFactory {
	return &clientFactory{}
}

// Create 根据配置创建HTTP客户端
func (f *clientFactory) Create(config *Config) (HTTPClient, error) {
	if config == nil {
		return nil, ErrNilConfig
	}

	return NewHTTPClient(config)
}