package ratelimit

// RateLimiter 代表限流器接口
type RateLimiter interface {
	// Allow 检查指定key是否允许通过
	Allow(key string) bool

	// Reset 重置指定key的限流状态
	Reset(key string)

	// Type 获取限流器类型
	Type() string
}

// RateLimiterFactory 代表限流器工厂接口
type RateLimiterFactory interface {
	// Create 根据配置创建限流器
	Create(perSecond float64, burst int) (RateLimiter, error)
}

// Config 代表限流配置
type Config struct {
	PerSecond float64 // 每秒允许的请求数
	Burst     int     // 突发流量上限
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		PerSecond: 100.0,
		Burst:     200,
	}
}
