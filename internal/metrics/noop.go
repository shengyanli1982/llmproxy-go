package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// noopCollector 空操作指标收集器，用于禁用指标收集时的占位实现
type noopCollector struct {
	name string
}

// NewNoopCollector 创建新的空操作指标收集器实例
func NewNoopCollector() MetricsCollector {
	return &noopCollector{
		name: "noop",
	}
}

// HTTP 服务器指标收集方法（空实现）

func (c *noopCollector) RecordRequest(forwardName, method, path string) {
	// 空实现
}

func (c *noopCollector) RecordResponse(forwardName, method, path string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
	// 空实现
}

func (c *noopCollector) RecordError(forwardName, errorType string) {
	// 空实现
}

// 上游服务指标收集方法（空实现）

func (c *noopCollector) RecordUpstreamRequest(upstreamGroup, upstreamName, method string) {
	// 空实现
}

func (c *noopCollector) RecordUpstreamResponse(upstreamGroup, upstreamName, method string, statusCode int, duration time.Duration) {
	// 空实现
}

func (c *noopCollector) RecordUpstreamError(upstreamGroup, upstreamName, errorType string) {
	// 空实现
}

// 断路器指标收集方法（空实现）

func (c *noopCollector) RecordCircuitBreakerState(upstreamGroup, upstreamName string, state int) {
	// 空实现
}

func (c *noopCollector) RecordCircuitBreakerRequest(upstreamGroup, upstreamName, result string) {
	// 空实现
}

func (c *noopCollector) RecordCircuitBreakerStateChange(upstreamGroup, upstreamName, fromState, toState string) {
	// 空实现
}

// 负载均衡器指标收集方法（空实现）

func (c *noopCollector) RecordLoadBalancerSelection(upstreamGroup, upstreamName, balancerType string) {
	// 空实现
}

func (c *noopCollector) RecordUpstreamHealthStatus(upstreamGroup, upstreamName string, healthy bool) {
	// 空实现
}

// 系统级指标收集方法（空实现）

func (c *noopCollector) RecordActiveConnections(forwardName string, connections int) {
	// 空实现
}

func (c *noopCollector) RecordRateLimitRejection(forwardName, limitType string) {
	// 空实现
}

// 工具方法

func (c *noopCollector) GetRegistry() *prometheus.Registry {
	// 返回空的注册器
	return prometheus.NewRegistry()
}

func (c *noopCollector) Name() string {
	return c.name
}

func (c *noopCollector) Close() error {
	// 空实现，无需清理资源
	return nil
}
