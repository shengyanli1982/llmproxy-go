package breaker

import (
	"time"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/constants"
	"github.com/sony/gobreaker"
)

// DefaultSettings 返回默认的熔断器设置
func DefaultSettings() gobreaker.Settings {
	return gobreaker.Settings{
		Name:        constants.DefaultBreakerName,
		MaxRequests: constants.DefaultBreakerMaxRequests,
		Interval:    time.Duration(constants.DefaultBreakerInterval) * time.Millisecond,
		Timeout:     time.Duration(constants.DefaultBreakerCooldown) * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= constants.DefaultBreakerMinRequests && failureRatio >= constants.DefaultBreakerFailureThreshold
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			// 可以在这里添加日志记录
		},
	}
}

// CreateFromConfig 从配置创建熔断器设置的便捷函数
func CreateFromConfig(name string, config *config.BreakerConfig) gobreaker.Settings {
	settings := DefaultSettings()
	settings.Name = name

	// 设置半开状态下允许通过的最大请求数
	if config.MaxRequests > 0 {
		settings.MaxRequests = config.MaxRequests
	}

	// 设置闭合状态下统计周期重置间隔
	if config.Interval > 0 {
		settings.Interval = time.Duration(config.Interval) * time.Millisecond
	}

	// 设置开放状态持续时间
	if config.Cooldown > 0 {
		settings.Timeout = time.Duration(config.Cooldown) * time.Millisecond
	}

	// 设置熔断触发条件
	if config.Threshold > 0 {
		settings.ReadyToTrip = func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= constants.DefaultBreakerMinRequests && failureRatio >= config.Threshold
		}
	}

	return settings
}
