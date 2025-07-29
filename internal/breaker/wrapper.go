package breaker

import "github.com/sony/gobreaker"

// BreakerWrapper 包装sony/gobreaker的实现
type BreakerWrapper struct {
	name string
	cb   *gobreaker.CircuitBreaker
}

// NewCircuitBreaker 创建新的熔断器实例
func NewCircuitBreaker(name string, settings gobreaker.Settings) CircuitBreaker {
	return &BreakerWrapper{
		name: name,
		cb:   gobreaker.NewCircuitBreaker(settings),
	}
}

// Execute 执行受保护的操作
func (w *BreakerWrapper) Execute(req func() (interface{}, error)) (interface{}, error) {
	return w.cb.Execute(req)
}

// Name 获取熔断器名称
func (w *BreakerWrapper) Name() string {
	return w.name
}

// State 获取当前状态
func (w *BreakerWrapper) State() gobreaker.State {
	return w.cb.State()
}