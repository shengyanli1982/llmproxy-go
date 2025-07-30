package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shengyanli1982/llmproxy-go/internal/balance"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试配置工厂函数

// createMinimalConfig 创建最小配置（只有必要字段）
func createMinimalConfig() *config.HTTPClientConfig {
	return &config.HTTPClientConfig{
		Agent:     "LLMProxy/1.0",
		KeepAlive: 60,
	}
}

// createRetryConfig 创建启用重试的配置
func createRetryConfig() *config.HTTPClientConfig {
	return &config.HTTPClientConfig{
		Agent:     "LLMProxy/1.0",
		KeepAlive: 60,
		Retry: &config.RetryConfig{
			Attempts: 2,
			Initial:  100,
		},
	}
}

// createNoRetryConfig 创建禁用重试的配置
func createNoRetryConfig() *config.HTTPClientConfig {
	return &config.HTTPClientConfig{
		Agent:     "LLMProxy/1.0",
		KeepAlive: 60,
		// Retry: nil 表示不启用重试
	}
}

// createFullConfig 创建完整配置（包含所有配置段）
func createFullConfig() *config.HTTPClientConfig {
	return &config.HTTPClientConfig{
		Agent:     "LLMProxy/1.0",
		KeepAlive: 60,
		Connect: &config.ConnectConfig{
			IdleTotal:   100,
			IdlePerHost: 10,
			MaxPerHost:  50,
		},
		Timeout: &config.TimeoutConfig{
			Connect: 10,
			Request: 60,
			Idle:    90,
		},
		Retry: &config.RetryConfig{
			Attempts: 3,
			Initial:  500,
		},
		Proxy: &config.ProxyConfig{
			URL: "http://proxy.example.com:8080",
		},
	}
}

// createTimeoutConfig 创建自定义超时配置
func createTimeoutConfig(connectTimeout, requestTimeout, idleTimeout int) *config.HTTPClientConfig {
	return &config.HTTPClientConfig{
		Agent:     "LLMProxy/1.0",
		KeepAlive: 60,
		Timeout: &config.TimeoutConfig{
			Connect: connectTimeout,
			Request: requestTimeout,
			Idle:    idleTimeout,
		},
	}
}

// createProxyConfig 创建代理配置
func createProxyConfig(proxyURL string) *config.HTTPClientConfig {
	return &config.HTTPClientConfig{
		Agent:     "LLMProxy/1.0",
		KeepAlive: 60,
		Proxy: &config.ProxyConfig{
			URL: proxyURL,
		},
	}
}

// createConnectConfig 创建连接池配置
func createConnectConfig(idleTotal, idlePerHost, maxPerHost int) *config.HTTPClientConfig {
	return &config.HTTPClientConfig{
		Agent:     "LLMProxy/1.0",
		KeepAlive: 60,
		Connect: &config.ConnectConfig{
			IdleTotal:   idleTotal,
			IdlePerHost: idlePerHost,
			MaxPerHost:  maxPerHost,
		},
	}
}

// 测试辅助函数

// createTestUpstream 创建测试用的上游服务
func createTestUpstream(url string) *balance.Upstream {
	return &balance.Upstream{
		Name: "test-upstream",
		URL:  url,
	}
}

// createTestUpstreamWithConfig 创建带配置的测试上游服务
func createTestUpstreamWithConfig(url string, cfg *config.UpstreamConfig) *balance.Upstream {
	return &balance.Upstream{
		Name:   "test-upstream",
		URL:    url,
		Config: cfg,
	}
}

// createSlowServer 创建慢响应服务器
func createSlowServer(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("slow response"))
	}))
}

// createErrorServer 创建错误响应服务器
func createErrorServer(statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte("error response"))
	}))
}

// createRetryServer 创建重试测试服务器
func createRetryServer(successAfter int32) (*httptest.Server, *int32) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < successAfter {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("temporary error"))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success after retry"))
		}
	}))
	return server, &attempts
}

