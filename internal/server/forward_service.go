package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	gotrycatch "github.com/shengyanli1982/go-trycatch"
	"github.com/shengyanli1982/llmproxy-go/internal/auth"
	"github.com/shengyanli1982/llmproxy-go/internal/balance"
	"github.com/shengyanli1982/llmproxy-go/internal/breaker"
	"github.com/shengyanli1982/llmproxy-go/internal/client"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/headers"
	"github.com/shengyanli1982/llmproxy-go/internal/ratelimit"
	"github.com/sony/gobreaker"
)

// ForwardService 代表转发服务，处理客户端请求转发逻辑
type ForwardService struct {
	mu               sync.RWMutex                    // 读写锁，保护并发访问
	config           *config.ForwardConfig           // 转发服务配置
	globalConfig     *config.Config                  // 全局配置
	logger           *logr.Logger                    // 日志记录器
	
	// 功能模块
	loadBalancer     balance.LoadBalancer            // 负载均衡器
	httpClient       client.HTTPClient               // HTTP客户端
	rateLimitMW      *ratelimit.RateLimitMiddleware  // 限流中间件
	authFactory      auth.AuthenticatorFactory       // 认证工厂
	headerOperator   headers.HeaderOperator          // 头部操作器
	breakerFactory   breaker.CircuitBreakerFactory   // 熔断器工厂
	
	// 运行时数据
	upstreams        []balance.Upstream              // 上游服务列表
	upstreamMap      map[string]*config.UpstreamConfig // 上游配置映射
	circuitBreakers  map[string]breaker.CircuitBreaker // 熔断器映射
	
	// 状态控制
	running          bool                            // 运行状态
	stopCh           chan struct{}                   // 停止信号
}

// NewForwardServices 创建新的转发服务实例
func NewForwardServices() *ForwardService {
	logger := logr.Discard() // 临时使用，后续会被重新设置
	
	return &ForwardService{
		logger:          &logger,
		authFactory:     auth.NewFactory(),
		headerOperator:  headers.NewOperator(),
		breakerFactory:  breaker.NewFactory(),
		upstreamMap:     make(map[string]*config.UpstreamConfig),
		circuitBreakers: make(map[string]breaker.CircuitBreaker),
		stopCh:          make(chan struct{}),
	}
}

// Initialize 初始化转发服务
func (s *ForwardService) Initialize(cfg *config.ForwardConfig, globalConfig *config.Config, logger *logr.Logger) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.config = cfg
	s.globalConfig = globalConfig
	s.logger = logger
	
	// 初始化限流中间件
	if cfg.RateLimit != nil {
		s.rateLimitMW = ratelimit.NewRateLimitMiddleware(
			float64(cfg.RateLimit.PerSecond), cfg.RateLimit.Burst,
			float64(cfg.RateLimit.PerSecond), cfg.RateLimit.Burst,
		)
	}
	
	// 查找默认上游组
	var defaultGroup *config.UpstreamGroupConfig
	for _, group := range globalConfig.UpstreamGroups {
		if group.Name == cfg.DefaultGroup {
			defaultGroup = &group
			break
		}
	}
	
	if defaultGroup == nil {
		return fmt.Errorf("default upstream group '%s' not found", cfg.DefaultGroup)
	}
	
	// 构建上游服务列表
	if err := s.buildUpstreams(defaultGroup, globalConfig); err != nil {
		return fmt.Errorf("failed to build upstreams: %w", err)
	}
	
	// 创建负载均衡器
	if err := s.createLoadBalancer(defaultGroup); err != nil {
		return fmt.Errorf("failed to create load balancer: %w", err)
	}
	
	// 创建HTTP客户端
	if err := s.createHttpClient(defaultGroup); err != nil {
		return fmt.Errorf("failed to create http client: %w", err)
	}
	
	// 初始化熔断器
	if err := s.initializeCircuitBreakers(); err != nil {
		return fmt.Errorf("failed to initialize circuit breakers: %w", err)
	}
	
	return nil
}

