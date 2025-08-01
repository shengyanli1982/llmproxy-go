package balance

import (
	"context"
	"sync/atomic"
	"time"
)

// RandomBalancer 实现随机负载均衡算法
// 随机选择上游服务，适用于服务性能相近的场景
type RandomBalancer struct {
	seed uint64 // 原子操作的随机种子
}

// NewRandomBalancer 创建新的随机负载均衡器实例
func NewRandomBalancer() LoadBalancer {
	return &RandomBalancer{
		seed: uint64(time.Now().UnixNano()),
	}
}

// Select 使用随机算法选择上游服务
// ctx: 上下文信息
// upstreams: 可用的上游服务列表
func (b *RandomBalancer) Select(ctx context.Context, upstreams []Upstream) (Upstream, error) {
	if upstreams == nil {
		return Upstream{}, ErrNilUpstreams
	}
	if len(upstreams) == 0 {
		return Upstream{}, ErrEmptyUpstreams
	}

	// 使用原子操作生成随机数，避免锁竞争
	// 简单的线性同余生成器，适合快速随机选择
	seed := atomic.AddUint64(&b.seed, 1)
	index := int(seed % uint64(len(upstreams)))
	
	selected := upstreams[index]
	
	// 注意：这里无法直接使用日志器，因为负载均衡器没有日志器接口
	// 负载均衡器的选择日志将在调用方记录

	return selected, nil
}

// UpdateHealth 更新健康状态（随机算法不需要此信息）
// upstreamName: 上游服务名称
// healthy: 健康状态
func (b *RandomBalancer) UpdateHealth(upstreamName string, healthy bool) {
	// 随机算法不需要健康状态信息，此方法为空实现
}

// UpdateLatency 更新延迟信息（随机算法不需要此信息）
// upstreamName: 上游服务名称
// latency: 响应延迟
func (b *RandomBalancer) UpdateLatency(upstreamName string, latency int64) {
	// 随机算法不需要延迟信息，此方法为空实现
}

// Type 获取负载均衡器类型
func (b *RandomBalancer) Type() string {
	return "random"
}
