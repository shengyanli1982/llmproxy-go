package ratelimit

import (
	"errors"
)

// 工厂相关错误定义
var (
	ErrInvalidPerSecond = errors.New("perSecond must be greater than 0")
	ErrInvalidBurst     = errors.New("burst must be greater than 0")
)

// defaultFactory 代表默认限流器工厂实现
type defaultFactory struct{}

// NewFactory 创建新的限流器工厂实例
func NewFactory() RateLimiterFactory {
	return &defaultFactory{}
}

// Create 根据配置创建限流器
func (f *defaultFactory) Create(perSecond float64, burst int) (RateLimiter, error) {
	if perSecond <= 0 {
		return nil, ErrInvalidPerSecond
	}
	if burst <= 0 {
		return nil, ErrInvalidBurst
	}

	return NewTokenBucketLimiter(perSecond, burst), nil
}