// buildUpstreams 构建上游服务列表
func (s *ForwardService) buildUpstreams(group *config.UpstreamGroupConfig, globalConfig *config.Config) error {
	upstreamConfigMap := make(map[string]*config.UpstreamConfig)
	for i := range globalConfig.Upstreams {
		upstreamConfigMap[globalConfig.Upstreams[i].Name] = &globalConfig.Upstreams[i]
	}
	
	s.upstreams = make([]balance.Upstream, 0, len(group.Upstreams))
	
	for _, upstreamRef := range group.Upstreams {
		upstreamConfig, exists := upstreamConfigMap[upstreamRef.Name]
		if !exists {
			return fmt.Errorf("upstream '%s' not found in configuration", upstreamRef.Name)
		}
		
		weight := upstreamRef.Weight
		if weight <= 0 {
			weight = 1 // 默认权重
		}
		
		upstream := balance.Upstream{
			Name:   upstreamConfig.Name,
			URL:    upstreamConfig.URL,
			Weight: weight,
			Config: upstreamConfig,
		}
		
		s.upstreams = append(s.upstreams, upstream)
		s.upstreamMap[upstreamConfig.Name] = upstreamConfig
	}
	
	return nil
}

// createLoadBalancer 创建负载均衡器
func (s *ForwardService) createLoadBalancer(group *config.UpstreamGroupConfig) error {
	factory := balance.NewFactory()
	
	var balanceConfig *config.BalanceConfig
	if group.Balance != nil {
		balanceConfig = group.Balance
	} else {
		balanceConfig = &config.BalanceConfig{Strategy: "roundrobin"}
	}
	
	lb, err := factory.Create(balanceConfig)
	if err != nil {
		return err
	}
	
	s.loadBalancer = lb
	return nil
}

// createHttpClient 创建HTTP客户端
func (s *ForwardService) createHttpClient(group *config.UpstreamGroupConfig) error {
	factory := client.NewFactory()
	
	// 构建客户端配置
	clientConfig := client.DefaultConfig()
	
	if group.HTTPClient != nil {
		if group.HTTPClient.KeepAlive > 0 {
			clientConfig.IdleConnTimeout = group.HTTPClient.KeepAlive
		}
		if group.HTTPClient.Timeout != nil {
			if group.HTTPClient.Timeout.Connect > 0 {
				clientConfig.ConnectTimeout = group.HTTPClient.Timeout.Connect
			}
			if group.HTTPClient.Timeout.Request > 0 {
				clientConfig.RequestTimeout = group.HTTPClient.Timeout.Request
			}
		}
		if group.HTTPClient.Retry != nil {
			clientConfig.EnableRetry = true
			clientConfig.MaxRetries = group.HTTPClient.Retry.Attempts
			clientConfig.RetryDelay = group.HTTPClient.Retry.Initial
		}
		if group.HTTPClient.Proxy != nil {
			clientConfig.ProxyURL = group.HTTPClient.Proxy.URL
		}
	}
	
	httpClient, err := factory.Create(clientConfig)
	if err != nil {
		return err
	}
	
	s.httpClient = httpClient
	return nil
}

// initializeCircuitBreakers 初始化熔断器
func (s *ForwardService) initializeCircuitBreakers() error {
	for _, upstream := range s.upstreams {
		if upstream.Config.Breaker != nil {
			settings := breaker.CreateFromConfig(
				upstream.Name,
				3,  // maxRequests
				10*time.Second, // interval  
				time.Duration(upstream.Config.Breaker.Cooldown)*time.Second, // timeout
				upstream.Config.Breaker.Threshold, // failureThreshold
				10, // minRequests
			)
			
			cb, err := s.breakerFactory.Create(upstream.Name, settings)
			if err != nil {
				return fmt.Errorf("failed to create circuit breaker for upstream '%s': %w", upstream.Name, err)
			}
			
			s.circuitBreakers[upstream.Name] = cb
			
			// 如果负载均衡器支持熔断器，也要设置
			if lbWithBreaker, ok := s.loadBalancer.(balance.LoadBalancerWithBreaker); ok {
				if err := lbWithBreaker.CreateBreaker(upstream.Name, settings); err != nil {
					s.logger.Error(err, "Failed to create breaker in load balancer", "upstream", upstream.Name)
				}
			}
		}
	}
	
	return nil
}

