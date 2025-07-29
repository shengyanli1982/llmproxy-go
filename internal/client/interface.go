package client

import (
	"net/http"

	"github.com/shengyanli1982/llmproxy-go/internal/balance"
)

// HttpClient 代表HTTP客户端接口
type HttpClient interface {
	// Do 执行HTTP请求到指定上游服务
	Do(req *http.Request, upstream *balance.Upstream) (*http.Response, error)
	
	// Close 关闭客户端并清理资源
	Close() error
	
	// Name 获取客户端名称
	Name() string
}

// HttpClientFactory 代表HTTP客户端工厂接口
type HttpClientFactory interface {
	// Create 根据配置创建HTTP客户端
	Create(config *Config) (HttpClient, error)
}

// Config 代表HTTP客户端配置
type Config struct {
	MaxIdleConns        int  // 最大空闲连接数
	MaxIdleConnsPerHost int  // 每个主机最大空闲连接数
	MaxConnsPerHost     int  // 每个主机最大连接数
	IdleConnTimeout     int  // 空闲连接超时时间(秒)
	ConnectTimeout      int  // 连接超时时间(秒)
	RequestTimeout      int  // 请求超时时间(秒)
	EnableKeepAlive     bool // 是否启用Keep-Alive
	EnableRetry         bool // 是否启用重试
	MaxRetries          int  // 最大重试次数
	RetryDelay          int  // 重试延迟时间(毫秒)
	ProxyURL            string // 代理URL
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     90,
		ConnectTimeout:      10,
		RequestTimeout:      60,
		EnableKeepAlive:     true,
		EnableRetry:         true,
		MaxRetries:          3,
		RetryDelay:          1000,
		ProxyURL:            "",
	}
}