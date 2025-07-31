package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// prometheusCollector 基于 Prometheus 的指标收集器实现
type prometheusCollector struct {
	name     string
	registry *prometheus.Registry
	config   *Config
	mu       sync.RWMutex

	// HTTP 服务器指标
	httpRequestsTotal     *prometheus.CounterVec
	httpRequestDuration   *prometheus.HistogramVec
	httpRequestSizeBytes  *prometheus.HistogramVec
	httpResponseSizeBytes *prometheus.HistogramVec

	// 上游服务指标
	upstreamRequestsTotal   *prometheus.CounterVec
	upstreamRequestDuration *prometheus.HistogramVec
	upstreamErrorsTotal     *prometheus.CounterVec

	// 断路器指标
	circuitBreakerState         *prometheus.GaugeVec
	circuitBreakerRequestsTotal *prometheus.CounterVec
	circuitBreakerStateChanges  *prometheus.CounterVec

	// 负载均衡器指标
	loadBalancerSelectionsTotal *prometheus.CounterVec
	upstreamHealthStatus        *prometheus.GaugeVec

	// 系统级指标
	activeConnections        *prometheus.GaugeVec
	rateLimitRejectionsTotal *prometheus.CounterVec
}

// NewPrometheusCollectorWithRegistry 创建使用指定注册器的 Prometheus 指标收集器实例
func NewPrometheusCollectorWithRegistry(config *Config, registry *prometheus.Registry) (MetricsCollector, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if registry == nil {
		return nil, fmt.Errorf("registry cannot be nil")
	}

	collector := &prometheusCollector{
		name:     "prometheus",
		registry: registry,
		config:   config,
	}

	if err := collector.initMetrics(); err != nil {
		return nil, err
	}

	return collector, nil
}

// initMetrics 初始化所有 Prometheus 指标
func (c *prometheusCollector) initMetrics() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 构建指标名称前缀
	prefix := c.config.Namespace
	if c.config.Subsystem != "" {
		prefix = c.config.Namespace + "_" + c.config.Subsystem
	}

	// HTTP 服务器指标
	c.httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"forward_name", "method", "path", "status_code"},
	)

	c.httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    prefix + "_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
		},
		[]string{"forward_name", "method", "path"},
	)

	c.httpRequestSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    prefix + "_http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8), // 100B to ~100MB
		},
		[]string{"forward_name", "method", "path"},
	)

	c.httpResponseSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    prefix + "_http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8), // 100B to ~100MB
		},
		[]string{"forward_name", "method", "path", "status_code"},
	)

	// 上游服务指标
	c.upstreamRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_upstream_requests_total",
			Help: "Total number of upstream requests",
		},
		[]string{"upstream_group", "upstream_name", "method", "status_code"},
	)

	c.upstreamRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    prefix + "_upstream_request_duration_seconds",
			Help:    "Upstream request duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
		},
		[]string{"upstream_group", "upstream_name", "method"},
	)

	c.upstreamErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_upstream_errors_total",
			Help: "Total number of upstream errors",
		},
		[]string{"upstream_group", "upstream_name", "error_type"},
	)

	// 断路器指标
	c.circuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: prefix + "_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
		},
		[]string{"upstream_group", "upstream_name"},
	)

	c.circuitBreakerRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_circuit_breaker_requests_total",
			Help: "Total number of circuit breaker requests",
		},
		[]string{"upstream_group", "upstream_name", "result"},
	)

	c.circuitBreakerStateChanges = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_circuit_breaker_state_changes_total",
			Help: "Total number of circuit breaker state changes",
		},
		[]string{"upstream_group", "upstream_name", "from_state", "to_state"},
	)

	// 负载均衡器指标
	c.loadBalancerSelectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_load_balancer_selections_total",
			Help: "Total number of load balancer selections",
		},
		[]string{"upstream_group", "upstream_name", "balancer_type"},
	)

	c.upstreamHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: prefix + "_upstream_health_status",
			Help: "Upstream health status (1=healthy, 0=unhealthy)",
		},
		[]string{"upstream_group", "upstream_name"},
	)

	// 系统级指标
	c.activeConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: prefix + "_active_connections",
			Help: "Number of active connections",
		},
		[]string{"forward_name"},
	)

	c.rateLimitRejectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prefix + "_rate_limit_rejections_total",
			Help: "Total number of rate limit rejections",
		},
		[]string{"forward_name", "limit_type"},
	)

	// 注册所有指标到注册器
	collectors := []prometheus.Collector{
		c.httpRequestsTotal,
		c.httpRequestDuration,
		c.httpRequestSizeBytes,
		c.httpResponseSizeBytes,
		c.upstreamRequestsTotal,
		c.upstreamRequestDuration,
		c.upstreamErrorsTotal,
		c.circuitBreakerState,
		c.circuitBreakerRequestsTotal,
		c.circuitBreakerStateChanges,
		c.loadBalancerSelectionsTotal,
		c.upstreamHealthStatus,
		c.activeConnections,
		c.rateLimitRejectionsTotal,
	}

	for _, collector := range collectors {
		if err := c.registry.Register(collector); err != nil {
			return err
		}
	}

	return nil
}