// RegisterGroup 实现orbit.Service接口，注册到orbit引擎
func (s *ForwardService) RegisterGroup(g *gin.RouterGroup) {
	// 注册限流中间件
	if s.rateLimitMW != nil {
		// 将orbit中间件转换为gin中间件
		g.Use(s.ginRateLimitMiddleware())
	}
	
	// 注册转发处理器，处理所有请求
	g.Any("/*path", s.handleForward)
}

// ginRateLimitMiddleware 将orbit限流中间件转换为gin中间件
func (s *ForwardService) ginRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 简单的IP限流检查
		if s.rateLimitMW != nil && s.rateLimitMW.IsEnabled() {
			// 这里简化处理，实际可以通过rateLimitMW进行限流检查
			// 目前先跳过具体的限流逻辑实现
		}
		c.Next()
	}
}

// handleForward 处理转发请求
func (s *ForwardService) handleForward(c *gin.Context) {
	startTime := time.Now()
	
	// 使用go-trycatch进行错误处理
	gotrycatch.New().
		Try(func() error {
			s.processRequest(c, startTime)
			return nil
		}).
		Catch(func(err error) {
			s.logger.Error(err, "Request processing failed", 
				"method", c.Request.Method, 
				"path", c.Request.URL.Path,
				"client_ip", c.ClientIP())
			
			s.sendErrorResponse(c, http.StatusInternalServerError, "Internal server error")
		}).
		Do()
}

// processRequest 处理请求的核心逻辑
func (s *ForwardService) processRequest(c *gin.Context, startTime time.Time) {
	req := c.Request
	ctx := req.Context()
	
	// 1. 选择上游服务
	upstream, err := s.loadBalancer.Select(ctx, s.upstreams)
	if err != nil {
		s.logger.Error(err, "Failed to select upstream")
		s.sendErrorResponse(c, http.StatusServiceUnavailable, "No available upstream")
		return
	}
	
	// 2. 检查熔断器状态
	if cb, exists := s.circuitBreakers[upstream.Name]; exists {
		if cb.State() == gobreaker.StateOpen {
			s.sendErrorResponse(c, http.StatusServiceUnavailable, "Service temporarily unavailable")
			return
		}
	}
	
	// 3. 创建请求副本
	proxyReq, err := s.createProxyRequest(req, upstream)
	if err != nil {
		s.logger.Error(err, "Failed to create proxy request")
		s.sendErrorResponse(c, http.StatusInternalServerError, "Failed to create proxy request")
		return
	}
	
	// 4. 执行请求（通过熔断器保护）
	var resp *http.Response
	if cb, exists := s.circuitBreakers[upstream.Name]; exists {
		result, err := cb.Execute(func() (interface{}, error) {
			return s.httpClient.Do(proxyReq, &upstream)
		})
		if err != nil {
			s.logger.Error(err, "Circuit breaker execution failed", "upstream", upstream.Name)
			s.sendErrorResponse(c, http.StatusServiceUnavailable, "Upstream service unavailable")
			return
		}
		resp = result.(*http.Response)
	} else {
		resp, err = s.httpClient.Do(proxyReq, &upstream)
		if err != nil {
			s.logger.Error(err, "HTTP request failed", "upstream", upstream.Name)
			s.sendErrorResponse(c, http.StatusBadGateway, "Upstream request failed")
			return
		}
	}
	
	defer resp.Body.Close()
	
	// 5. 计算响应时间并更新负载均衡器
	latency := time.Since(startTime).Milliseconds()
	s.loadBalancer.UpdateLatency(upstream.Name, latency)
	
	// 6. 转发响应
	s.forwardResponse(c, resp)
	
	// 7. 记录访问日志
	s.logger.Info("Request forwarded successfully",
		"method", req.Method,
		"path", req.URL.Path,
		"upstream", upstream.Name,
		"status", resp.StatusCode,
		"latency_ms", latency)
}

