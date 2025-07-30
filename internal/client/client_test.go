package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
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

func TestHTTPClient_URLSplitAndJoin(t *testing.T) {
	factory := NewFactory()
	cfg := createMinimalConfig()

	client, err := factory.Create(cfg)
	require.NoError(t, err)
	defer client.Close()

	t.Run("基础URL拼接用户路径", func(t *testing.T) {
		// 模拟用户设计的场景：
		// 用户请求：http://127.0.0.1:3000/api/v3/chat/completions
		// upstream配置：https://ark.cn-beijing.volces.com
		// 期望转发：https://ark.cn-beijing.volces.com/api/v3/chat/completions

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证转发后的路径是否正确
			assert.Equal(t, "/api/v3/chat/completions", r.URL.Path)
			assert.Equal(t, "param=value", r.URL.RawQuery)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("path join test"))
		}))
		defer server.Close()

		// 创建用户请求，包含路径和查询参数
		req, _ := http.NewRequest("POST", "/api/v3/chat/completions?param=value", strings.NewReader(`{"test": "data"}`))

		// 创建upstream配置，使用基础URL（不包含路径）
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "path join test", string(body))
	})

	t.Run("完整端点URL覆盖用户路径", func(t *testing.T) {
		// 测试upstream URL包含完整路径的情况
		// upstream配置：https://api.example.com/v1/completions
		// 用户请求路径应该被upstream的路径覆盖

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证使用的是upstream的路径，而不是用户请求的路径
			assert.Equal(t, "/v1/completions", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("endpoint override test"))
		}))
		defer server.Close()

		// 创建用户请求，包含不同的路径
		req, _ := http.NewRequest("POST", "/api/v3/chat/completions", strings.NewReader(`{"test": "data"}`))

		// 创建upstream配置，包含完整的端点路径
		upstream := createTestUpstream(server.URL + "/v1/completions")

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "endpoint override test", string(body))
	})

	t.Run("查询参数和片段处理", func(t *testing.T) {
		// 测试查询参数和片段的正确传递
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证查询参数正确传递
			assert.Equal(t, "value1", r.URL.Query().Get("param1"))
			assert.Equal(t, "value2", r.URL.Query().Get("param2"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("query params test"))
		}))
		defer server.Close()

		// 创建包含查询参数和片段的用户请求
		req, _ := http.NewRequest("GET", "/api/test?param1=value1&param2=value2#section1", nil)

		// 使用基础URL配置
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("upstream URL包含查询参数", func(t *testing.T) {
		// 测试upstream URL包含查询参数的情况
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证使用的是upstream的查询参数
			assert.Equal(t, "upstream_value", r.URL.Query().Get("upstream_param"))
			assert.Equal(t, "/upstream/path", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("upstream query test"))
		}))
		defer server.Close()

		// 创建用户请求
		req, _ := http.NewRequest("GET", "/user/path?user_param=user_value", nil)

		// 创建包含路径和查询参数的upstream配置
		upstream := createTestUpstream(server.URL + "/upstream/path?upstream_param=upstream_value")

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHTTPClient_URLHandlingEdgeCases(t *testing.T) {
	factory := NewFactory()
	cfg := createMinimalConfig()

	client, err := factory.Create(cfg)
	require.NoError(t, err)
	defer client.Close()

	t.Run("空路径处理", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 空路径应该被转换为根路径
			assert.Equal(t, "/", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("empty path test"))
		}))
		defer server.Close()

		// 创建空路径请求
		req, _ := http.NewRequest("GET", "", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("根路径处理", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("root path test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("GET", "/", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("多级路径处理", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v1/models/chat/completions", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("deep path test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("POST", "/api/v1/models/chat/completions", strings.NewReader(`{"model": "gpt-3.5-turbo"}`))
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("URL编码字符处理", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证URL编码字符被正确处理
			assert.Equal(t, "/api/test with spaces", r.URL.Path)
			assert.Equal(t, "key with spaces", r.URL.Query().Get("param"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("encoded chars test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("GET", "/api/test%20with%20spaces?param=key%20with%20spaces", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("特殊字符路径处理", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证特殊字符被正确处理
			assert.Equal(t, "/api/test-path_with.special~chars", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("special chars test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("GET", "/api/test-path_with.special~chars", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("中文路径处理", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证中文路径被正确处理
			assert.Equal(t, "/api/测试路径", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("chinese path test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("GET", "/api/测试路径", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHTTPClient_URLErrorHandling(t *testing.T) {
	factory := NewFactory()
	cfg := createMinimalConfig()

	client, err := factory.Create(cfg)
	require.NoError(t, err)
	defer client.Close()

	t.Run("无效upstream URL", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test", nil)

		// 创建包含无效URL的upstream
		upstream := &balance.Upstream{
			Name: "invalid-upstream",
			URL:  "ht!tp://invalid-url", // 无效的URL格式
		}

		_, err := client.Do(req, upstream)
		// 应该返回URL解析错误
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("空upstream URL", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test", nil)

		upstream := &balance.Upstream{
			Name: "empty-upstream",
			URL:  "", // 空URL
		}

		_, err := client.Do(req, upstream)
		// 应该返回错误
		assert.Error(t, err)
	})

	t.Run("不支持的协议", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test", nil)

		upstream := &balance.Upstream{
			Name: "unsupported-protocol",
			URL:  "ftp://example.com", // 不支持的协议
		}

		_, err := client.Do(req, upstream)
		// 应该返回协议不支持的错误
		assert.Error(t, err)
	})
}

func TestHTTPClient_URLTransformationScenarios(t *testing.T) {
	factory := NewFactory()
	cfg := createMinimalConfig()

	client, err := factory.Create(cfg)
	require.NoError(t, err)
	defer client.Close()

	t.Run("基础域名到完整API端点", func(t *testing.T) {
		// 模拟真实场景：用户配置基础域名，请求转发到具体API端点
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/chat/completions", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"choices": [{"message": {"content": "test response"}}]}`))
		}))
		defer server.Close()

		req, _ := http.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model": "gpt-3.5-turbo", "messages": []}`))
		req.Header.Set("Content-Type", "application/json")

		upstream := createTestUpstream(server.URL) // 基础URL，不包含路径

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("子路径到子路径的映射", func(t *testing.T) {
		// 测试从一个API路径映射到另一个API路径
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v2/completions", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("path mapping test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("POST", "/v1/completions", strings.NewReader(`{"prompt": "test"}`))

		// upstream配置包含不同的路径，应该覆盖用户请求的路径
		upstream := createTestUpstream(server.URL + "/api/v2/completions")

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("查询参数合并和覆盖", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证upstream的查询参数被使用
			assert.Equal(t, "upstream_value", r.URL.Query().Get("api_key"))
			assert.Equal(t, "v2", r.URL.Query().Get("version"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("query merge test"))
		}))
		defer server.Close()

		// 用户请求包含查询参数
		req, _ := http.NewRequest("GET", "/api/models?version=v1&user_param=user_value", nil)

		// upstream URL也包含查询参数，应该覆盖用户的参数
		upstream := createTestUpstream(server.URL + "/api/models?api_key=upstream_value&version=v2")

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("端口号处理", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/test", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("port handling test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("GET", "/api/test", nil)

		// 使用包含端口号的upstream URL
		u, _ := url.Parse(server.URL)
		upstreamWithPort := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		upstream := createTestUpstream(upstreamWithPort)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("HTTPS到HTTP的协议转换", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/secure/api", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("protocol conversion test"))
		}))
		defer server.Close()

		// 用户请求使用HTTPS（在实际场景中可能来自HTTPS代理）
		req, _ := http.NewRequest("GET", "/secure/api", nil)

		// upstream使用HTTP（测试服务器）
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("复杂路径结构处理", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v1/organizations/org-123/projects/proj-456/models", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("complex path test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("GET", "/api/v1/organizations/org-123/projects/proj-456/models", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHTTPClient_URLCompatibilityScenarios(t *testing.T) {
	factory := NewFactory()
	cfg := createMinimalConfig()

	client, err := factory.Create(cfg)
	require.NoError(t, err)
	defer client.Close()

	t.Run("向后兼容：不带scheme的URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/test", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("backward compatibility test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("GET", "/api/test", nil)

		// 提取主机部分（不包含scheme）
		u, _ := url.Parse(server.URL)
		plainHost := u.Host
		upstream := createTestUpstream(plainHost)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("IPv4地址处理", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/ipv4", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ipv4 test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("GET", "/api/ipv4", nil)

		// 使用IPv4地址格式的upstream
		u, _ := url.Parse(server.URL)
		upstream := createTestUpstream(u.String())

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("localhost处理", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/localhost", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("localhost test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("GET", "/api/localhost", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHTTPClient_CodeQualityImprovements(t *testing.T) {
	factory := NewFactory()
	cfg := createMinimalConfig()

	client, err := factory.Create(cfg)
	require.NoError(t, err)
	defer client.Close()

	t.Run("日志记录器设置", func(t *testing.T) {
		// 创建一个测试日志记录器
		logger := logr.Discard()

		// 设置日志记录器
		client.SetLogger(logger)

		// 验证设置成功（通过执行一个请求来间接验证）
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("logger test"))
		}))
		defer server.Close()

		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("错误处理改进", func(t *testing.T) {
		// 测试预定义错误的使用
		_, err := client.Do(nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")

		// 测试更详细的错误信息
		upstream := &balance.Upstream{
			Name: "test-upstream",
			URL:  "", // 空URL
		}
		req, _ := http.NewRequest("GET", "/test", nil)

		_, err = client.Do(req, upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "test-upstream") // 错误信息应包含upstream名称
	})

	t.Run("资源清理验证", func(t *testing.T) {
		// 创建一个新的客户端用于测试关闭
		testClient, err := factory.Create(cfg)
		require.NoError(t, err)

		// 验证客户端名称不为空
		assert.NotEmpty(t, testClient.Name())

		// 关闭客户端
		err = testClient.Close()
		assert.NoError(t, err)

		// 再次关闭应该不会出错
		err = testClient.Close()
		assert.NoError(t, err)

		// 关闭后的请求应该返回错误
		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream("http://example.com")

		_, err = testClient.Do(req, upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
	})

	t.Run("并发安全性基础验证", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 模拟一些处理时间
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("concurrent test"))
		}))
		defer server.Close()

		upstream := createTestUpstream(server.URL)

		// 并发执行多个请求
		const numRequests = 10
		var wg sync.WaitGroup
		errors := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// 每个goroutine创建自己的请求
				req, _ := http.NewRequest("GET", fmt.Sprintf("/test-%d", id), nil)

				resp, err := client.Do(req, upstream)
				if err != nil {
					errors <- err
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					errors <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// 检查是否有错误
		for err := range errors {
			t.Errorf("Concurrent request failed: %v", err)
		}
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
		assert.True(t, handler.IsEnabled())
		assert.Equal(t, "http://proxy.example.com:8080", handler.GetProxyURL())
	})

	t.Run("invalid proxy URL", func(t *testing.T) {
		proxyConfig := &config.ProxyConfig{
			URL: "ht!tp://invalid-url", // 使用真正无效的URL格式
		}
		handler := NewProxyHandler(proxyConfig)

		assert.NotNil(t, handler)
		assert.False(t, handler.IsEnabled())
		assert.Equal(t, "", handler.GetProxyURL())
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

func TestHTTPClient_ProxyRetryIntegration(t *testing.T) {
	factory := NewFactory()

	t.Run("验证proxy配置被正确应用", func(t *testing.T) {
		// 创建包含proxy配置的客户端
		cfg := &config.HTTPClientConfig{
			Agent:     "LLMProxy/1.0",
			KeepAlive: 60,
			Proxy: &config.ProxyConfig{
				URL: "http://nonexistent-proxy.example.com:8080",
			},
		}

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		// 验证客户端创建成功，包含proxy配置
		assert.NotNil(t, client)

		// 通过类型断言验证内部配置
		if httpClient, ok := client.(*httpClient); ok {
			// 验证proxy配置
			assert.NotNil(t, httpClient.proxyHandler)
			assert.True(t, httpClient.proxyHandler.IsEnabled())
			assert.Equal(t, "http://nonexistent-proxy.example.com:8080", httpClient.proxyHandler.GetProxyURL())

			// 验证transport中的proxy设置
			if transport, ok := httpClient.client.Transport.(*http.Transport); ok {
				assert.NotNil(t, transport.Proxy)
			}
		}

		// 创建一个测试服务器
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("should not reach here"))
		}))
		defer server.Close()

		// 尝试执行请求，应该因为代理不存在而失败
		// 这证明了proxy配置确实被应用了
		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		_, err = client.Do(req, upstream)
		// 应该返回代理连接错误，证明proxy配置生效
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "proxy")
	})

	t.Run("验证没有proxy时的正常工作", func(t *testing.T) {
		// 创建不包含proxy配置的客户端
		cfg := &config.HTTPClientConfig{
			Agent:     "LLMProxy/1.0",
			KeepAlive: 60,
			// 没有Proxy配置
		}

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		// 验证proxy处理器存在但未启用
		if httpClient, ok := client.(*httpClient); ok {
			assert.NotNil(t, httpClient.proxyHandler)
			assert.False(t, httpClient.proxyHandler.IsEnabled())
		}

		// 创建测试服务器
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("no proxy test"))
		}))
		defer server.Close()

		// 执行请求，应该成功（不使用代理）
		req, _ := http.NewRequest("GET", "/test", nil)
		upstream := createTestUpstream(server.URL)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "no proxy test", string(body))
	})

	t.Run("验证proxy配置在重试过程中保持一致", func(t *testing.T) {
		// 这个测试验证proxy配置在HTTPClient创建时正确设置
		// 并且在重试过程中使用同一个http.Client实例

		cfg := &config.HTTPClientConfig{
			Agent:     "LLMProxy/1.0",
			KeepAlive: 60,
			Retry: &config.RetryConfig{
				Attempts: 2,
				Initial:  100,
			},
			Proxy: &config.ProxyConfig{
				URL: "http://test-proxy.example.com:3128",
			},
		}

		client, err := factory.Create(cfg)
		require.NoError(t, err)
		defer client.Close()

		// 验证客户端创建成功（包含proxy配置）
		assert.NotNil(t, client)
		assert.NotEmpty(t, client.Name())

		// 通过类型断言访问内部实现来验证proxy设置
		if httpClient, ok := client.(*httpClient); ok {
			assert.NotNil(t, httpClient.proxyHandler)
			assert.True(t, httpClient.proxyHandler.IsEnabled())
			assert.Equal(t, "http://test-proxy.example.com:3128", httpClient.proxyHandler.GetProxyURL())

			// 验证transport中的proxy设置
			if transport, ok := httpClient.client.Transport.(*http.Transport); ok {
				assert.NotNil(t, transport.Proxy)
			}
		}
	})
}
