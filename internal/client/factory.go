package client

import (
	"errors"
)

// 工厂相关错误定义
var (
	ErrNilConfig = errors.New("client config cannot be nil")
)

// defaultFactory 代表默认HTTP客户端工厂实现
type defaultFactory struct{}

// NewFactory 创建新的HTTP客户端工厂实例
func NewFactory() HttpClientFactory {
	return &defaultFactory{}
}

// Create 根据配置创建HTTP客户端
func (f *defaultFactory) Create(config *Config) (HttpClient, error) {
	if config == nil {
		return nil, ErrNilConfig
	}

	return NewHttpClient(config)
}