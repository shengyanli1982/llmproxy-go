package ratelimit

import (
	"net/http"

	"github.com/shengyanli1982/llmproxy-go/internal/response"
	"github.com/shengyanli1982/orbit"
)

// RateLimitMiddleware 限流中间件结构
type RateLimitMiddleware struct {
	ipLimiter       *IPLimiter
	upstreamLimiter *UpstreamLimiter
	enabled         bool
}

// NewRateLimitMiddleware 创建新的限流中间件实例
func NewRateLimitMiddleware(ipPerSecond float64, ipBurst int, upstreamPerSecond float64, upstreamBurst int) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		ipLimiter:       NewIPLimiter(ipPerSecond, ipBurst),
		upstreamLimiter: NewUpstreamLimiter(upstreamPerSecond, upstreamBurst),
		enabled:         true,
	}
}

// Middleware 返回orbit中间件函数
func (m *RateLimitMiddleware) Middleware() orbit.HandlerFunc {
	return func(c *orbit.Context) {
		if !m.enabled {
			c.Next()
			return
		}

		// IP级别限流检查
		if !m.ipLimiter.Allow(c.Request) {
			detail := map[string]interface{}{
				"type": "ipLimit",
			}
			response.Error(response.CodeRateLimit, "too many requests from this IP").
				WithDetail(detail).
				JSON(c, http.StatusTooManyRequests)
			c.Abort()
			return
		}

		// 上游级别限流检查（如果有指定上游）
		if upstream := c.GetString("upstream"); upstream != "" {
			if !m.upstreamLimiter.Allow(upstream) {
				detail := map[string]interface{}{
					"type":     "upstreamLimit",
					"upstream": upstream,
				}
				response.Error(response.CodeUpstreamLimit, "too many requests to this upstream").
					WithDetail(detail).
					JSON(c, http.StatusTooManyRequests)
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// Enable 启用限流
func (m *RateLimitMiddleware) Enable() {
	m.enabled = true
}

// Disable 禁用限流
func (m *RateLimitMiddleware) Disable() {
	m.enabled = false
}

// IsEnabled 检查是否启用限流
func (m *RateLimitMiddleware) IsEnabled() bool {
	return m.enabled
}

// ResetIP 重置指定IP的限流状态
func (m *RateLimitMiddleware) ResetIP(ip string) {
	m.ipLimiter.Reset(ip)
}

// ResetUpstream 重置指定上游的限流状态
func (m *RateLimitMiddleware) ResetUpstream(upstream string) {
	m.upstreamLimiter.Reset(upstream)
}

// AllowRequest 检查HTTP请求是否允许通过（IP级别限流）
func (m *RateLimitMiddleware) AllowRequest(req *http.Request) bool {
	if !m.enabled {
		return true
	}
	return m.ipLimiter.Allow(req)
}

// AllowUpstream 检查指定上游是否允许通过（上游级别限流）
func (m *RateLimitMiddleware) AllowUpstream(upstream string) bool {
	if !m.enabled {
		return true
	}
	return m.upstreamLimiter.Allow(upstream)
}
