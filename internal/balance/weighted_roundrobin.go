package balance

import (
	"context"
	"sync"
)

// weightedRoundRobinBalancer 实现加权轮询负载均衡算法
// 根据上游服务的权重进行选择，权重越高被选中的概率越大
type weightedRoundRobinBalancer struct {
	mu      sync.Mutex     // 保护并发访问
	weights map[string]int // 当前权重映射
}

// NewWeightedRoundRobinBalancer 创建新的加权轮询负载均衡器实例
func NewWeightedRoundRobinBalancer() LoadBalancer {
	return &weightedRoundRobinBalancer{
		weights: make(map[string]int),
	}
}

// Select 使用加权轮询算法选择上游服务
// ctx: 上下文信息
// upstreams: 可用的上游服务列表
func (b *weightedRoundRobinBalancer) Select(ctx context.Context, upstreams []Upstream) (Upstream, error) {
	if upstreams == nil {
		return Upstream{}, ErrNilUpstreams
	}
	if len(upstreams) == 0 {
		return Upstream{}, ErrEmptyUpstreams
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 计算总权重并更新当前权重
	var totalWeight int
	var selected Upstream
	maxCurrentWeight := -1

	for _, upstream := range upstreams {
		weight := upstream.Weight
		if weight <= 0 {
			weight = 1 // 默认权重为1
		}
		totalWeight += weight

		// 增加当前权重
		currentWeight := b.weights[upstream.Name] + weight
		b.weights[upstream.Name] = currentWeight

		// 选择当前权重最大的服务
		if currentWeight > maxCurrentWeight {
			maxCurrentWeight = currentWeight
			selected = upstream
		}
	}

	// 减少选中服务的当前权重
	b.weights[selected.Name] -= totalWeight

	return selected, nil
}

// UpdateHealth 更新健康状态（加权轮询算法不需要此信息）
// upstreamName: 上游服务名称
// healthy: 健康状态
func (b *weightedRoundRobinBalancer) UpdateHealth(upstreamName string, healthy bool) {
	// 加权轮询算法不需要健康状态信息，此方法为空实现
}

// UpdateLatency 更新延迟信息（加权轮询算法不需要此信息）
// upstreamName: 上游服务名称
// latency: 响应延迟
func (b *weightedRoundRobinBalancer) UpdateLatency(upstreamName string, latency int64) {
	// 加权轮询算法不需要延迟信息，此方法为空实现
}

// Type 获取负载均衡器类型
func (b *weightedRoundRobinBalancer) Type() string {
	return "weighted_roundrobin"
}