// HTTP 服务器指标收集方法实现

// RecordRequest 记录 HTTP 请求
func (c *prometheusCollector) RecordRequest(forwardName, method, path string) {
	// 请求计数将在 RecordResponse 中统一处理，这里可以预留扩展
}

// RecordResponse 记录 HTTP 响应
func (c *prometheusCollector) RecordResponse(forwardName, method, path string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	statusCodeStr := fmt.Sprintf("%d", statusCode)

	// 记录请求总数
	c.httpRequestsTotal.WithLabelValues(forwardName, method, path, statusCodeStr).Inc()

	// 记录请求处理时间
	c.httpRequestDuration.WithLabelValues(forwardName, method, path).Observe(duration.Seconds())

	// 记录请求体大小
	if requestSize > 0 {
		c.httpRequestSizeBytes.WithLabelValues(forwardName, method, path).Observe(float64(requestSize))
	}

	// 记录响应体大小
	if responseSize > 0 {
		c.httpResponseSizeBytes.WithLabelValues(forwardName, method, path, statusCodeStr).Observe(float64(responseSize))
	}
}

// RecordError 记录 HTTP 错误
func (c *prometheusCollector) RecordError(forwardName, errorType string) {
	// HTTP 错误通过 RecordResponse 中的状态码处理
	// 这里可以记录额外的错误信息，如果需要的话
}

// 上游服务指标收集方法实现

// RecordUpstreamRequest 记录上游请求
func (c *prometheusCollector) RecordUpstreamRequest(upstreamGroup, upstreamName, method string) {
	// 上游请求计数将在 RecordUpstreamResponse 中统一处理
}

// RecordUpstreamResponse 记录上游响应
func (c *prometheusCollector) RecordUpstreamResponse(upstreamGroup, upstreamName, method string, statusCode int, duration time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	statusCodeStr := fmt.Sprintf("%d", statusCode)

	// 记录上游请求总数
	c.upstreamRequestsTotal.WithLabelValues(upstreamGroup, upstreamName, method, statusCodeStr).Inc()

	// 记录上游响应时间
	c.upstreamRequestDuration.WithLabelValues(upstreamGroup, upstreamName, method).Observe(duration.Seconds())
}

// RecordUpstreamError 记录上游错误
func (c *prometheusCollector) RecordUpstreamError(upstreamGroup, upstreamName, errorType string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.upstreamErrorsTotal.WithLabelValues(upstreamGroup, upstreamName, errorType).Inc()
}

// 断路器指标收集方法实现

// RecordCircuitBreakerState 记录断路器状态
func (c *prometheusCollector) RecordCircuitBreakerState(upstreamGroup, upstreamName string, state int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.circuitBreakerState.WithLabelValues(upstreamGroup, upstreamName).Set(float64(state))
}

// RecordCircuitBreakerRequest 记录断路器请求
func (c *prometheusCollector) RecordCircuitBreakerRequest(upstreamGroup, upstreamName, result string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.circuitBreakerRequestsTotal.WithLabelValues(upstreamGroup, upstreamName, result).Inc()
}

// RecordCircuitBreakerStateChange 记录断路器状态变化
func (c *prometheusCollector) RecordCircuitBreakerStateChange(upstreamGroup, upstreamName, fromState, toState string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.circuitBreakerStateChanges.WithLabelValues(upstreamGroup, upstreamName, fromState, toState).Inc()
}

// 负载均衡器指标收集方法实现

// RecordLoadBalancerSelection 记录负载均衡器选择
func (c *prometheusCollector) RecordLoadBalancerSelection(upstreamGroup, upstreamName, balancerType string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.loadBalancerSelectionsTotal.WithLabelValues(upstreamGroup, upstreamName, balancerType).Inc()
}

// RecordUpstreamHealthStatus 记录上游健康状态
func (c *prometheusCollector) RecordUpstreamHealthStatus(upstreamGroup, upstreamName string, healthy bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	healthValue := float64(0)
	if healthy {
		healthValue = 1
	}
	c.upstreamHealthStatus.WithLabelValues(upstreamGroup, upstreamName).Set(healthValue)
}

// 系统级指标收集方法实现

// RecordActiveConnections 记录活跃连接数
func (c *prometheusCollector) RecordActiveConnections(forwardName string, connections int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.activeConnections.WithLabelValues(forwardName).Set(float64(connections))
}

// RecordRateLimitRejection 记录限流拒绝
func (c *prometheusCollector) RecordRateLimitRejection(forwardName, limitType string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.rateLimitRejectionsTotal.WithLabelValues(forwardName, limitType).Inc()
}

// 工具方法实现

// GetRegistry 获取 Prometheus 注册器
func (c *prometheusCollector) GetRegistry() *prometheus.Registry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.registry
}

// Name 获取收集器名称
func (c *prometheusCollector) Name() string {
	return c.name
}

// Close 关闭收集器并清理资源
func (c *prometheusCollector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prometheus 收集器不需要特殊的清理操作
	// 注册器会在垃圾回收时自动清理
	return nil
}
