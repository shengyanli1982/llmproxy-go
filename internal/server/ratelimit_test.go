package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"k8s.io/klog/v2"

	"github.com/shengyanli1982/llmproxy-go/internal/ratelimit"
	"github.com/shengyanli1982/llmproxy-go/internal/response"
)

// TestGinRateLimitMiddleware 测试gin限流中间件
func TestGinRateLimitMiddleware(t *testing.T) {
	// 设置gin为测试模式
	gin.SetMode(gin.TestMode)

	t.Run("disabled rate limit", func(t *testing.T) {
		// 创建禁用限流的服务
		logger := klog.NewKlogr()
		service := &ForwardService{
			logger:      &logger,
			rateLimitMW: nil, // 禁用限流
		}

		// 创建gin引擎和中间件
		router := gin.New()
		router.Use(service.ginRateLimitMiddleware())
		router.GET("/test", func(c *gin.Context) {
			response.OK(c, map[string]interface{}{"message": "success"})
		})

		// 发送请求
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("enabled rate limit - allow request", func(t *testing.T) {
		// 创建限流中间件
		rateLimitMW := ratelimit.NewRateLimitMiddleware(
			10.0,  // IP每秒10个请求
			20,    // IP突发20个请求
			100.0, // 上游每秒100个请求
			200,   // 上游突发200个请求
		)

		// 创建服务
		logger := klog.NewKlogr()
		service := &ForwardService{
			logger:      &logger,
			rateLimitMW: rateLimitMW,
		}

		// 创建gin引擎和中间件
		router := gin.New()
		router.Use(service.ginRateLimitMiddleware())
		router.GET("/test", func(c *gin.Context) {
			response.OK(c, map[string]interface{}{"message": "success"})
		})

		// 发送请求
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("enabled rate limit - exceed limit", func(t *testing.T) {
		// 创建严格的限流中间件
		rateLimitMW := ratelimit.NewRateLimitMiddleware(
			1.0,   // IP每秒1个请求
			1,     // IP突发1个请求
			100.0, // 上游每秒100个请求
			200,   // 上游突发200个请求
		)

		// 创建服务
		logger := klog.NewKlogr()
		service := &ForwardService{
			logger:      &logger,
			rateLimitMW: rateLimitMW,
		}

		// 创建gin引擎和中间件
		router := gin.New()
		router.Use(service.ginRateLimitMiddleware())
		router.GET("/test", func(c *gin.Context) {
			response.OK(c, map[string]interface{}{"message": "success"})
		})

		// 发送第一个请求（应该成功）
		req1 := httptest.NewRequest("GET", "/test", nil)
		req1.RemoteAddr = "127.0.0.1:12345"
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// 立即发送第二个请求（应该被限流）
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.RemoteAddr = "127.0.0.1:12345"
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)

		// 验证错误响应内容 - 检查新的响应格式
		responseBody := w2.Body.String()
		assert.Contains(t, responseBody, "errorCode")
		assert.Contains(t, responseBody, "1004")
		assert.Contains(t, responseBody, "too many requests from this IP")
	})
}
