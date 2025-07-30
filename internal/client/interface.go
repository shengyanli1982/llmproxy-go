package client

import (
	"net/http"

	"github.com/shengyanli1982/llmproxy-go/internal/balance"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
)

// HTTPClient 代表HTTP客户端接口
type HTTPClient interface {
	// Do 执行HTTP请求到指定上游服务
	Do(req *http.Request, upstream *balance.Upstream) (*http.Response, error)

	// Close 关闭客户端并清理资源
	Close() error

	// Name 获取客户端名称
	Name() string
}

// HTTPClientFactory 代表HTTP客户端工厂接口
type HTTPClientFactory interface {
	// Create 根据配置创建HTTP客户端
	Create(config *config.HTTPClientConfig) (HTTPClient, error)
}
