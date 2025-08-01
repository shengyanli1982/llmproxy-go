package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-logr/logr"
	"github.com/shengyanli1982/llmproxy-go/internal/auth"
	"github.com/shengyanli1982/llmproxy-go/internal/balance"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/constants"
	"github.com/shengyanli1982/llmproxy-go/internal/headers"
)

// 客户端相关错误定义
var (
	ErrNilRequest     = errors.New(constants.ErrMsgNilRequest)
	ErrNilUpstream    = errors.New(constants.ErrMsgNilUpstream)
	ErrClientClosed   = errors.New(constants.ErrMsgClientClosed)
	ErrInvalidTimeout = errors.New(constants.ErrMsgInvalidTimeout)
)

// httpClient HTTP客户端实现
type httpClient struct {
	name         string
	client       *http.Client
	pool         *ConnectionPool
	proxyHandler *ProxyHandler
	config       *config.HTTPClientConfig
	closed       bool

	// 依赖的模块
	authFactory    auth.AuthenticatorFactory
	headerOperator headers.HeaderOperator

	// 日志记录器（可选）
	logger logr.Logger
}

// NewHTTPClient 创建新的HTTP客户端实例
func NewHTTPClient(cfg *config.HTTPClientConfig) (HTTPClient, error) {
	// 创建连接池
	pool := NewConnectionPool(cfg)

	// 创建代理处理器
	var proxyHandler *ProxyHandler
	if cfg.Proxy != nil {
		proxyHandler = NewProxyHandler(cfg.Proxy)
	} else {
		proxyHandler = NewProxyHandlerFromURL("")
	}

	// 获取请求超时时间
	requestTimeout := constants.DefaultRequestTimeout // 默认60000毫秒
	if cfg.Timeout != nil {
		requestTimeout = cfg.Timeout.Request
	}

	// 创建HTTP客户端
	client := &http.Client{
		Transport: pool.GetTransport(),
		Timeout:   time.Duration(requestTimeout) * time.Millisecond,
	}

	// 设置代理
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.Proxy = proxyHandler.GetProxyFunc()
	}

	return &httpClient{
		name:           fmt.Sprintf("http-client-%d", time.Now().UnixNano()),
		client:         client,
		pool:           pool,
		proxyHandler:   proxyHandler,
		config:         cfg,
		closed:         false,
		authFactory:    auth.NewFactory(),
		headerOperator: headers.NewOperator(),
		logger:         logr.Discard(), // 默认使用丢弃日志记录器
	}, nil
}

