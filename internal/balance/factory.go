package balance

import (
	"errors"
	"fmt"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
)

// 工厂相关错误定义
var (
	ErrNilBalanceConfig = errors.New("balance config cannot be nil")
)

// BalanceFactory 代表负载均衡器工厂实现
type BalanceFactory struct{}

// NewFactory 创建新的负载均衡器工厂实例
func NewFactory() LoadBalancerFactory {
	return &BalanceFactory{}
}

// Create 根据配置创建对应的负载均衡器
// config: 负载均衡配置
func (f *BalanceFactory) Create(config *config.BalanceConfig) (LoadBalancer, error) {
	if config == nil {
		return nil, ErrNilBalanceConfig
	}

	strategy := config.Strategy
	if strategy == "" {
		strategy = "roundrobin" // 默认使用轮询
	}

	switch strategy {
	case "roundrobin":
		return NewRRBalancer(), nil
	case "weighted_roundrobin":
		return NewWeightedRRBalancer(), nil
	case "random":
		return NewRandomBalancer(), nil
	case "iphash":
		return NewIPHashBalancer(), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownStrategy, strategy)
	}
}

// CreateFromConfig 从上游组配置创建负载均衡器的便捷方法
// upstreamGroupConfig: 上游组配置
func CreateFromConfig(upstreamGroupConfig *config.UpstreamGroupConfig) (LoadBalancer, error) {
	if upstreamGroupConfig == nil {
		return nil, errors.New("upstream group config cannot be nil")
	}

	// 如果没有负载均衡配置，使用默认的轮询策略
	if upstreamGroupConfig.Balance == nil {
		return NewRRBalancer(), nil
	}

	factory := NewFactory()
	return factory.Create(upstreamGroupConfig.Balance)
}
