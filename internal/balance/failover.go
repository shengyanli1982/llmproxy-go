package balance

import (
	"context"
	"sync"

	"github.com/shengyanli1982/llmproxy-go/internal/breaker"
	"github.com/sony/gobreaker"
)

// failoverBalancer 实现故障转移负载均衡算法
// 按优先级顺序选择第一个健康的上游服务，适用于主备场景
type failoverBalancer struct {
	mu        sync.RWMutex    // 读写锁，保护并发访问
	healthMap map[string]bool // 健康状态映射
	breakerMap map[string]breaker.CircuitBreaker // 熔断器映射
	breakerFactory breaker.CircuitBreakerFactory // 熔断器工厂
}

// NewFailoverBalancer 创建新的故障转移负载均衡器实例
func NewFailoverBalancer() LoadBalancer {
	return &failoverBalancer{
		healthMap:      make(map[string]bool),
		breakerMap:     make(map[string]breaker.CircuitBreaker),
		breakerFactory: breaker.NewFactory(),
	}
}

// Select 按优先级顺序选择第一个健康的上游服务
// ctx: 上下文信息
// upstreams: 可用的上游服务列表（按优先级排序）
func (b *failoverBalancer) Select(ctx context.Context, upstreams []Upstream) (Upstream, error) {
	if upstreams == nil {
		return Upstream{}, ErrNilUpstreams
	}
	if len(upstreams) == 0 {
		return Upstream{}, ErrEmptyUpstreams
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	// 按顺序查找第一个健康且熔断器允许的上游服务
	for _, upstream := range upstreams {
		// 检查健康状态
		if healthy, exists := b.healthMap[upstream.Name]; exists && !healthy {
			continue
		}

		// 检查熔断器状态
		if cb, exists := b.breakerMap[upstream.Name]; exists {
			if cb.State() == gobreaker.StateOpen {
				continue // 熔断器开启，跳过此上游
			}
		}

		return upstream, nil
	}

	// 如果所有服务都不健康或熔断，返回第一个作为最后尝试
	return upstreams[0], nil
}

// UpdateHealth 更新上游服务的健康状态
// upstreamName: 上游服务名称
// healthy: 健康状态
func (b *failoverBalancer) UpdateHealth(upstreamName string, healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.healthMap[upstreamName] = healthy
}

// UpdateLatency 更新延迟信息（故障转移算法不需要此信息）
// upstreamName: 上游服务名称
// latency: 响应延迟
func (b *failoverBalancer) UpdateLatency(upstreamName string, latency int64) {
	// 故障转移算法不需要延迟信息，此方法为空实现
}

// CreateBreaker 为上游服务创建熔断器
func (b *failoverBalancer) CreateBreaker(upstreamName string, settings gobreaker.Settings) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	cb, err := b.breakerFactory.Create(upstreamName, settings)
	if err != nil {
		return err
	}

	b.breakerMap[upstreamName] = cb
	return nil
}

// GetBreaker 获取指定上游的熔断器
func (b *failoverBalancer) GetBreaker(upstreamName string) (breaker.CircuitBreaker, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	cb, exists := b.breakerMap[upstreamName]
	return cb, exists
}

// Type 获取负载均衡器类型
func (b *failoverBalancer) Type() string {
	return "failover"
}