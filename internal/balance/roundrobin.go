package balance

import (
	"context"
	"sync/atomic"

	"github.com/shengyanli1982/llmproxy-go/internal/constants"
)

// RRBalancer 实现轮询负载均衡算法
// 按顺序依次选择上游服务，实现请求的均匀分布
type RRBalancer struct {
	index uint64 // 当前选择索引，使用原子操作
}

// NewRRBalancer 创建新的轮询负载均衡器实例
func NewRRBalancer() LoadBalancer {
	return &RRBalancer{
		index: 0,
	}
}

// Select 使用轮询算法选择上游服务
// ctx: 上下文信息
// upstreams: 可用的上游服务列表
func (b *RRBalancer) Select(ctx context.Context, upstreams []Upstream) (Upstream, error) {
	if upstreams == nil {
		return Upstream{}, ErrNilUpstreams
	}
	if len(upstreams) == 0 {
		return Upstream{}, ErrEmptyUpstreams
	}

	// 使用原子操作获取下一个索引
	idx := atomic.AddUint64(&b.index, 1) - 1
	selectedIndex := idx % uint64(len(upstreams))
	selected := upstreams[selectedIndex]

	// 注意：负载均衡器的选择日志将在调用方记录
	// 这里记录一些关键的选择决策信息供调用方使用

	return selected, nil
}

// UpdateHealth 更新健康状态（轮询算法不需要此信息）
// upstreamName: 上游服务名称
// healthy: 健康状态
func (b *RRBalancer) UpdateHealth(upstreamName string, healthy bool) {
	// 轮询算法不需要健康状态信息，此方法为空实现
}

// UpdateLatency 更新延迟信息（轮询算法不需要此信息）
// upstreamName: 上游服务名称
// latency: 响应延迟
func (b *RRBalancer) UpdateLatency(upstreamName string, latency int64) {
	// 轮询算法不需要延迟信息，此方法为空实现
}

// Type 获取负载均衡器类型
func (b *RRBalancer) Type() string {
	return constants.BalanceRoundRobin
}