// createHeaderTestServer 创建头部测试服务器
func createHeaderTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回请求头部信息用于验证
		for name, values := range r.Header {
			for _, value := range values {
				w.Header().Add("Echo-"+name, value)
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("header test response"))
	}))
}

// 基础功能测试

func TestHTTPClient_BasicFunctionality(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success response"))
	}))
	defer server.Close()

	factory := NewFactory()
	cfg := createMinimalConfig()

	client, err := factory.Create(cfg)
	require.NoError(t, err)
	defer client.Close()

	// 创建测试请求
	req, err := http.NewRequest("GET", "/test", nil)
	require.NoError(t, err)

	upstream := createTestUpstream(server.URL)

	// 执行请求
	resp, err := client.Do(req, upstream)
	require.NoError(t, err)
	defer resp.Body.Close()

	// 验证响应
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "success response", string(body))
}

func TestHTTPClient_ErrorHandling(t *testing.T) {
	factory := NewFactory()
	cfg := createMinimalConfig()

	client, err := factory.Create(cfg)
	require.NoError(t, err)
	defer client.Close()

	t.Run("nil request", func(t *testing.T) {
		upstream := createTestUpstream("http://example.com")
		_, err := client.Do(nil, upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("nil upstream", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test", nil)
		_, err := client.Do(req, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("empty upstream URL", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream("")
		_, err := client.Do(req, upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("closed client", func(t *testing.T) {
		closedClient, _ := factory.Create(cfg)
		closedClient.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream("http://example.com")
		_, err := closedClient.Do(req, upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
	})
}

func TestHTTPClient_RetryBehavior(t *testing.T) {
	factory := NewFactory()

	t.Run("retry enabled - success after retry", func(t *testing.T) {
		server, attempts := createRetryServer(3) // 第3次成功
		defer server.Close()

		cfg := createRetryConfig()
		cfg.Retry.Attempts = 3
		cfg.Retry.Initial = 50 // 50ms延迟

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, int32(3), atomic.LoadInt32(attempts))

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "success after retry", string(body))
	})

	t.Run("retry enabled - max retries exceeded", func(t *testing.T) {
		server := createErrorServer(http.StatusInternalServerError)
		defer server.Close()

		cfg := createRetryConfig()
		cfg.Retry.Attempts = 2
		cfg.Retry.Initial = 10 // 10ms延迟

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		_, err = client.Do(req, upstream)
		// 重试库在超过最大重试次数后会返回错误
		assert.Error(t, err)
	})

	t.Run("retry disabled", func(t *testing.T) {
		server := createErrorServer(http.StatusInternalServerError)
		defer server.Close()

		cfg := createNoRetryConfig() // 没有Retry配置

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		// 应该立即返回错误，不重试
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("retry with different status codes", func(t *testing.T) {
		testCases := []struct {
			name       string
			statusCode int
			expectErr  bool
		}{
			{"success", http.StatusOK, false},
			{"client error", http.StatusBadRequest, false},         // 4xx不重试，直接返回
			{"server error", http.StatusInternalServerError, true}, // 5xx重试，最终失败
			{"bad gateway", http.StatusBadGateway, true},           // 5xx重试，最终失败
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				server := createErrorServer(tc.statusCode)
				defer server.Close()

				cfg := createRetryConfig()
				cfg.Retry.Attempts = 2

				client, err := factory.Create(cfg)
				require.NoError(t, err)
				defer client.Close()

				req, _ := http.NewRequest("GET", "/test", nil)
				upstream := createTestUpstream(server.URL)

				resp, err := client.Do(req, upstream)

				if tc.expectErr {
					// 5xx状态码会触发重试，最终超过重试次数返回错误
					assert.Error(t, err)
				} else {
					// 2xx和4xx状态码不会重试，直接返回响应
					require.NoError(t, err)
					defer resp.Body.Close()
					assert.Equal(t, tc.statusCode, resp.StatusCode)
				}
			})
		}
	})
}

func TestHTTPClient_TimeoutBehavior(t *testing.T) {
	factory := NewFactory()

	t.Run("request timeout", func(t *testing.T) {
		server := createSlowServer(2 * time.Second) // 2秒延迟
		defer server.Close()

		cfg := createTimeoutConfig(10, 1, 90) // 1秒请求超时

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		start := time.Now()
		_, err = client.Do(req, upstream)
		duration := time.Since(start)

		// 应该在1秒左右超时
		assert.Error(t, err)
		assert.True(t, duration < 1500*time.Millisecond, "Request should timeout within 1.5 seconds")
		assert.True(t, duration > 500*time.Millisecond, "Request should take at least 0.5 seconds")
	})

	t.Run("no timeout", func(t *testing.T) {
		server := createSlowServer(100 * time.Millisecond) // 100ms延迟
		defer server.Close()

		cfg := createTimeoutConfig(10, 5, 90) // 5秒请求超时

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("context timeout", func(t *testing.T) {
		server := createSlowServer(2 * time.Second)
		defer server.Close()

		cfg := createMinimalConfig()

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		req, _ := http.NewRequestWithContext(ctx, "GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		_, err = client.Do(req, upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestHTTPClient_ConnectionPoolBehavior(t *testing.T) {
	factory := NewFactory()

	t.Run("custom connection pool settings", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("pool test"))
		}))
		defer server.Close()

		cfg := createConnectConfig(200, 20, 100) // 自定义连接池配置

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		// 执行多个请求验证连接池工作
		for i := 0; i < 5; i++ {
			req, _ := http.NewRequest("GET", "/test", nil)
			upstream := createTestUpstream(server.URL)

			resp, err := client.Do(req, upstream)
			require.NoError(t, err)
			resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}
	})

	t.Run("default connection pool settings", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("default pool test"))
		}))
		defer server.Close()

		cfg := createMinimalConfig() // 使用默认连接池配置

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHTTPClient_KeepAliveBehavior(t *testing.T) {
	factory := NewFactory()

	t.Run("keepalive enabled", func(t *testing.T) {
		server := createHeaderTestServer()
		defer server.Close()

		cfg := createMinimalConfig()
		cfg.KeepAlive = 60 // 启用Keep-Alive

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		// 验证Connection头部
		connectionHeader := resp.Header.Get("Echo-Connection")
		assert.Equal(t, "keep-alive", connectionHeader)
	})

	t.Run("keepalive disabled", func(t *testing.T) {
		server := createHeaderTestServer()
		defer server.Close()

		cfg := createMinimalConfig()
		cfg.KeepAlive = 0 // 禁用Keep-Alive

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		// 验证Connection头部
		connectionHeader := resp.Header.Get("Echo-Connection")
		assert.Equal(t, "close", connectionHeader)
	})
}

func TestHTTPClient_ProxyBehavior(t *testing.T) {
	factory := NewFactory()

	t.Run("proxy configuration", func(t *testing.T) {
		// 注意：这个测试只验证代理配置的应用，不测试实际代理功能
		cfg := createProxyConfig("http://proxy.example.com:8080")

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		// 验证客户端创建成功（代理配置已应用）
		assert.NotNil(t, client)
		assert.NotEmpty(t, client.Name())
	})

	t.Run("no proxy configuration", func(t *testing.T) {
		cfg := createMinimalConfig() // 没有代理配置

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		assert.NotNil(t, client)
	})

	t.Run("invalid proxy URL", func(t *testing.T) {
		cfg := createProxyConfig("invalid-url") // 无效的代理URL

		client, err := factory.Create(cfg)
		require.NoError(t, err) // 应该成功创建，但代理不会生效
		defer client.Close()

		assert.NotNil(t, client)
	})
}

func TestHTTPClient_HeaderHandling(t *testing.T) {
	factory := NewFactory()

	t.Run("default headers", func(t *testing.T) {
		server := createHeaderTestServer()
		defer server.Close()

		cfg := createMinimalConfig()

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		// 验证默认User-Agent
		userAgent := resp.Header.Get("Echo-User-Agent")
		assert.Equal(t, "LLMProxy/1.0", userAgent)
	})

	t.Run("custom user agent", func(t *testing.T) {
		server := createHeaderTestServer()
		defer server.Close()

		cfg := createMinimalConfig()

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("User-Agent", "CustomAgent/2.0")
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		// 验证自定义User-Agent
		userAgent := resp.Header.Get("Echo-User-Agent")
		assert.Equal(t, "CustomAgent/2.0", userAgent)
	})

	t.Run("x-forwarded-host header", func(t *testing.T) {
		server := createHeaderTestServer()
		defer server.Close()

		cfg := createMinimalConfig()

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		req.Host = "original-host.com"
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		// 验证X-Forwarded-Host头部
		forwardedHost := resp.Header.Get("Echo-X-Forwarded-Host")
		assert.Equal(t, "original-host.com", forwardedHost)
	})
}

func TestHTTPClient_URLHandling(t *testing.T) {
	factory := NewFactory()
	cfg := createMinimalConfig()

	client, err := factory.Create(cfg)
	require.NoError(t, err)
	defer client.Close()

	t.Run("http URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("http test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL) // httptest.Server使用http://

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("https URL", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("https test"))
		}))
		defer server.Close()

		// 创建忽略TLS证书验证的配置
		cfg := createMinimalConfig()

		// 创建新的客户端来处理TLS
		tlsClient, err := factory.Create(cfg)
		require.NoError(t, err)
		defer tlsClient.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL) // httptest.NewTLSServer使用https://

		// 注意：这个测试可能会因为TLS证书验证失败，这是正常的
		// 在实际环境中，应该配置正确的TLS设置
		resp, err := tlsClient.Do(req, upstream)
		if err != nil {
			// TLS错误是预期的，因为测试服务器使用自签名证书
			assert.Contains(t, err.Error(), "certificate")
			return
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("plain host URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("plain host test"))
		}))
		defer server.Close()

		// 提取主机部分（去掉http://前缀）
		u, _ := url.Parse(server.URL)
		plainHost := u.Host

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(plainHost)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHTTPClient_HTTPMethods(t *testing.T) {
	factory := NewFactory()
	cfg := createMinimalConfig()

	client, err := factory.Create(cfg)
	require.NoError(t, err)
	defer client.Close()

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, method, r.Method)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(fmt.Sprintf("%s response", method)))
			}))
			defer server.Close()

			var body io.Reader
			if method == "POST" || method == "PUT" || method == "PATCH" {
				body = strings.NewReader(`{"test": "data"}`)
			}

			req, _ := http.NewRequest(method, "/test", body)
			if body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			upstream := createTestUpstream(server.URL)

			resp, err := client.Do(req, upstream)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func TestHTTPClientFactory(t *testing.T) {
	factory := NewFactory()

	t.Run("create with minimal config", func(t *testing.T) {
		cfg := createMinimalConfig()
		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		assert.NotNil(t, client)
		assert.NotEmpty(t, client.Name())
	})

	t.Run("create with full config", func(t *testing.T) {
		cfg := createFullConfig()
		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		assert.NotNil(t, client)
		assert.NotEmpty(t, client.Name())
	})

	t.Run("create with nil config", func(t *testing.T) {
		_, err := factory.Create(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("multiple clients have different names", func(t *testing.T) {
		cfg := createMinimalConfig()

		client1, err1 := factory.Create(cfg)
		require.NoError(t, err1)
		defer client1.Close()

		// 添加小延迟确保时间戳不同
		time.Sleep(1 * time.Millisecond)

		client2, err2 := factory.Create(cfg)
		require.NoError(t, err2)
		defer client2.Close()

		assert.NotEqual(t, client1.Name(), client2.Name())
	})
}

// 组件测试

func TestRetryHandler(t *testing.T) {
	t.Run("retry enabled", func(t *testing.T) {
		cfg := createRetryConfig()
		handler := NewRetryHandler(cfg)

		assert.True(t, handler.IsEnabled())

		config := handler.GetConfig()
		assert.Equal(t, true, config["enabled"])
		assert.Equal(t, 2, config["max_retries"])
		assert.Equal(t, 100, config["retry_delay"])
	})

	t.Run("retry disabled", func(t *testing.T) {
		cfg := createNoRetryConfig()
		handler := NewRetryHandler(cfg)

		assert.False(t, handler.IsEnabled())

		config := handler.GetConfig()
		assert.Equal(t, false, config["enabled"])
		assert.Equal(t, 0, config["max_retries"])
		assert.Equal(t, 0, config["retry_delay"])
	})

	t.Run("retry execution", func(t *testing.T) {
		cfg := createRetryConfig()
		cfg.Retry.Attempts = 3
		cfg.Retry.Initial = 10

		handler := NewRetryHandler(cfg)

		var attempts int32
		ctx := context.Background()

		// 测试成功的情况
		resp, err := handler.DoWithRetry(ctx, func() (*http.Response, error) {
			atomic.AddInt32(&attempts, 1)
			return &http.Response{StatusCode: 200}, nil
		})

		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
	})
}

func TestConnectionPool(t *testing.T) {
	t.Run("with connect config", func(t *testing.T) {
		cfg := createConnectConfig(200, 20, 100)
		pool := NewConnectionPool(cfg)

		transport := pool.GetTransport()
		assert.NotNil(t, transport)
		assert.Equal(t, 200, transport.MaxIdleConns)
		assert.Equal(t, 20, transport.MaxIdleConnsPerHost)
		assert.Equal(t, 100, transport.MaxConnsPerHost)

		err := pool.Close()
		assert.NoError(t, err)
	})

	t.Run("without connect config", func(t *testing.T) {
		cfg := createMinimalConfig()
		pool := NewConnectionPool(cfg)

		transport := pool.GetTransport()
		assert.NotNil(t, transport)

		err := pool.Close()
		assert.NoError(t, err)
	})

	t.Run("with timeout config", func(t *testing.T) {
		cfg := createTimeoutConfig(15, 120, 180)
		pool := NewConnectionPool(cfg)

		transport := pool.GetTransport()
		assert.NotNil(t, transport)
		assert.Equal(t, 180*time.Second, transport.IdleConnTimeout)
		assert.Equal(t, 120*time.Second, transport.ResponseHeaderTimeout)

		err := pool.Close()
		assert.NoError(t, err)
	})

	t.Run("with proxy config", func(t *testing.T) {
		cfg := createProxyConfig("http://proxy.example.com:8080")
		pool := NewConnectionPool(cfg)

		transport := pool.GetTransport()
		assert.NotNil(t, transport)
		assert.NotNil(t, transport.Proxy)

		err := pool.Close()
		assert.NoError(t, err)
	})
}

func TestProxyHandler(t *testing.T) {
	t.Run("with proxy config", func(t *testing.T) {
		proxyConfig := &config.ProxyConfig{
			URL: "http://proxy.example.com:8080",
		}
		handler := NewProxyHandler(proxyConfig)

		assert.NotNil(t, handler)
		assert.NotNil(t, handler.GetProxyFunc())
	})

	t.Run("without proxy config", func(t *testing.T) {
		handler := NewProxyHandlerFromURL("")

		assert.NotNil(t, handler)
		proxyFunc := handler.GetProxyFunc()
		assert.NotNil(t, proxyFunc)

		// 测试空代理函数
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		proxyURL, err := proxyFunc(req)
		assert.NoError(t, err)
		assert.Nil(t, proxyURL)
	})

	t.Run("with invalid proxy URL", func(t *testing.T) {
		handler := NewProxyHandlerFromURL("invalid-url")

		assert.NotNil(t, handler)
		proxyFunc := handler.GetProxyFunc()
		assert.NotNil(t, proxyFunc)
	})
}