// Do 执行HTTP请求到指定上游服务
func (c *httpClient) Do(req *http.Request, upstream *balance.Upstream) (*http.Response, error) {
	if c.closed {
		c.logger.Error(ErrClientClosed, "Client is closed, cannot execute request")
		return nil, ErrClientClosed
	}
	if req == nil {
		c.logger.Error(ErrNilRequest, "Request cannot be nil")
		return nil, ErrNilRequest
	}
	if upstream == nil {
		c.logger.Error(ErrNilUpstream, "Upstream cannot be nil")
		return nil, ErrNilUpstream
	}

	// 准备请求
	c.logger.Info("Preparing HTTP request",
		"method", req.Method,
		"path", req.URL.Path,
		"upstream", upstream.Name,
		"client_name", c.name)

	startTime := time.Now()
	if err := c.prepareRequest(req, upstream); err != nil {
		c.logger.Error(err, "Failed to prepare request",
			"upstream", upstream.Name,
			"duration_ms", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}

	c.logger.Info("HTTP request prepared and executing",
		"method", req.Method,
		"target_url", req.URL.String(),
		"upstream", upstream.Name,
		"preparation_duration_ms", time.Since(startTime).Milliseconds())

	// 执行请求
	execStartTime := time.Now()
	resp, err := c.client.Do(req)
	execDuration := time.Since(execStartTime)

	if err != nil {
		c.logger.Error(err, "HTTP request execution failed",
			"upstream", upstream.Name,
			"target_url", req.URL.String(),
			"execution_duration_ms", execDuration.Milliseconds())
		return nil, err
	}

	c.logger.Info("HTTP request executed successfully",
		"upstream", upstream.Name,
		"status_code", resp.StatusCode,
		"content_length", resp.ContentLength,
		"execution_duration_ms", execDuration.Milliseconds())

	return resp, nil
}

// prepareRequest 准备HTTP请求，设置目标URL和认证信息
// 注意：此方法会修改传入的http.Request，调用者需要确保并发安全
func (c *httpClient) prepareRequest(req *http.Request, upstream *balance.Upstream) error {
	// 验证输入参数
	if req == nil {
		return ErrNilRequest
	}
	if upstream == nil {
		return ErrNilUpstream
	}
	if upstream.URL == "" {
		c.logger.Error(nil, "Upstream URL cannot be empty", "upstream_name", upstream.Name)
		return fmt.Errorf("upstream URL cannot be empty for upstream '%s'", upstream.Name)
	}

	originalURL := req.URL.String()

	// 解析upstream URL，处理不带scheme的情况
	var upstreamURL *url.URL
	var err error

	// 首先尝试直接解析
	upstreamURL, err = url.Parse(upstream.URL)
	if err != nil {
		// 如果解析失败，可能是因为缺少scheme，尝试添加http://前缀
		upstreamURL, err = url.Parse(constants.DefaultScheme + upstream.URL)
		if err != nil {
			c.logger.Error(err, "Invalid upstream URL", "upstream", upstream.Name, "url", upstream.URL)
			return fmt.Errorf("invalid upstream URL '%s': %w", upstream.URL, err)
		}
	} else if upstreamURL.Scheme == "" {
		// 如果解析成功但没有scheme，添加http://前缀重新解析
		upstreamURL, err = url.Parse(constants.DefaultScheme + upstream.URL)
		if err != nil {
			c.logger.Error(err, "Invalid upstream URL after adding scheme", "upstream", upstream.Name, "url", upstream.URL)
			return fmt.Errorf("invalid upstream URL '%s': %w", upstream.URL, err)
		}
	}

	// 验证URL必须包含host
	if upstreamURL.Host == "" {
		c.logger.Error(nil, "Upstream URL must include host", "upstream", upstream.Name, "url", upstream.URL)
		return fmt.Errorf("upstream URL must include host: %s", upstream.URL)
	}

	// 设置目标URL的scheme和host
	req.URL.Scheme = upstreamURL.Scheme
	req.URL.Host = upstreamURL.Host

	c.logger.Info("URL rewriting completed",
		"upstream", upstream.Name,
		"original_url", originalURL,
		"target_scheme", req.URL.Scheme,
		"target_host", req.URL.Host)

	// 实现URL拆分和拼接转发机制
	// 用户设计思路：用户请求URL拆分成基础URL和路径，然后与upstream URL拼接
	// 例如：用户请求 http://127.0.0.1:3000/api/v3/chat/completions
	//      拆分为：基础URL(http://127.0.0.1:3000) + 路径(/api/v3/chat/completions)
	//      upstream配置：https://ark.cn-beijing.volces.com
	//      最终转发：https://ark.cn-beijing.volces.com/api/v3/chat/completions

	if upstreamURL.Path != "" && upstreamURL.Path != "/" {
		// 如果upstream URL包含具体路径，使用upstream的路径（支持完整端点配置）
		req.URL.Path = upstreamURL.Path
		// 同时保留upstream URL的查询参数和片段
		if upstreamURL.RawQuery != "" {
			req.URL.RawQuery = upstreamURL.RawQuery
		}
		if upstreamURL.Fragment != "" {
			req.URL.Fragment = upstreamURL.Fragment
		}
		c.logger.Info("Using upstream specific path", "upstream", upstream.Name, "path", req.URL.Path)
	}
	// 注意：当upstream URL是基础URL时，我们不需要修改req.URL.Path、RawQuery、Fragment
	// 它们保持用户请求的原始值，实现了"基础URL + 用户路径"的拼接机制

	// 应用认证（使用缓存的认证器）
	if upstream.Authenticator != nil {
		c.logger.Info("Applying authentication", "upstream", upstream.Name, "auth_type", upstream.Authenticator.Type())
		if err := upstream.ApplyAuth(req); err != nil {
			c.logger.Error(err, "Failed to apply authentication", "upstream", upstream.Name)
			return fmt.Errorf("failed to apply authentication: %w", err)
		}
	} else {
		c.logger.Info("No authentication configured", "upstream", upstream.Name)
	}

	// 应用头部操作（如果配置中有）
	if upstream.Config != nil && len(upstream.Config.Headers) > 0 {
		c.logger.Info("Processing custom headers", "upstream", upstream.Name, "header_count", len(upstream.Config.Headers))
		if err := c.headerOperator.Process(req.Header, upstream.Config.Headers); err != nil {
			c.logger.Error(err, "Failed to process headers", "upstream", upstream.Name)
			return fmt.Errorf("failed to process headers: %w", err)
		}
	}

	// 设置默认头部
	c.setDefaultHeaders(req)

	c.logger.Info("Request preparation completed",
		"upstream", upstream.Name,
		"final_url", req.URL.String(),
		"user_agent", req.Header.Get("User-Agent"),
		"connection", req.Header.Get("Connection"))

	return nil
}

// setDefaultHeaders 设置默认HTTP头部
func (c *httpClient) setDefaultHeaders(req *http.Request) {
	// 设置User-Agent
	if req.Header.Get(constants.HeaderUserAgent) == "" {
		req.Header.Set(constants.HeaderUserAgent, constants.UserAgent)
	}

	// 设置Connection头部
	// 如果KeepAlive为0，表示禁用Keep-Alive
	if c.config.KeepAlive == 0 {
		req.Header.Set(constants.HeaderConnection, constants.ConnectionClose)
	} else {
		req.Header.Set(constants.HeaderConnection, constants.ConnectionKeepAlive)
	}

	// 保持原始Host头部用于代理
	if req.Header.Get(constants.HeaderXForwardedHost) == "" && req.Host != "" {
		req.Header.Set(constants.HeaderXForwardedHost, req.Host)
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

// SetLogger 设置日志记录器
func (c *httpClient) SetLogger(logger logr.Logger) {
	c.logger = logger
}
