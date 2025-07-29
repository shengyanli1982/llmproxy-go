package balance

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// randomBalancer 实现随机负载均衡算法
// 随机选择上游服务，适用于服务性能相近的场景
type randomBalancer struct {
	rand *rand.Rand // 随机数生成器
	mu   sync.Mutex // 保护并发访问
}

// NewRandomBalancer 创建新的随机负载均衡器实例
func NewRandomBalancer() LoadBalancer {
	return &randomBalancer{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select 使用随机算法选择上游服务
// ctx: 上下文信息
// upstreams: 可用的上游服务列表
func (b *randomBalancer) Select(ctx context.Context, upstreams []Upstream) (Upstream, error) {
	if upstreams == nil {
		return Upstream{}, ErrNilUpstreams
	}
	if len(upstreams) == 0 {
		return Upstream{}, ErrEmptyUpstreams
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 随机选择一个上游服务
	index := b.rand.Intn(len(upstreams))
	return upstreams[index], nil
}

// UpdateHealth 更新健康状态（随机算法不需要此信息）
// upstreamName: 上游服务名称
// healthy: 健康状态
func (b *randomBalancer) UpdateHealth(upstreamName string, healthy bool) {
	// 随机算法不需要健康状态信息，此方法为空实现
}

// UpdateLatency 更新延迟信息（随机算法不需要此信息）
// upstreamName: 上游服务名称
// latency: 响应延迟
func (b *randomBalancer) UpdateLatency(upstreamName string, latency int64) {
	// 随机算法不需要延迟信息，此方法为空实现
}

// Type 获取负载均衡器类型
func (b *randomBalancer) Type() string {
	return "random"
}