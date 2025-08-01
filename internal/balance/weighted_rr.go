package balance

import (
	"context"
	"sync"
	"sync/atomic"
)

// WeightedRRBalancer 实现加权轮询负载均衡算法
// 根据上游服务的权重进行选择，权重越高被选中的概率越大
type WeightedRRBalancer struct {
	weights sync.Map // 使用 sync.Map 存储 string -> *int64，支持原子操作
}

// NewWeightedRRBalancer 创建新的加权轮询负载均衡器实例
func NewWeightedRRBalancer() LoadBalancer {
	return &WeightedRRBalancer{}
}

// Select 使用加权轮询算法选择上游服务
// ctx: 上下文信息
// upstreams: 可用的上游服务列表
func (b *WeightedRRBalancer) Select(ctx context.Context, upstreams []Upstream) (Upstream, error) {
	if upstreams == nil {
		return Upstream{}, ErrNilUpstreams
	}
	if len(upstreams) == 0 {
		return Upstream{}, ErrEmptyUpstreams
	}

	// 使用原子操作优化权重计算，无需显式锁
	var totalWeight int64
	var selected Upstream
	var maxCurrentWeight int64 = -1

	for _, upstream := range upstreams {
		weight := int64(upstream.Weight)
		if weight <= 0 {
			weight = 1 // 默认权重为1
		}
		totalWeight += weight

		// 获取或创建原子权重值
		weightPtr := b.getOrCreateAtomicWeight(upstream.Name)

		// 原子操作增加当前权重
		newWeight := atomic.AddInt64(weightPtr, weight)

		// 选择当前权重最大的服务
		if newWeight > maxCurrentWeight {
			maxCurrentWeight = newWeight
			selected = upstream
		}
	}

	// 原子操作减少选中服务的当前权重
	if weightPtr := b.getAtomicWeight(selected.Name); weightPtr != nil {
		atomic.AddInt64(weightPtr, -totalWeight)
	}

	return selected, nil
}

// getOrCreateAtomicWeight 获取或创建原子权重值
func (b *WeightedRRBalancer) getOrCreateAtomicWeight(name string) *int64 {
	if value, ok := b.weights.Load(name); ok {
		return value.(*int64)
	}

	// 创建新的原子权重值
	newWeight := new(int64)
	actual, _ := b.weights.LoadOrStore(name, newWeight)
	return actual.(*int64)
}

// getAtomicWeight 获取原子权重值
func (b *WeightedRRBalancer) getAtomicWeight(name string) *int64 {
	if value, ok := b.weights.Load(name); ok {
		return value.(*int64)
	}
	return nil
}

// UpdateHealth 更新健康状态（加权轮询算法不需要此信息）
// upstreamName: 上游服务名称
// healthy: 健康状态
func (b *WeightedRRBalancer) UpdateHealth(upstreamName string, healthy bool) {
	// 加权轮询算法不需要健康状态信息，此方法为空实现
}

// UpdateLatency 更新延迟信息（加权轮询算法不需要此信息）
// upstreamName: 上游服务名称
// latency: 响应延迟
func (b *WeightedRRBalancer) UpdateLatency(upstreamName string, latency int64) {
	// 加权轮询算法不需要延迟信息，此方法为空实现
}

// Type 获取负载均衡器类型
func (b *WeightedRRBalancer) Type() string {
	return "weighted_roundrobin"
}
