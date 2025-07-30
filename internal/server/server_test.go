package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/shengyanli1982/llmproxy-go/internal/balance"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForwardService_Initialize(t *testing.T) {
	logger := logr.Discard()

	// Create test configurations
	forwardConfig := &config.ForwardConfig{
		Name:         "test-forward",
		Address:      "127.0.0.1",
		Port:         8080,
		DefaultGroup: "test-group",
		RateLimit: &config.RateLimitConfig{
			PerSecond: 100.0,
			Burst:     200,
		},
		Timeout: &config.TimeoutConfig{
			Idle:  30,
			Read:  15,
			Write: 15,
		},
	}

	globalConfig := &config.Config{
		UpstreamGroups: []config.UpstreamGroupConfig{
			{
				Name: "test-group",
				Balance: &config.BalanceConfig{
					Strategy: "roundrobin",
				},
				Upstreams: []config.UpstreamRefConfig{
					{Name: "upstream1", Weight: 1},
					{Name: "upstream2", Weight: 2},
				},
			},
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name: "upstream1",
				URL:  "http://example1.com",
				Auth: &config.AuthConfig{
					Type:  "bearer",
					Token: "token1",
				},
			},
			{
				Name: "upstream2",
				URL:  "http://example2.com",
				Auth: &config.AuthConfig{
					Type: "none",
				},
			},
		},
	}

	service := NewForwardServices()
	err := service.Initialize(forwardConfig, globalConfig, &logger)

	require.NoError(t, err)
	assert.NotNil(t, service.loadBalancer)
	assert.NotNil(t, service.httpClient)
	assert.NotNil(t, service.rateLimitMW)
	assert.Equal(t, 2, len(service.upstreams))
	assert.Equal(t, 2, len(service.upstreamMap))
}

