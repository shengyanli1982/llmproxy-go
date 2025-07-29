package breaker

import (
	"time"

	"github.com/sony/gobreaker"
)

// DefaultSettings 返回默认的熔断器设置
func DefaultSettings() gobreaker.Settings {
	return gobreaker.Settings{
		Name:        "default",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			// 可以在这里添加日志记录
		},
	}
}

// CreateFromConfig 从配置创建熔断器设置的便捷函数
func CreateFromConfig(name string, maxRequests uint32, interval, timeout time.Duration, failureThreshold float64, minRequests uint32) gobreaker.Settings {
	settings := DefaultSettings()
	settings.Name = name
	settings.MaxRequests = maxRequests
	settings.Interval = interval
	settings.Timeout = timeout
	settings.ReadyToTrip = func(counts gobreaker.Counts) bool {
		failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
		return counts.Requests >= minRequests && failureRatio >= failureThreshold
	}
	return settings
}