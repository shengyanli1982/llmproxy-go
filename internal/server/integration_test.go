package server

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
)

// TestProxyServer_EndToEnd 端到端集成测试
// 这个测试验证了整个代理服务器的核心功能
func TestProxyServer_EndToEnd(t *testing.T) {
	t.Log("开始端到端集成测试")

	// 创建上游服务器
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Upstream-Name", "test-upstream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"message": "Hello from upstream", "method": "%s", "path": "%s"}`, r.Method, r.URL.Path)))
	}))
	defer upstream.Close()

	logger := logr.Discard()

	// 创建服务器配置
	httpServerConfig := &config.HTTPServerConfig{
		Forwards: []config.ForwardConfig{
			{
				Name:         "test-forward",
				Address:      "127.0.0.1",
				Port:         0, // 使用随机端口
				DefaultGroup: "test-group",
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
				Upstreams: []config.UpstreamRefConfig{
					{Name: "test-upstream", Weight: 1},
				},
			},
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name: "test-upstream",
				URL:  upstream.URL,
			},
		},
	}

	// 创建服务器
	server := NewServer(true, &logger, httpServerConfig, globalConfig)
	require.NotNil(t, server)

	// 启动服务器
	server.Start()
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(200 * time.Millisecond)

	// 获取代理服务器的地址
	forwardServer := server.GetForwardServer("test-forward")
	require.NotNil(t, forwardServer)

	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", forwardServer.GetConfig().Port)
	t.Logf("代理服务器地址: %s", proxyURL)

	// 测试基本的 GET 请求
	t.Run("Basic GET Request", func(t *testing.T) {
		resp, err := http.Get(proxyURL + "/api/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		// 验证响应
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		assert.Equal(t, "test-upstream", resp.Header.Get("X-Upstream-Name"))

		// 验证响应内容
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		responseBody := string(body[:n])
		assert.Contains(t, responseBody, "Hello from upstream")
		assert.Contains(t, responseBody, "GET")
		assert.Contains(t, responseBody, "/api/test")

		t.Logf("GET 响应: %s", responseBody)
	})

	// 测试 POST 请求
	t.Run("POST Request", func(t *testing.T) {
		requestBody := bytes.NewBufferString(`{"test": "data"}`)
		resp, err := http.Post(proxyURL+"/api/create", "application/json", requestBody)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "test-upstream", resp.Header.Get("X-Upstream-Name"))

		// 验证响应内容
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		responseBody := string(body[:n])
		assert.Contains(t, responseBody, "POST")
		assert.Contains(t, responseBody, "/api/create")

		t.Logf("POST 响应: %s", responseBody)
	})

	// 测试自定义头部
	t.Run("Custom Headers", func(t *testing.T) {
		client := &http.Client{}
		req, err := http.NewRequest("GET", proxyURL+"/api/headers", nil)
		require.NoError(t, err)

		req.Header.Set("X-Custom-Header", "test-value")
		req.Header.Set("Authorization", "Bearer token123")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "test-upstream", resp.Header.Get("X-Upstream-Name"))

		t.Log("自定义头部测试通过")
	})

	// 测试多个并发请求
	t.Run("Concurrent Requests", func(t *testing.T) {
		const numRequests = 10
		results := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(id int) {
				resp, err := http.Get(proxyURL + fmt.Sprintf("/api/concurrent/%d", id))
				if err != nil {
					results <- err
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					results <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
					return
				}

				results <- nil
			}(i)
		}

		// 等待所有请求完成
		for i := 0; i < numRequests; i++ {
			err := <-results
			assert.NoError(t, err, "并发请求 %d 失败", i)
		}

		t.Logf("并发测试完成：%d 个请求全部成功", numRequests)
	})

	t.Log("端到端集成测试完成：所有核心功能正常工作")
}

// TestProxyServer_Configuration 测试服务器配置功能
func TestProxyServer_Configuration(t *testing.T) {
	t.Log("测试服务器配置功能")

	logger := logr.Discard()

	// 创建基本配置
	httpServerConfig := &config.HTTPServerConfig{
		Forwards: []config.ForwardConfig{
			{
				Name:         "config-test",
				Address:      "127.0.0.1",
				Port:         0,
				DefaultGroup: "test-group",
				Timeout: &config.TimeoutConfig{
					Idle:  30,
					Read:  15,
					Write: 15,
				},
			},
		},
		Admin: config.AdminConfig{
			Address: "127.0.0.1",
			Port:    0,
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
				Upstreams: []config.UpstreamRefConfig{},
			},
		},
		Upstreams: []config.UpstreamConfig{},
	}

	// 创建服务器
	server := NewServer(true, &logger, httpServerConfig, globalConfig)
	require.NotNil(t, server)

	// 测试服务器属性
	assert.Equal(t, 1, len(server.forwardServers))
	assert.NotNil(t, server.adminServer)

	// 测试获取转发服务器
	forwardServer := server.GetForwardServer("config-test")
	assert.NotNil(t, forwardServer)
	assert.Equal(t, "config-test", forwardServer.GetConfig().Name)

	// 测试获取不存在的服务器
	nonExistentServer := server.GetForwardServer("nonexistent")
	assert.Nil(t, nonExistentServer)

	t.Log("服务器配置测试完成")
}
