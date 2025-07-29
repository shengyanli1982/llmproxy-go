package ratelimit

import (
	"errors"
)

// 工厂相关错误定义
var (
	ErrInvalidPerSecond = errors.New("perSecond must be greater than 0")
	ErrInvalidBurst     = errors.New("burst must be greater than 0")
)

// rateLimitFactory 代表限流器工厂实现
type rateLimitFactory struct{}

// NewFactory 创建新的限流器工厂实例
func NewFactory() RateLimiterFactory {
	return &rateLimitFactory{}
}

// Create 根据配置创建限流器
func (f *rateLimitFactory) Create(perSecond float64, burst int) (RateLimiter, error) {
	if perSecond <= 0 {
		return nil, ErrInvalidPerSecond
	}
	if burst <= 0 {
		return nil, ErrInvalidBurst
	}

	return NewTokenBucketLimiter(perSecond, burst), nil
}