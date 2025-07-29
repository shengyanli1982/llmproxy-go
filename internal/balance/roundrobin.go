package balance

import (
	"context"
	"sync"
)

// roundRobinBalancer 实现轮询负载均衡算法
// 按顺序依次选择上游服务，实现请求的均匀分布
type roundRobinBalancer struct {
	mu    sync.Mutex // 保护并发访问
	index int        // 当前选择索引
}

// NewRoundRobinBalancer 创建新的轮询负载均衡器实例
func NewRoundRobinBalancer() LoadBalancer {
	return &roundRobinBalancer{
		index: 0,
	}
}

// Select 使用轮询算法选择上游服务
// ctx: 上下文信息
// upstreams: 可用的上游服务列表
func (b *roundRobinBalancer) Select(ctx context.Context, upstreams []Upstream) (Upstream, error) {
	if upstreams == nil {
		return Upstream{}, ErrNilUpstreams
	}
	if len(upstreams) == 0 {
		return Upstream{}, ErrEmptyUpstreams
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 选择当前索引对应的上游服务
	selected := upstreams[b.index%len(upstreams)]
	b.index++

	return selected, nil
}

// UpdateHealth 更新健康状态（轮询算法不需要此信息）
// upstreamName: 上游服务名称
// healthy: 健康状态
func (b *roundRobinBalancer) UpdateHealth(upstreamName string, healthy bool) {
	// 轮询算法不需要健康状态信息，此方法为空实现
}

// UpdateLatency 更新延迟信息（轮询算法不需要此信息）
// upstreamName: 上游服务名称
// latency: 响应延迟
func (b *roundRobinBalancer) UpdateLatency(upstreamName string, latency int64) {
	// 轮询算法不需要延迟信息，此方法为空实现
}

// Type 获取负载均衡器类型
func (b *roundRobinBalancer) Type() string {
	return "roundrobin"
}