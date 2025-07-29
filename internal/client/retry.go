package client

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// RetryHandler 重试处理器
type RetryHandler struct {
	config *Config
}

// NewRetryHandler 创建新的重试处理器实例
func NewRetryHandler(config *Config) *RetryHandler {
	return &RetryHandler{
		config: config,
	}
}

// DoWithRetry 执行带重试的HTTP请求
func (r *RetryHandler) DoWithRetry(ctx context.Context, fn func() (*http.Response, error)) (*http.Response, error) {
	if !r.config.EnableRetry {
		// 未启用重试，直接执行
		return fn()
	}

	var lastErr error
	maxRetries := r.config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		response, err := fn()
		if err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				// 等待后重试
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(r.calculateDelay(attempt)):
					continue
				}
			}
			continue
		}

		// 检查是否需要重试
		if r.shouldRetry(response) {
			response.Body.Close()
			lastErr = &RetryableError{Message: "server error, retrying"}
			if attempt < maxRetries-1 {
				// 等待后重试
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(r.calculateDelay(attempt)):
					continue
				}
			}
			continue
		}

		// 成功，返回响应
		return response, nil
	}

	// 所有重试都失败了
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("max retries exceeded")
}

// calculateDelay 计算重试延迟（指数退避）
func (r *RetryHandler) calculateDelay(attempt int) time.Duration {
	baseDelay := time.Duration(r.config.RetryDelay) * time.Millisecond
	
	// 指数退避：baseDelay * 2^attempt
	delay := baseDelay
	for i := 0; i < attempt; i++ {
		delay *= 2
	}
	
	// 限制最大延迟为30秒
	maxDelay := 30 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}
	
	return delay
}

// shouldRetry 判断是否应该重试
func (r *RetryHandler) shouldRetry(resp *http.Response) bool {
	if resp == nil {
		return true
	}

	// 基于状态码判断是否重试
	switch resp.StatusCode {
	case http.StatusInternalServerError,     // 500
		http.StatusBadGateway,               // 502
		http.StatusServiceUnavailable,       // 503
		http.StatusGatewayTimeout:           // 504
		return true
	case http.StatusTooManyRequests:        // 429
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