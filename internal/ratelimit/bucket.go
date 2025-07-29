package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

// tokenBucketLimiter 基于token bucket算法的限流器实现
type tokenBucketLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	limit    rate.Limit
	burst    int
}

// NewTokenBucketLimiter 创建新的token bucket限流器实例
func NewTokenBucketLimiter(perSecond float64, burst int) RateLimiter {
	return &tokenBucketLimiter{
		limiters: make(map[string]*rate.Limiter),
		limit:    rate.Limit(perSecond),
		burst:    burst,
	}
}

// Allow 检查指定key是否允许通过
func (l *tokenBucketLimiter) Allow(key string) bool {
	limiter := l.getLimiter(key)
	return limiter.Allow()
}

// Reset 重置指定key的限流状态
func (l *tokenBucketLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.limiters, key)
}

// Type 获取限流器类型
func (l *tokenBucketLimiter) Type() string {
	return "token_bucket"
}

// getLimiter 获取或创建指定key的限流器
func (l *tokenBucketLimiter) getLimiter(key string) *rate.Limiter {
	l.mu.RLock()
	limiter, exists := l.limiters[key]
	l.mu.RUnlock()

	if exists {
		return limiter
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 双重检查
	if limiter, exists := l.limiters[key]; exists {
		return limiter
	}

	// 创建新的限流器
	limiter = rate.NewLimiter(l.limit, l.burst)
	l.limiters[key] = limiter

	return limiter
}
