package constants

const (
	// LoadBalanceStrategies - 负载均衡策略

	// BalanceRoundRobin 轮询负载均衡策略
	BalanceRoundRobin = "roundrobin"

	// BalanceWeightedRoundRobin 加权轮询负载均衡策略
	BalanceWeightedRoundRobin = "weighted_roundrobin"

	// BalanceRandom 随机负载均衡策略
	BalanceRandom = "random"

	// BalanceIPHash IP哈希负载均衡策略
	BalanceIPHash = "iphash"

	// DefaultBalanceStrategy 默认负载均衡策略
	DefaultBalanceStrategy = BalanceRoundRobin
)
