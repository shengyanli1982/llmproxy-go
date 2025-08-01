package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/shengyanli1982/llmproxy-go/internal/auth"
	"github.com/shengyanli1982/llmproxy-go/internal/balance"
	"github.com/shengyanli1982/llmproxy-go/internal/breaker"
	"github.com/shengyanli1982/llmproxy-go/internal/client"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/headers"
	"github.com/shengyanli1982/llmproxy-go/internal/metrics"
	"github.com/shengyanli1982/llmproxy-go/internal/ratelimit"
	"github.com/shengyanli1982/llmproxy-go/internal/response"
)

const (
	// MaxRequestBodySize 定义请求体的最大大小（64MB）
	// 防止过大的请求体导致内存耗尽
	MaxRequestBodySize = 64 << 20 // 64MB
)

// 流式传输缓冲区对象池，减少频繁的内存分配
var streamingBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 4096)
	},
}

// 非流式传输缓冲区对象池，减少频繁的内存分配
var nonStreamingBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024)
	},
}

// ForwardService 代表转发服务，处理客户端请求转发逻辑
type ForwardService struct {
	mu           sync.RWMutex          // 读写锁，保护并发访问
	config       *config.ForwardConfig // 转发服务配置
	globalConfig *config.Config        // 全局配置
	logger       *logr.Logger          // 日志记录器

	// 功能模块
	loadBalancer     balance.LoadBalancer           // 负载均衡器
	httpClient       client.HTTPClient              // HTTP客户端
	rateLimitMW      *ratelimit.RateLimitMiddleware // 限流中间件
	authFactory      auth.AuthenticatorFactory      // 认证工厂
	headerOperator   headers.HeaderOperator         // 头部操作器
	breakerFactory   breaker.CircuitBreakerFactory  // 熔断器工厂
	metricsCollector metrics.MetricsCollector       // 指标收集器

	// 运行时数据
	upstreams   []balance.Upstream                // 上游服务列表
	upstreamMap map[string]*config.UpstreamConfig // 上游配置映射

	// 状态控制
	running bool          // 运行状态
	stopCh  chan struct{} // 停止信号
}

