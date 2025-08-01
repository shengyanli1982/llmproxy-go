package ratelimit

// UpstreamLimiter 上游级别限流器
type UpstreamLimiter struct {
	limiter RateLimiter
}

// NewUpstreamLimiter 创建新的上游限流器实例
func NewUpstreamLimiter(perSecond float64, burst int) *UpstreamLimiter {
	return &UpstreamLimiter{
		limiter: NewTokenBucketLimiter(perSecond, burst),
	}
}

// Allow 检查指定上游是否允许通过
func (l *UpstreamLimiter) Allow(upstreamName string) bool {
	if upstreamName == "" {
		return true // 无上游名称时默认通过
	}

	return l.limiter.Allow(upstreamName)
}

// Reset 重置指定上游的限流状态
func (l *UpstreamLimiter) Reset(upstreamName string) {
	l.limiter.Reset(upstreamName)
}

// Type 获取限流器类型
func (l *UpstreamLimiter) Type() string {
	return "upstream_" + l.limiter.Type()
}