// createProxyRequest 创建代理请求
func (s *ForwardService) createProxyRequest(originalReq *http.Request, upstream balance.Upstream) (*http.Request, error) {
	// 创建新的请求
	body := originalReq.Body
	if body != nil {
		// 对于有body的请求，需要复制body内容
		// 这里简化处理，实际项目中可能需要更复杂的body处理
	}
	
	proxyReq, err := http.NewRequestWithContext(
		originalReq.Context(),
		originalReq.Method,
		originalReq.URL.String(), // URL会在httpClient中被重写
		body,
	)
	if err != nil {
		return nil, err
	}
	
	// 复制原始请求的头部
	for name, values := range originalReq.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}
	
	// 设置代理相关头部
	proxyReq.Header.Set("X-Forwarded-For", s.getClientIP(originalReq))
	proxyReq.Header.Set("X-Forwarded-Proto", s.getScheme(originalReq))
	proxyReq.Header.Set("X-Forwarded-Host", originalReq.Host)
	
	return proxyReq, nil
}

// forwardResponse 转发响应
func (s *ForwardService) forwardResponse(c *gin.Context, resp *http.Response) {
	// 复制响应头部
	for name, values := range resp.Header {
		for _, value := range values {
			c.Header(name, value)
		}
	}
	
	// 设置状态码
	c.Status(resp.StatusCode)
	
	// 判断是否为流式响应
	if s.isStreamingResponse(resp) {
		s.forwardStreamingResponse(c, resp)
	} else {
		s.forwardRegularResponse(c, resp)
	}
}

// isStreamingResponse 判断是否为流式响应
func (s *ForwardService) isStreamingResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "text/event-stream") ||
		   strings.Contains(contentType, "application/stream+json") ||
		   resp.Header.Get("Transfer-Encoding") == "chunked"
}

// forwardStreamingResponse 转发流式响应
func (s *ForwardService) forwardStreamingResponse(c *gin.Context, resp *http.Response) {
	// 确保响应支持流式传输
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	
	// 流式复制响应体
	buffer := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := c.Writer.Write(buffer[:n]); writeErr != nil {
				s.logger.Error(writeErr, "Failed to write streaming response")
				break
			}
			c.Writer.Flush()
		}
		if err != nil {
			if err != io.EOF {
				s.logger.Error(err, "Error reading streaming response")
			}
			break
		}
	}
}

// forwardRegularResponse 转发常规响应
func (s *ForwardService) forwardRegularResponse(c *gin.Context, resp *http.Response) {
	// 直接复制响应体
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		s.logger.Error(err, "Failed to copy response body")
	}
}

// sendErrorResponse 发送错误响应
func (s *ForwardService) sendErrorResponse(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"error":     http.StatusText(statusCode),
		"message":   message,
		"timestamp": time.Now().Unix(),
	})
}

// getClientIP 获取客户端IP
func (s *ForwardService) getClientIP(req *http.Request) string {
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	
	if idx := strings.LastIndex(req.RemoteAddr, ":"); idx >= 0 {
		return req.RemoteAddr[:idx]
	}
	
	return req.RemoteAddr
}

// getScheme 获取请求协议
func (s *ForwardService) getScheme(req *http.Request) string {
	if req.TLS != nil {
		return "https"
	}
	if scheme := req.Header.Get("X-Forwarded-Proto"); scheme != "" {
		return scheme
	}
	return "http"
}

// Run 启动转发服务
func (s *ForwardService) Run() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.running {
		return
	}
	
	s.running = true
	s.logger.Info("Forward service started")
}

// Stop 停止转发服务
func (s *ForwardService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.running {
		return
	}
	
	s.running = false
	close(s.stopCh)
	
	// 清理资源
	if s.httpClient != nil {
		s.httpClient.Close()
	}
	
	s.logger.Info("Forward service stopped")
}

// IsRunning 检查服务是否运行中
func (s *ForwardService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}