// NewForwardServices 创建新的转发服务实例
func NewForwardServices() *ForwardService {
	logger := logr.Discard() // 临时使用，后续会被重新设置

	return &ForwardService{
		logger:         &logger,
		authFactory:    auth.NewFactory(),
		headerOperator: headers.NewOperator(),
		breakerFactory: breaker.NewFactory(),
		upstreamMap:    make(map[string]*config.UpstreamConfig),
		stopCh:         make(chan struct{}),
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

	// 初始化负载均衡器熔断器
	if err := s.initializeCircuitBreakers(); err != nil {
		return fmt.Errorf("failed to initialize circuit breakers: %w", err)
	}

	// 初始化指标收集器
	if err := s.initializeMetricsCollector(); err != nil {
		return fmt.Errorf("failed to initialize metrics collector: %w", err)
	}

	return nil
}

// buildUpstreams 构建上游服务列表
func (s *ForwardService) buildUpstreams(group *config.UpstreamGroupConfig, globalConfig *config.Config) error {
	// 预分配map容量，减少rehash操作
	upstreamConfigMap := make(map[string]*config.UpstreamConfig, len(globalConfig.Upstreams))
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

		// 创建认证器
		authenticator, err := auth.CreateFromConfig(upstreamConfig)
		if err != nil {
			return fmt.Errorf("failed to create authenticator for %s: %w", upstreamConfig.Name, err)
		}

		// 创建熔断器
		var breakerInstance breaker.CircuitBreaker
		if upstreamConfig.Breaker != nil {
			settings := breaker.CreateFromConfig(upstreamConfig.Name, upstreamConfig.Breaker)
			breakerInstance, err = s.breakerFactory.Create(upstreamConfig.Name, settings)
			if err != nil {
				return fmt.Errorf("failed to create breaker for %s: %w", upstreamConfig.Name, err)
			}
		}

		// 创建限流器
		var rateLimiterInstance *ratelimit.UpstreamLimiter
		if upstreamConfig.RateLimit != nil {
			rateLimiterInstance = ratelimit.NewUpstreamLimiter(
				float64(upstreamConfig.RateLimit.PerSecond),
				upstreamConfig.RateLimit.Burst)
		}

		upstream := balance.Upstream{
			Name:          upstreamConfig.Name,
			URL:           upstreamConfig.URL,
			Weight:        weight,
			Config:        upstreamConfig,
			Authenticator: authenticator,
			Breaker:       breakerInstance,
			RateLimiter:   rateLimiterInstance,
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
	var clientConfig *config.HTTPClientConfig

	if group.HTTPClient != nil {
		// 如果组配置中有HTTP客户端配置，使用它
		clientConfig = group.HTTPClient
	} else {
		// 否则使用默认配置
		clientConfig = &config.HTTPClientConfig{
			Agent:     "LLMProxy/1.0",
			KeepAlive: 60, // 默认60秒
		}
	}

	httpClient, err := factory.Create(clientConfig)
	if err != nil {
		return err
	}

	s.httpClient = httpClient
	return nil
}

// initializeCircuitBreakers 初始化负载均衡器中的熔断器
func (s *ForwardService) initializeCircuitBreakers() error {
	for _, upstream := range s.upstreams {
		if upstream.Breaker != nil {
			// 如果负载均衡器支持熔断器，设置熔断器
			if lbWithBreaker, ok := s.loadBalancer.(balance.LoadBalancerWithBreaker); ok {
				settings := breaker.CreateFromConfig(upstream.Name, upstream.Config.Breaker)
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
	g.POST("/*path", s.handleForward).
		GET("/*path", s.handleForward)
}

// ginRateLimitMiddleware 将orbit限流中间件转换为gin中间件
func (s *ForwardService) ginRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查限流中间件是否启用
		if s.rateLimitMW == nil || !s.rateLimitMW.IsEnabled() {
			c.Next()
			return
		}

		// 执行IP级别的限流检查
		if !s.rateLimitMW.AllowRequest(c.Request) {
			clientIP := s.getClientIP(c.Request)
			s.logger.V(1).Info("Rate limit exceeded for IP", "ip", clientIP)
			detail := map[string]interface{}{
				"code": "RATE_LIMIT_EXCEEDED",
				"ip":   clientIP,
			}
			response.Error(response.CodeRateLimit, "too many requests from this IP").
				WithDetail(detail).
				JSON(c, http.StatusTooManyRequests)
			c.Abort()
			return
		}

		// 如果有上游信息，也进行上游级别的限流检查
		// 注意：这里我们还没有选择上游，所以暂时跳过上游限流
		// 上游限流会在选择上游后在 processRequest 中进行

		c.Next()
	}
}

// handleForward 处理转发请求
func (s *ForwardService) handleForward(c *gin.Context) {
	startTime := time.Now()

	// 记录请求开始
	if s.metricsCollector != nil {
		s.metricsCollector.RecordRequest(s.config.Name, c.Request.Method, c.Request.URL.Path)
	}

	// 处理请求，如果有错误，直接返回错误响应
	if err := s.processRequest(c, startTime); err != nil {
		s.logger.Error(err, "Request processing failed",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"client_ip", c.ClientIP())

		// 记录错误
		if s.metricsCollector != nil {
			s.metricsCollector.RecordError(s.config.Name, "processing_error")
		}

		s.sendErrorResponse(c, http.StatusInternalServerError, "Internal server error")
	}
}

// processRequest 处理请求的核心逻辑
func (s *ForwardService) processRequest(c *gin.Context, startTime time.Time) error {
	req := c.Request
	ctx := req.Context()

	// 1. 选择上游服务
	upstream, err := s.loadBalancer.Select(ctx, s.upstreams)
	if err != nil {
		s.logger.Error(err, "Failed to select upstream")

		// 记录上游错误
		if s.metricsCollector != nil {
			s.metricsCollector.RecordUpstreamError(s.config.DefaultGroup, "unknown", "selection_failed")
		}

		s.sendErrorResponse(c, http.StatusServiceUnavailable, "No available upstream")
		return fmt.Errorf("failed to select upstream: %w", err)
	}

	// 2. 检查上游级别的限流
	if !upstream.CheckRateLimit() {
		s.logger.V(1).Info("Rate limit exceeded for upstream", "upstream", upstream.Name)

		// 记录限流拒绝
		if s.metricsCollector != nil {
			s.metricsCollector.RecordRateLimitRejection(s.config.Name, "upstream")
		}

		s.sendErrorResponse(c, http.StatusTooManyRequests, "Too many requests to upstream service")
		return fmt.Errorf("rate limit exceeded for upstream: %s", upstream.Name)
	}

	// 3. 创建请求副本
	proxyReq, err := s.createProxyRequest(req)
	if err != nil {
		s.logger.Error(err, "Failed to create proxy request")
		s.sendErrorResponse(c, http.StatusInternalServerError, "Failed to create proxy request")
		return fmt.Errorf("failed to create proxy request: %w", err)
	}

	// 4. 执行请求（通过Upstream封装的熔断器保护）
	resp, err := upstream.ExecuteWithBreaker(func() (*http.Response, error) {
		return s.httpClient.Do(proxyReq, &upstream)
	})
	if err != nil {
		s.logger.Error(err, "Request execution failed", "upstream", upstream.Name)

		// 记录上游错误
		if s.metricsCollector != nil {
			s.metricsCollector.RecordUpstreamError(s.config.DefaultGroup, upstream.Name, "execution_error")
		}

		s.sendErrorResponse(c, http.StatusServiceUnavailable, "Upstream service unavailable")
		return fmt.Errorf("request execution failed for upstream %s: %w", upstream.Name, err)
	}

	defer resp.Body.Close()

	// 6. 计算响应时间并更新负载均衡器
	duration := time.Since(startTime)
	latency := duration.Milliseconds()
	s.loadBalancer.UpdateLatency(upstream.Name, latency)

	// 7. 转发响应
	s.forwardResponse(c, resp)

	// 8. 记录指标
	if s.metricsCollector != nil {
		// 获取请求和响应大小
		requestSize := s.getRequestSize(req)
		responseSize := s.getResponseSize(resp)

		// 记录 HTTP 响应指标
		s.metricsCollector.RecordResponse(
			s.config.Name,
			req.Method,
			req.URL.Path,
			resp.StatusCode,
			duration,
			requestSize,
			responseSize,
		)

		// 记录上游响应指标
		s.metricsCollector.RecordUpstreamResponse(
			s.config.DefaultGroup,
			upstream.Name,
			req.Method,
			resp.StatusCode,
			duration,
		)

		// 记录负载均衡器选择
		s.metricsCollector.RecordLoadBalancerSelection(
			s.config.DefaultGroup,
			upstream.Name,
			s.loadBalancer.Type(),
		)
	}

	// 9. 记录访问日志
	s.logger.Info("Request forwarded successfully",
		"method", req.Method,
		"path", req.URL.Path,
		"upstream", upstream.Name,
		"status", resp.StatusCode,
		"latency_ms", latency)

	return nil
}

// createProxyRequest 创建代理请求
func (s *ForwardService) createProxyRequest(originalReq *http.Request) (*http.Request, error) {
	var proxyBody io.Reader

	// 处理请求体
	if originalReq.Body != nil {
		// 确保原始请求体在函数结束时被关闭
		defer func() {
			if closeErr := originalReq.Body.Close(); closeErr != nil {
				s.logger.V(1).Info("Failed to close original request body", "error", closeErr)
			}
		}()

		// 使用 LimitReader 限制读取大小，防止内存耗尽
		limitedReader := io.LimitReader(originalReq.Body, MaxRequestBodySize+1)

		// 读取请求体内容到内存
		bodyBytes, err := io.ReadAll(limitedReader)
		if err != nil {
			s.logger.Error(err, "Failed to read request body")
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}

		// 检查是否超过大小限制
		if len(bodyBytes) > MaxRequestBodySize {
			s.logger.V(1).Info("Request body too large", "size", len(bodyBytes), "limit", MaxRequestBodySize)
			return nil, fmt.Errorf("request body too large: %d bytes (limit: %d bytes)", len(bodyBytes), MaxRequestBodySize)
		}

		// 创建新的可读取的请求体
		if len(bodyBytes) > 0 {
			proxyBody = bytes.NewReader(bodyBytes)
			s.logger.V(2).Info("Request body copied", "size", len(bodyBytes))
		}
	}

	// 创建新的代理请求
	proxyReq, err := http.NewRequestWithContext(
		originalReq.Context(),
		originalReq.Method,
		originalReq.URL.String(), // URL会在httpClient中被重写
		proxyBody,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy request: %w", err)
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
	// 从对象池获取缓冲区
	buffer := streamingBufferPool.Get()
	defer streamingBufferPool.Put(buffer)
	bufSlice := buffer.([]byte) // 使用完整的缓冲区，不截断为0长度

	// 流式复制响应体
	for {
		n, err := resp.Body.Read(bufSlice)
		if n > 0 {
			if _, writeErr := c.Writer.Write(bufSlice[:n]); writeErr != nil {
				s.logger.Error(writeErr, "Failed to write streaming response")
				break
			}
			// 移除 Flush() 调用以避免 orbit 框架的双写问题
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
	// 从对象池获取缓冲区
	buffer := nonStreamingBufferPool.Get()
	defer nonStreamingBufferPool.Put(buffer)
	bufSlice := buffer.([]byte) // 使用完整的缓冲区，不截断为0长度

	// 直接复制响应体
	if _, err := io.CopyBuffer(c.Writer, resp.Body, bufSlice); err != nil {
		s.logger.Error(err, "Failed to copy response body")
	}
}

// sendErrorResponse 发送错误响应
func (s *ForwardService) sendErrorResponse(c *gin.Context, statusCode int, message string) {
	var code int64
	switch statusCode {
	case http.StatusTooManyRequests:
		code = response.CodeRateLimit
	case http.StatusServiceUnavailable:
		code = response.CodeServiceUnavailable
	case http.StatusBadGateway:
		code = response.CodeBadGateway
	case http.StatusGatewayTimeout:
		code = response.CodeGatewayTimeout
	case http.StatusBadRequest:
		code = response.CodeBadRequest
	case http.StatusUnauthorized:
		code = response.CodeUnauthorized
	case http.StatusForbidden:
		code = response.CodeForbidden
	case http.StatusNotFound:
		code = response.CodeNotFound
	default:
		code = response.CodeInternalError
	}

	detail := map[string]interface{}{
		"error":     http.StatusText(statusCode),
		"timestamp": time.Now().Unix(),
	}

	response.Error(code, message).WithDetail(detail).JSON(c, statusCode)
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

	// 安全关闭channel
	select {
	case <-s.stopCh:
		// channel已经关闭
	default:
		close(s.stopCh)
	}

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

// initializeMetricsCollector 初始化指标收集器
func (s *ForwardService) initializeMetricsCollector() error {
	// 使用全局 MetricsRegistry 获取或创建唯一的共享收集器
	globalRegistry := metrics.GetGlobalRegistry()

	// 使用固定名称 "global" 确保所有服务共享同一个收集器
	const globalCollectorName = "global"

	// 如果收集器已经存在，直接使用
	if existingCollector, exists := globalRegistry.GetCollector(globalCollectorName); exists {
		s.metricsCollector = existingCollector
		s.logger.V(1).Info("Reusing global metrics collector", "service", s.config.Name)
		return nil
	}

	// 创建全局共享收集器配置
	config := &metrics.Config{
		Type:      "prometheus",
		Enabled:   true,
		Namespace: "llmproxy",
		Subsystem: "",
	}

	// 创建新的全局共享收集器
	collector, err := globalRegistry.CreateSharedCollector(globalCollectorName, config)
	if err != nil {
		return fmt.Errorf("failed to create global metrics collector: %w", err)
	}

	s.metricsCollector = collector
	s.logger.Info("Created global metrics collector", "service", s.config.Name)
	return nil
}

// getRequestSize 获取请求体大小
func (s *ForwardService) getRequestSize(req *http.Request) int64 {
	if req.ContentLength > 0 {
		return req.ContentLength
	}
	return 0
}

// getResponseSize 获取响应体大小
func (s *ForwardService) getResponseSize(resp *http.Response) int64 {
	if resp.ContentLength > 0 {
		return resp.ContentLength
	}
	return 0
}
