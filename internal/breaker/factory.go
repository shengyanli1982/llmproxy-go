package breaker

import (
	"errors"

	"github.com/sony/gobreaker"
)

// 工厂相关错误定义
var (
	ErrEmptyName     = errors.New("breaker name cannot be empty")
	ErrNilSettings   = errors.New("breaker settings cannot be nil")
)

// defaultFactory 代表默认熔断器工厂实现
type defaultFactory struct{}

// NewFactory 创建新的熔断器工厂实例
func NewFactory() CircuitBreakerFactory {
	return &defaultFactory{}
}

// Create 根据配置创建熔断器
func (f *defaultFactory) Create(name string, settings gobreaker.Settings) (CircuitBreaker, error) {
	if name == "" {
		return nil, ErrEmptyName
	}

	// 使用提供的settings创建gobreaker实例
	gb := gobreaker.NewCircuitBreaker(settings)
	
	return &circuitBreakerWrapper{
		name: name,
		cb:   gb,
	}, nil
}