func TestForwardService_Initialize_MissingDefaultGroup(t *testing.T) {
	logger := logr.Discard()

	forwardConfig := &config.ForwardConfig{
		Name:         "test-forward",
		DefaultGroup: "nonexistent-group",
		Timeout: &config.TimeoutConfig{
			Idle:  30,
			Read:  15,
			Write: 15,
		},
	}

	globalConfig := &config.Config{
		UpstreamGroups: []config.UpstreamGroupConfig{},
		Upstreams:      []config.UpstreamConfig{},
	}

	service := NewForwardServices()
	err := service.Initialize(forwardConfig, globalConfig, &logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default upstream group 'nonexistent-group' not found")
}

func TestForwardService_Initialize_MissingUpstream(t *testing.T) {
	logger := logr.Discard()

	forwardConfig := &config.ForwardConfig{
		Name:         "test-forward",
		DefaultGroup: "test-group",
		Timeout: &config.TimeoutConfig{
			Idle:  30,
			Read:  15,
			Write: 15,
		},
	}

	globalConfig := &config.Config{
		UpstreamGroups: []config.UpstreamGroupConfig{
			{
				Name: "test-group",
				Upstreams: []config.UpstreamRefConfig{
					{Name: "nonexistent-upstream", Weight: 1},
				},
			},
		},
		Upstreams: []config.UpstreamConfig{},
	}

	service := NewForwardServices()
	err := service.Initialize(forwardConfig, globalConfig, &logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upstream 'nonexistent-upstream' not found")
}

func TestForwardService_ServiceLifecycle(t *testing.T) {
	service := NewForwardServices()

	// Initially should not be running
	assert.False(t, service.IsRunning())

	// Start service
	service.Run()
	assert.True(t, service.IsRunning())

	// Stop service
	service.Stop()
	assert.False(t, service.IsRunning())

	// Should be able to stop multiple times
	service.Stop()
	assert.False(t, service.IsRunning())
}

// TestServer_Integration 真正的集成测试，验证整个系统的端到端功能
// 注意：由于 orbit 库的 Prometheus 指标注册问题，这个测试只能运行一次
// 在实际项目中，建议将这些测试分离到不同的测试文件中，或者使用构建标签来控制
func TestServer_Integration(t *testing.T) {
	// 由于 Prometheus 指标重复注册的问题，我们只运行一个核心的集成测试
	// 这个测试涵盖了最重要的功能：基本代理转发

	t.Log("Starting integration test: verifying basic proxy forwarding functionality")

	// 创建上游服务器
	upstream := createTestUpstream("upstream1", 0, 0)
	defer upstream.server.Close()

	// 创建代理服务器
	server, cleanup := createTestServer(t, []*testUpstreamServer{upstream}, nil)
	defer cleanup()

	// 获取代理服务器的地址
	forwardServer := server.GetForwardServer("test-forward")
	require.NotNil(t, forwardServer)

	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", forwardServer.GetConfig().Port)
	t.Logf("Proxy server address: %s", proxyURL)

	// 测试基本的 GET 请求
	t.Run("Basic GET Request", func(t *testing.T) {
		resp, err := http.Get(proxyURL + "/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		// 验证响应
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		assert.Equal(t, "upstream1", resp.Header.Get("X-Upstream-Name"))

		// 验证响应内容
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		responseBody := string(body[:n])
		assert.Contains(t, responseBody, "response from upstream1")
		assert.Contains(t, responseBody, "request_count")

		t.Logf("Response content: %s", responseBody)
	})

	// 测试 POST 请求
	t.Run("POST Request", func(t *testing.T) {
		body := bytes.NewBufferString(`{"test": "data"}`)
		resp, err := http.Post(proxyURL+"/api/test", "application/json", body)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "upstream1", resp.Header.Get("X-Upstream-Name"))
	})

	// 测试自定义头部
	t.Run("Custom Headers", func(t *testing.T) {
		client := &http.Client{}
		req, err := http.NewRequest("GET", proxyURL+"/test", nil)
		require.NoError(t, err)

		req.Header.Set("X-Custom-Header", "test-value")
		req.Header.Set("Authorization", "Bearer token123")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "upstream1", resp.Header.Get("X-Upstream-Name"))
	})

	// 验证上游服务器收到了所有请求
	assert.Equal(t, 3, upstream.requests, "上游服务器应该收到3个请求")

	t.Log("集成测试完成：基本代理转发功能正常")
}

func TestForwardServer_Integration(t *testing.T) {
	logger := logr.Discard()

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back the request method and path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"method": "` + r.Method + `", "path": "` + r.URL.Path + `"}`))
	}))
	defer upstreamServer.Close()

	forwardConfig := &config.ForwardConfig{
		Name:         "integration-test",
		Address:      "127.0.0.1",
		Port:         0, // Use random port
		DefaultGroup: "test-group",
		Timeout: &config.TimeoutConfig{
			Idle:  30,
			Read:  15,
			Write: 15,
		},
	}

	globalConfig := &config.Config{
		UpstreamGroups: []config.UpstreamGroupConfig{
			{
				Name: "test-group",
				Balance: &config.BalanceConfig{
					Strategy: "roundrobin",
				},
				Upstreams: []config.UpstreamRefConfig{
					{Name: "test-upstream", Weight: 1},
				},
			},
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name: "test-upstream",
				URL:  upstreamServer.URL,
			},
		},
	}

	forwardServer := NewForwardServer(true, &logger, forwardConfig, globalConfig)
	require.NotNil(t, forwardServer)

	// Test server properties
	assert.Equal(t, "integration-test", forwardServer.GetConfig().Name)
	assert.Contains(t, forwardServer.GetEndpoint(), "127.0.0.1:")
	assert.False(t, forwardServer.IsRunning())

	// Test service initialization
	service := forwardServer.GetService()
	assert.NotNil(t, service)
}

func TestAdminService_Integration(t *testing.T) {
	logger := logr.Discard()

	// Create a minimal server setup
	httpServerConfig := &config.HTTPServerConfig{
		Admin: config.AdminConfig{
			Address: "127.0.0.1",
			Port:    0,
			Timeout: &config.TimeoutConfig{
				Read:  30,
				Write: 30,
				Idle:  60,
			},
		},
	}

	globalConfig := &config.Config{
		HTTPServer: *httpServerConfig,
	}

	server := NewServer(true, &logger, httpServerConfig, globalConfig)

	// Test server properties
	assert.NotNil(t, server)
	assert.NotNil(t, server.adminServer)

	// Test admin service properties through the server's admin server
	adminService := server.adminServer.service
	assert.NotNil(t, adminService)
	assert.False(t, adminService.IsRunning())

	// Test service lifecycle
	adminService.Run()
	assert.True(t, adminService.IsRunning())

	adminService.Stop()
	assert.False(t, adminService.IsRunning())
}

func TestForwardService_CreateProxyRequest(t *testing.T) {
	service := NewForwardServices()

	// Create original request
	body := bytes.NewReader([]byte(`{"test": "data"}`))
	originalReq, err := http.NewRequest("POST", "http://original.com/api/test", body)
	require.NoError(t, err)
	originalReq.Header.Set("Content-Type", "application/json")
	originalReq.Header.Set("Authorization", "Bearer original-token")
	originalReq.RemoteAddr = "192.168.1.100:12345" // 设置测试用的客户端地址

	// Create proxy request
	proxyReq, err := service.createProxyRequest(originalReq)
	require.NoError(t, err)

	// Verify proxy request properties
	assert.Equal(t, "POST", proxyReq.Method)
	assert.Equal(t, "http://original.com/api/test", proxyReq.URL.String())
	assert.Equal(t, "application/json", proxyReq.Header.Get("Content-Type"))
	assert.Equal(t, "Bearer original-token", proxyReq.Header.Get("Authorization"))

	// Verify forwarding headers are set
	assert.NotEmpty(t, proxyReq.Header.Get("X-Forwarded-For"))
	assert.NotEmpty(t, proxyReq.Header.Get("X-Forwarded-Proto"))
	assert.NotEmpty(t, proxyReq.Header.Get("X-Forwarded-Host"))

	// Verify body is properly copied
	if proxyReq.Body != nil {
		bodyBytes, err := io.ReadAll(proxyReq.Body)
		require.NoError(t, err)
		assert.Equal(t, `{"test": "data"}`, string(bodyBytes))
	}
}

func TestForwardService_CreateProxyRequest_LargeBody(t *testing.T) {
	service := NewForwardServices()

	// Create a large body that exceeds the limit
	largeData := make([]byte, MaxRequestBodySize+1000) // 32MB + 1000 bytes
	for i := range largeData {
		largeData[i] = 'A'
	}

	body := bytes.NewReader(largeData)
	originalReq, err := http.NewRequest("POST", "http://original.com/api/test", body)
	require.NoError(t, err)
	originalReq.RemoteAddr = "192.168.1.100:12345"

	// Should return error for large body
	_, err = service.createProxyRequest(originalReq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request body too large")
}

func TestForwardService_CreateProxyRequest_EmptyBody(t *testing.T) {
	service := NewForwardServices()

	// Create request with no body
	originalReq, err := http.NewRequest("GET", "http://original.com/api/test", nil)
	require.NoError(t, err)
	originalReq.RemoteAddr = "192.168.1.100:12345"

	// Should work fine with no body
	proxyReq, err := service.createProxyRequest(originalReq)
	require.NoError(t, err)
	assert.Nil(t, proxyReq.Body)
}

func TestForwardService_ErrorHandling(t *testing.T) {
	service := NewForwardServices()

	// Initialize service with minimal config to avoid nil pointer
	cfg := &config.ForwardConfig{
		DefaultGroup: "test-group",
	}
	globalConfig := &config.Config{
		UpstreamGroups: []config.UpstreamGroupConfig{
			{
				Name: "test-group",
				Upstreams: []config.UpstreamRefConfig{
					{Name: "test-upstream", Weight: 1},
				},
			},
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name: "test-upstream",
				URL:  "http://test.com",
			},
		},
	}
	logger := logr.Discard()

	err := service.Initialize(cfg, globalConfig, &logger)
	require.NoError(t, err)

	// Test with no upstreams
	ctx := context.Background()
	emptyUpstreams := []balance.Upstream{}

	_, err = service.loadBalancer.Select(ctx, emptyUpstreams)
	assert.Error(t, err)
}

func TestForwardService_StreamingResponseDetection(t *testing.T) {
	service := NewForwardServices()

	tests := []struct {
		name        string
		contentType string
		encoding    string
		expected    bool
	}{
		{
			name:        "text/event-stream",
			contentType: "text/event-stream",
			expected:    true,
		},
		{
			name:        "application/stream+json",
			contentType: "application/stream+json",
			expected:    true,
		},
		{
			name:        "chunked encoding",
			contentType: "application/json",
			encoding:    "chunked",
			expected:    true,
		},
		{
			name:        "regular response",
			contentType: "application/json",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header: make(http.Header),
			}

			if tt.contentType != "" {
				resp.Header.Set("Content-Type", tt.contentType)
			}
			if tt.encoding != "" {
				resp.Header.Set("Transfer-Encoding", tt.encoding)
			}

			result := service.isStreamingResponse(resp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestForwardService_ClientIP_Detection(t *testing.T) {
	service := NewForwardServices()

	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "X-Forwarded-For priority",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 10.0.0.1",
				"X-Real-IP":       "203.0.113.2",
			},
			expected: "203.0.113.1",
		},
		{
			name:       "X-Real-IP fallback",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.2",
			},
			expected: "203.0.113.2",
		},
		{
			name:       "RemoteAddr fallback",
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{},
			expected:   "192.168.1.1",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			headers:    map[string]string{},
			expected:   "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: tt.remoteAddr,
				Header:     make(http.Header),
			}

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := service.getClientIP(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestForwardService_Scheme_Detection(t *testing.T) {
	service := NewForwardServices()

	tests := []struct {
		name     string
		tls      bool
		headers  map[string]string
		expected string
	}{
		{
			name:     "HTTPS with TLS",
			tls:      true,
			expected: "https",
		},
		{
			name: "X-Forwarded-Proto header",
			headers: map[string]string{
				"X-Forwarded-Proto": "https",
			},
			expected: "https",
		},
		{
			name:     "HTTP default",
			expected: "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: make(http.Header),
			}

			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := service.getScheme(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Benchmarks
func BenchmarkForwardService_CreateProxyRequest(b *testing.B) {
	service := NewForwardServices()

	originalReq, _ := http.NewRequest("GET", "http://example.com/test", nil)
	originalReq.Header.Set("User-Agent", "test-agent")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.createProxyRequest(originalReq)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkForwardService_GetClientIP(b *testing.B) {
	service := NewForwardServices()
	req := &http.Request{
		RemoteAddr: "192.168.1.1:12345",
		Header:     make(http.Header),
	}
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.getClientIP(req)
	}
}

// ============================================================================
// 集成测试辅助函数
// ============================================================================

// testUpstreamServer 测试用的上游服务器配置
type testUpstreamServer struct {
	server   *httptest.Server
	name     string
	requests int
	delay    time.Duration
	failRate float64 // 0.0-1.0, 失败率
}

// createTestUpstream 创建测试用的上游服务器
func createTestUpstream(name string, delay time.Duration, failRate float64) *testUpstreamServer {
	upstream := &testUpstreamServer{
		name:     name,
		delay:    delay,
		failRate: failRate,
	}

	upstream.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream.requests++

		// 模拟延迟
		if upstream.delay > 0 {
			time.Sleep(upstream.delay)
		}

		// 模拟失败
		if upstream.failRate > 0 && rand.Float64() < upstream.failRate {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "upstream error"}`))
			return
		}

		// 正常响应
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Upstream-Name", upstream.name)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"message": "response from %s", "request_count": %d}`, upstream.name, upstream.requests)))
	}))

	return upstream
}

