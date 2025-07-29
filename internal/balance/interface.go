package balance

import (
	"context"
	"errors"

	"github.com/shengyanli1982/llmproxy-go/internal/breaker"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/sony/gobreaker"
)

// 负载均衡相关错误定义
var (
	ErrNoAvailableUpstream = errors.New("no available upstream")
	ErrUnknownStrategy     = errors.New("unknown load balance strategy")
	ErrNilUpstreams        = errors.New("upstreams cannot be nil")
	ErrEmptyUpstreams      = errors.New("upstreams cannot be empty")
)

// Upstream 代表一个上游服务实例
type Upstream struct {
	Name   string                 // 上游服务名称
	URL    string                 // 上游服务 URL
	Weight int                    // 权重（用于加权轮询）
	Config *config.UpstreamConfig // 上游服务配置
}

// LoadBalancer 代表负载均衡器接口，定义选择上游服务的行为
type LoadBalancer interface {
	// Select 根据负载均衡策略选择一个上游服务
	// ctx: 上下文信息
	// upstreams: 可用的上游服务列表
	Select(ctx context.Context, upstreams []Upstream) (Upstream, error)

	// UpdateHealth 更新上游服务的健康状态
	// upstreamName: 上游服务名称
	// healthy: 健康状态
	UpdateHealth(upstreamName string, healthy bool)

	// UpdateLatency 更新上游服务的响应延迟（用于响应时间感知负载均衡）
	// upstreamName: 上游服务名称
	// latency: 响应延迟（毫秒）
	UpdateLatency(upstreamName string, latency int64)

	// Type 获取负载均衡器类型
	Type() string
}

// LoadBalancerWithBreaker 扩展负载均衡器接口，支持熔断器功能
type LoadBalancerWithBreaker interface {
	LoadBalancer
	
	// CreateBreaker 为上游服务创建熔断器
	CreateBreaker(upstreamName string, settings gobreaker.Settings) error
	
	// GetBreaker 获取指定上游的熔断器
	GetBreaker(upstreamName string) (breaker.CircuitBreaker, bool)
}

// LoadBalancerFactory 代表负载均衡器工厂接口
type LoadBalancerFactory interface {
	// Create 根据配置创建负载均衡器
	// config: 负载均衡配置
	Create(config *config.BalanceConfig) (LoadBalancer, error)
}
