package ratelimit

import (
	"net"
	"net/http"
)

// IPLimiter IP级别限流器
type IPLimiter struct {
	limiter RateLimiter
}

// NewIPLimiter 创建新的IP限流器实例
func NewIPLimiter(perSecond float64, burst int) *IPLimiter {
	return &IPLimiter{
		limiter: NewTokenBucketLimiter(perSecond, burst),
	}
}

// Allow 检查请求IP是否允许通过
func (l *IPLimiter) Allow(req *http.Request) bool {
	ip := l.getClientIP(req)
	if ip == "" {
		return true // 无法获取IP时默认通过
	}

	return l.limiter.Allow(ip)
}

// Reset 重置指定IP的限流状态
func (l *IPLimiter) Reset(ip string) {
	l.limiter.Reset(ip)
}

// getClientIP 从HTTP请求中获取客户端真实IP地址
func (l *IPLimiter) getClientIP(req *http.Request) string {
	// 优先检查X-Forwarded-For头部
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For可能包含多个IP，取第一个
		if ip := parseFirstIP(xff); ip != "" {
			return ip
		}
	}

	// 检查X-Real-IP头部
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return xri
		}
	}

	// 使用RemoteAddr
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}

	return host
}

// parseFirstIP 解析并返回第一个有效的IP地址
func parseFirstIP(xff string) string {
	for i := 0; i < len(xff); i++ {
		if xff[i] == ',' {
			ip := net.ParseIP(xff[:i])
			if ip != nil {
				return xff[:i]
			}
			break
		}
	}

	// 如果没有逗号，检查整个字符串
	if ip := net.ParseIP(xff); ip != nil {
		return xff
	}

	return ""
}
