package breaker

import "github.com/sony/gobreaker"

// CircuitBreaker 代表熔断器接口
type CircuitBreaker interface {
	// Execute 执行受保护的操作
	Execute(req func() (interface{}, error)) (interface{}, error)
	
	// Name 获取熔断器名称
	Name() string
	
	// State 获取当前状态
	State() gobreaker.State
}

// CircuitBreakerFactory 代表熔断器工厂接口
type CircuitBreakerFactory interface {
	// Create 根据配置创建熔断器
	Create(name string, settings gobreaker.Settings) (CircuitBreaker, error)
}