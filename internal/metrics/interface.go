package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MetricsCollector 代表指标收集器接口，定义统一的指标收集行为
type MetricsCollector interface {
	// HTTP 服务器指标收集方法

	// RecordRequest 记录 HTTP 请求
	// forwardName: 转发服务名称
	// method: HTTP 方法
	// path: 请求路径
	RecordRequest(forwardName, method, path string)

	// RecordResponse 记录 HTTP 响应
	// forwardName: 转发服务名称
	// method: HTTP 方法
	// path: 请求路径
	// statusCode: HTTP 状态码
	// duration: 请求处理时间
	// requestSize: 请求体大小（字节）
	// responseSize: 响应体大小（字节）
	RecordResponse(forwardName, method, path string, statusCode int, duration time.Duration, requestSize, responseSize int64)

	// RecordError 记录 HTTP 错误
	// forwardName: 转发服务名称
	// errorType: 错误类型
	RecordError(forwardName, errorType string)

	// 上游服务指标收集方法

	// RecordUpstreamRequest 记录上游请求
	// upstreamGroup: 上游组名称
	// upstreamName: 上游服务名称
	// method: HTTP 方法
	RecordUpstreamRequest(upstreamGroup, upstreamName, method string)

	// RecordUpstreamResponse 记录上游响应
	// upstreamGroup: 上游组名称
	// upstreamName: 上游服务名称
	// method: HTTP 方法
	// statusCode: HTTP 状态码
	// duration: 响应时间
	RecordUpstreamResponse(upstreamGroup, upstreamName, method string, statusCode int, duration time.Duration)

	// RecordUpstreamError 记录上游错误
	// upstreamGroup: 上游组名称
	// upstreamName: 上游服务名称
	// errorType: 错误类型
	RecordUpstreamError(upstreamGroup, upstreamName, errorType string)

	// 断路器指标收集方法

	// RecordCircuitBreakerState 记录断路器状态
	// upstreamGroup: 上游组名称
	// upstreamName: 上游服务名称
	// state: 断路器状态（0=关闭, 1=半开, 2=开启）
	RecordCircuitBreakerState(upstreamGroup, upstreamName string, state int)

	// RecordCircuitBreakerRequest 记录断路器请求
	// upstreamGroup: 上游组名称
	// upstreamName: 上游服务名称
	// result: 请求结果（success, failure, rejected）
	RecordCircuitBreakerRequest(upstreamGroup, upstreamName, result string)

	// RecordCircuitBreakerStateChange 记录断路器状态变化
	// upstreamGroup: 上游组名称
	// upstreamName: 上游服务名称
	// fromState: 原状态
	// toState: 新状态
	RecordCircuitBreakerStateChange(upstreamGroup, upstreamName, fromState, toState string)

	// 负载均衡器指标收集方法

	// RecordLoadBalancerSelection 记录负载均衡器选择
	// upstreamGroup: 上游组名称
	// upstreamName: 上游服务名称
	// balancerType: 负载均衡器类型
	RecordLoadBalancerSelection(upstreamGroup, upstreamName, balancerType string)

	// RecordUpstreamHealthStatus 记录上游健康状态
	// upstreamGroup: 上游组名称
	// upstreamName: 上游服务名称
	// healthy: 健康状态（true=健康, false=不健康）
	RecordUpstreamHealthStatus(upstreamGroup, upstreamName string, healthy bool)

	// 系统级指标收集方法

	// RecordActiveConnections 记录活跃连接数
	// forwardName: 转发服务名称
	// connections: 连接数
	RecordActiveConnections(forwardName string, connections int)

	// RecordRateLimitRejection 记录限流拒绝
	// forwardName: 转发服务名称
	// limitType: 限流类型（ip, global）
	RecordRateLimitRejection(forwardName, limitType string)

	// 工具方法

	// GetRegistry 获取 Prometheus 注册器，用于与 orbit 框架集成
	GetRegistry() *prometheus.Registry

	// Name 获取收集器名称
	Name() string

	// Close 关闭收集器并清理资源
	Close() error
}

// MetricsCollectorFactory 代表指标收集器工厂接口
type MetricsCollectorFactory interface {
	// Create 根据配置创建指标收集器
	// config: 指标收集器配置
	Create(config *Config) (MetricsCollector, error)
}

// Config 代表指标收集器配置
type Config struct {
	// Type 指标收集器类型（prometheus, noop）
	Type string `yaml:"type" json:"type"`

	// Enabled 是否启用指标收集
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Namespace 指标命名空间前缀
	Namespace string `yaml:"namespace" json:"namespace"`

	// Subsystem 指标子系统名称
	Subsystem string `yaml:"subsystem" json:"subsystem"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Type:      "noop",
		Enabled:   true,
		Namespace: "llmproxy",
		Subsystem: "",
	}
}