// createTestServer 创建测试用的代理服务器
func createTestServer(t *testing.T, upstreams []*testUpstreamServer, rateLimitConfig *config.RateLimitConfig) (*Server, func()) {
	logger := logr.Discard()

	// 构建上游配置
	var upstreamConfigs []config.UpstreamConfig
	var upstreamRefs []config.UpstreamRefConfig

	for _, upstream := range upstreams {
		upstreamConfigs = append(upstreamConfigs, config.UpstreamConfig{
			Name: upstream.name,
			URL:  upstream.server.URL,
		})
		upstreamRefs = append(upstreamRefs, config.UpstreamRefConfig{
			Name:   upstream.name,
			Weight: 1,
		})
	}

	// 创建服务器配置
	httpServerConfig := &config.HTTPServerConfig{
		Forwards: []config.ForwardConfig{
			{
				Name:         "test-forward",
				Address:      "127.0.0.1",
				Port:         0, // 使用随机端口
				DefaultGroup: "test-group",
				RateLimit:    rateLimitConfig,
				Timeout: &config.TimeoutConfig{
					Idle:  30,
					Read:  15,
					Write: 15,
				},
			},
		},
		Admin: config.AdminConfig{
			Address: "127.0.0.1",
			Port:    0, // 使用随机端口
			Timeout: &config.TimeoutConfig{
				Idle:  30,
				Read:  15,
				Write: 15,
			},
		},
	}

	globalConfig := &config.Config{
		HTTPServer: *httpServerConfig,
		UpstreamGroups: []config.UpstreamGroupConfig{
			{
				Name: "test-group",
				Balance: &config.BalanceConfig{
					Strategy: "roundrobin",
				},
				Upstreams: upstreamRefs,
			},
		},
		Upstreams: upstreamConfigs,
	}

	// 创建服务器
	server := NewServer(true, &logger, httpServerConfig, globalConfig)
	require.NotNil(t, server)

	// 启动服务器
	server.Start()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 返回清理函数
	cleanup := func() {
		server.Stop()
		for _, upstream := range upstreams {
			upstream.server.Close()
		}
	}

	return server, cleanup
}
