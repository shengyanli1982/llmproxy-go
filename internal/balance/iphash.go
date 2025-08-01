package balance

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"

	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash/v2"
	"github.com/shengyanli1982/llmproxy-go/internal/constants"
)

// IPHashBalancer 实现基于客户端 IP 的一致性哈希负载均衡算法
// 使用一致性哈希环确保相同客户端 IP 总是路由到相同的上游服务
// 支持虚拟节点机制以提高负载分布的均匀性
type IPHashBalancer struct {
	mu        sync.RWMutex           // 读写锁，保护并发访问
	ring      *consistent.Consistent // 一致性哈希环
	upstreams map[string]Upstream    // 上游服务映射，key 为服务名称
}

// hasher 实现 consistent.Hasher 接口，使用 xxhash 算法
type hasher struct{}

// Sum64 实现哈希函数，使用 xxhash 提供高性能的哈希计算
func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

// member 实现 consistent.Member 接口，用于表示哈希环中的节点
type member string

// String 实现 consistent.Member 接口的 String 方法
func (m member) String() string {
	return string(m)
}

// NewIPHashBalancer 创建新的基于 IP 的一致性哈希负载均衡器实例
// 使用合理的配置参数确保负载分布的均匀性
func NewIPHashBalancer() LoadBalancer {
	return &IPHashBalancer{
		ring:      nil, // 延迟初始化，在第一次添加成员时创建
		upstreams: make(map[string]Upstream),
	}
}

// Select 使用一致性哈希算法根据客户端 IP 选择上游服务
// 相同的客户端 IP 总是会被路由到相同的上游服务
func (b *IPHashBalancer) Select(ctx context.Context, upstreams []Upstream) (Upstream, error) {
	if upstreams == nil {
		return Upstream{}, ErrNilUpstreams
	}
	if len(upstreams) == 0 {
		return Upstream{}, ErrEmptyUpstreams
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 更新哈希环中的节点
	if err := b.updateRing(upstreams); err != nil {
		return Upstream{}, fmt.Errorf("failed to update hash ring: %w", err)
	}

	// 从 context 获取客户端 IP
	clientIP, ok := GetClientIP(ctx)
	if !ok || clientIP == "" {
		// 如果无法获取客户端 IP，使用随机选择作为降级策略
		return b.selectRandomUpstream(upstreams), nil
	}

	// 使用一致性哈希选择上游服务
	member := b.ring.LocateKey([]byte(clientIP))
	if member == nil {
		// 如果哈希环为空，使用随机选择
		return b.selectRandomUpstream(upstreams), nil
	}

	// 根据成员名称查找对应的上游服务
	upstreamName := member.String()
	if upstream, exists := b.upstreams[upstreamName]; exists {
		return upstream, nil
	}

	// 如果找不到对应的上游服务，使用随机选择作为降级策略
	return b.selectRandomUpstream(upstreams), nil
}

// updateRing 更新一致性哈希环中的节点
func (b *IPHashBalancer) updateRing(upstreams []Upstream) error {
	// 如果哈希环还未初始化，先创建它
	if b.ring == nil {
		// 创建初始成员列表
		var members []consistent.Member
		for _, upstream := range upstreams {
			members = append(members, member(upstream.Name))
		}

		// 使用默认配置创建哈希环
		b.ring = consistent.New(members, consistent.Config{
			Hasher: hasher{}, // 使用自定义的 xxhash 哈希函数
		})
	} else {
		// 哈希环已存在，更新节点
		for _, upstream := range upstreams {
			// 添加新节点到哈希环
			if _, exists := b.upstreams[upstream.Name]; !exists {
				b.ring.Add(member(upstream.Name))
			}
		}

		// 移除不再存在的节点
		for name := range b.upstreams {
			found := false
			for _, upstream := range upstreams {
				if upstream.Name == name {
					found = true
					break
				}
			}
			if !found {
				b.ring.Remove(name)
			}
		}
	}

	// 构建新的上游服务映射
	newUpstreams := make(map[string]Upstream)
	for _, upstream := range upstreams {
		newUpstreams[upstream.Name] = upstream
	}

	// 更新上游服务映射
	b.upstreams = newUpstreams
	return nil
}

// selectRandomUpstream 随机选择一个上游服务作为降级策略
func (b *IPHashBalancer) selectRandomUpstream(upstreams []Upstream) Upstream {
	if len(upstreams) == 1 {
		return upstreams[0]
	}

	// 使用加密安全的随机数生成器
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(upstreams))))
	if err != nil {
		// 如果随机数生成失败，返回第一个上游服务
		return upstreams[0]
	}

	return upstreams[n.Int64()]
}

// UpdateHealth 更新上游服务的健康状态
// IPHash 负载均衡器不需要维护健康状态，提供空实现以满足接口要求
func (b *IPHashBalancer) UpdateHealth(upstreamName string, healthy bool) {
	// IPHash 算法不依赖健康状态，此方法为空实现
	// 健康检查应该在更上层的组件中处理
}

// UpdateLatency 更新上游服务的响应延迟
// IPHash 负载均衡器不需要维护延迟信息，提供空实现以满足接口要求
func (b *IPHashBalancer) UpdateLatency(upstreamName string, latency int64) {
	// IPHash 算法不依赖响应延迟，此方法为空实现
	// 延迟监控应该在更上层的组件中处理
}

// Type 获取负载均衡器类型标识
func (b *IPHashBalancer) Type() string {
	return constants.BalanceIPHash
}
