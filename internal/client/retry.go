package client

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/shengyanli1982/retry"
)

// RetryHandler 重试处理器
type RetryHandler struct {
	config     *Config
	retryAgent *retry.Retry
}

// NewRetryHandler 创建新的重试处理器实例
func NewRetryHandler(config *Config) *RetryHandler {
	// 创建重试配置
	retryConfig := retry.NewConfig()

	if config.EnableRetry {
		// 设置重试次数
		if config.MaxRetries > 0 {
			retryConfig = retryConfig.WithAttempts(uint64(config.MaxRetries))
		}

		// 设置初始延迟
		if config.RetryDelay > 0 {
			retryConfig = retryConfig.WithInitDelay(time.Duration(config.RetryDelay) * time.Millisecond)
		}

		// 设置指数退避因子
		retryConfig = retryConfig.WithFactor(2.0)

		// 设置重试条件函数
		retryConfig = retryConfig.WithRetryIfFunc(func(err error) bool {
			if err == nil {
				return false
			}
			// 检查是否是可重试的错误
			if retryableErr, ok := err.(*RetryableError); ok {
				return retryableErr != nil
			}
			// 网络错误通常可以重试
			return true
		})
	} else {
		// 如果禁用重试，设置重试次数为1（即不重试）
		retryConfig = retryConfig.WithAttempts(1)
	}

	// 创建重试实例
	retryAgent := retry.New(retryConfig)

	return &RetryHandler{
		config:     config,
		retryAgent: retryAgent,
	}
}

// DoWithRetry 执行带重试的HTTP请求
func (r *RetryHandler) DoWithRetry(ctx context.Context, fn func() (*http.Response, error)) (*http.Response, error) {
	if !r.config.EnableRetry {
		// 未启用重试，直接执行
		return fn()
	}

	// 使用新的retry库执行重试
	result := r.retryAgent.TryOnConflict(func() (any, error) {
		response, err := fn()
		if err != nil {
			return nil, err
		}

		// 检查是否需要重试
		if r.shouldRetry(response) {
			response.Body.Close()
			return nil, &RetryableError{Message: "server error, retrying"}
		}

		return response, nil
	})

	// 检查结果
	if !result.IsSuccess() {
		if result.TryError() != nil {
			return nil, result.TryError()
		}
		return nil, errors.New("max retries exceeded")
	}

	// 返回成功的响应
	if response, ok := result.Data().(*http.Response); ok {
		return response, nil
	}

	return nil, errors.New("unexpected result type")
}

// shouldRetry 判断是否应该重试
func (r *RetryHandler) shouldRetry(resp *http.Response) bool {
	if resp == nil {
		return true
	}

	// 基于状态码判断是否重试
	switch resp.StatusCode {
	case http.StatusInternalServerError, // 500
		http.StatusBadGateway,         // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	case http.StatusTooManyRequests: // 429
		return true
	default:
		return false
	}
}

// IsEnabled 检查重试是否启用
func (r *RetryHandler) IsEnabled() bool {
	return r.config.EnableRetry
}

// GetConfig 获取重试配置
func (r *RetryHandler) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled":     r.config.EnableRetry,
		"max_retries": r.config.MaxRetries,
		"retry_delay": r.config.RetryDelay,
	}
}

// RetryableError 可重试错误
type RetryableError struct {
	Message string
}

func (e *RetryableError) Error() string {
	return e.Message
}
