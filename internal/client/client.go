package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/shengyanli1982/llmproxy-go/internal/auth"
	"github.com/shengyanli1982/llmproxy-go/internal/balance"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/headers"
)

// 客户端相关错误定义
var (
	ErrNilRequest     = errors.New("request cannot be nil")
	ErrNilUpstream    = errors.New("upstream cannot be nil")
	ErrClientClosed   = errors.New("client is closed")
	ErrInvalidTimeout = errors.New("invalid timeout configuration")
)

// httpClient HTTP客户端实现
type httpClient struct {
	name         string
	client       *http.Client
	pool         *ConnectionPool
	retryHandler *RetryHandler
	proxyHandler *ProxyHandler
	config       *config.HTTPClientConfig
	closed       bool

	// 依赖的模块
	authFactory    auth.AuthenticatorFactory
	headerOperator headers.HeaderOperator
}

// NewHTTPClient 创建新的HTTP客户端实例
func NewHTTPClient(cfg *config.HTTPClientConfig) (HTTPClient, error) {
	// 创建连接池
	pool := NewConnectionPool(cfg)

	// 创建重试处理器
	retryHandler := NewRetryHandler(cfg)

	// 创建代理处理器
	var proxyHandler *ProxyHandler
	if cfg.Proxy != nil {
		proxyHandler = NewProxyHandler(cfg.Proxy)
	} else {
		proxyHandler = NewProxyHandlerFromURL("")
	}

	// 获取请求超时时间
	requestTimeout := 60 // 默认60秒
	if cfg.Timeout != nil {
		requestTimeout = cfg.Timeout.Request
	}

	// 创建HTTP客户端
	client := &http.Client{
		Transport: pool.GetTransport(),
		Timeout:   time.Duration(requestTimeout) * time.Second,
	}

	// 设置代理
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.Proxy = proxyHandler.GetProxyFunc()
	}

	return &httpClient{
		name:           fmt.Sprintf("http-client-%d", time.Now().UnixNano()),
		client:         client,
		pool:           pool,
		retryHandler:   retryHandler,
		proxyHandler:   proxyHandler,
		config:         cfg,
		closed:         false,
		authFactory:    auth.NewFactory(),
		headerOperator: headers.NewOperator(),
	}, nil
}

// Do 执行HTTP请求到指定上游服务
func (c *httpClient) Do(req *http.Request, upstream *balance.Upstream) (*http.Response, error) {
	if c.closed {
		return nil, ErrClientClosed
	}
	if req == nil {
		return nil, ErrNilRequest
	}
	if upstream == nil {
		return nil, ErrNilUpstream
	}

	// 准备请求
	if err := c.prepareRequest(req, upstream); err != nil {
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}

	// 创建上下文
	ctx := req.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// 执行请求（带重试）
	return c.retryHandler.DoWithRetry(ctx, func() (*http.Response, error) {
		return c.client.Do(req)
	})
}

// prepareRequest 准备HTTP请求
func (c *httpClient) prepareRequest(req *http.Request, upstream *balance.Upstream) error {
	// 从URL解析目标地址
	if upstream.URL == "" {
		return errors.New("upstream URL cannot be empty")
	}

	// 直接使用upstream的URL
	req.URL.Scheme = "http"
	req.URL.Host = upstream.URL

	// 如果URL包含scheme，保持原有设置
	if len(upstream.URL) > 8 && upstream.URL[:8] == "https://" {
		req.URL.Scheme = "https"
		req.URL.Host = upstream.URL[8:]
	} else if len(upstream.URL) > 7 && upstream.URL[:7] == "http://" {
		req.URL.Scheme = "http"
		req.URL.Host = upstream.URL[7:]
	}

	// 应用认证（如果配置中有）
	if upstream.Config != nil && upstream.Config.Auth != nil {
		authenticator, err := c.authFactory.Create(upstream.Config.Auth)
		if err != nil {
			return fmt.Errorf("failed to create authenticator: %w", err)
		}
		if err := authenticator.Apply(req); err != nil {
			return fmt.Errorf("failed to apply authentication: %w", err)
		}
	}

	// 应用头部操作（如果配置中有）
	if upstream.Config != nil && len(upstream.Config.Headers) > 0 {
		if err := c.headerOperator.Process(req.Header, upstream.Config.Headers); err != nil {
			return fmt.Errorf("failed to process headers: %w", err)
		}
	}

	// 设置默认头部
	c.setDefaultHeaders(req)

	return nil
}

// setDefaultHeaders 设置默认HTTP头部
func (c *httpClient) setDefaultHeaders(req *http.Request) {
	// 设置User-Agent
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "LLMProxy/1.0")
	}

	// 设置Connection头部
	// 如果KeepAlive为0，表示禁用Keep-Alive
	if c.config.KeepAlive == 0 {
		req.Header.Set("Connection", "close")
	} else {
		req.Header.Set("Connection", "keep-alive")
	}

	// 保持原始Host头部用于代理
	if req.Header.Get("X-Forwarded-Host") == "" && req.Host != "" {
		req.Header.Set("X-Forwarded-Host", req.Host)
	}
}

// Close 关闭客户端并清理资源
func (c *httpClient) Close() error {
	if c.closed {
		return nil
	}

	c.closed = true

	// 关闭连接池
	if c.pool != nil {
		return c.pool.Close()
	}

	return nil
}

// Name 获取客户端名称
func (c *httpClient) Name() string {
	return c.name
}
