package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestServer_Integration(t *testing.T) {
	logger := logr.Discard()
	
	// Create a test HTTP server to act as upstream
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "upstream response"}`))
	}))
	defer upstreamServer.Close()

	// Create server configuration
	httpServerConfig := &config.HTTPServerConfig{
		Forwards: []config.ForwardConfig{
			{
				Name:         "test-forward",
				Address:      "127.0.0.1",
				Port:         0, // Use random available port
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
			Port:    0, // Use random available port
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
				URL:  upstreamServer.URL,
			},
		},
	}

	// Create and test server
	server := NewServer(true, &logger, httpServerConfig, globalConfig)
	require.NotNil(t, server)
	
	// Test server properties
	assert.Equal(t, 1, len(server.forwardServers))
	assert.NotNil(t, server.adminServer)
	
	// Test adding and removing forward servers
	newForwardConfig := config.ForwardConfig{
		Name:         "new-forward",
		Address:      "127.0.0.1",
		Port:         0,
		DefaultGroup: "test-group",
		Timeout: &config.TimeoutConfig{
			Idle:  30,
			Read:  15,
			Write: 15,
		},
	}
	
	newForwardServer := NewForwardServer(true, &logger, &newForwardConfig, globalConfig)
	server.AddForwardServer(newForwardServer)
	assert.Equal(t, 2, len(server.forwardServers))
	
	// Test getting forward server
	retrievedServer := server.GetForwardServer("new-forward")
	assert.NotNil(t, retrievedServer)
	assert.Equal(t, "new-forward", retrievedServer.GetConfig().Name)
	
	// Test removing forward server
	server.RemoveForwardServer(newForwardServer)
	assert.Equal(t, 1, len(server.forwardServers))
	
	// Test getting non-existent server
	nonExistentServer := server.GetForwardServer("nonexistent")
	assert.Nil(t, nonExistentServer)
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
		},
	}
	
	globalConfig := &config.Config{
		HTTPServer: *httpServerConfig,
	}
	
	server := NewServer(true, &logger, httpServerConfig, globalConfig)
	adminService := NewAdminServices()
	adminService.Initialize(&httpServerConfig.Admin, globalConfig, &logger, server)
	
	// Test admin service properties
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
	
	upstream := balance.Upstream{
		Name: "test-upstream",
		URL:  "http://upstream.com",
	}
	
	// Create proxy request
	proxyReq, err := service.createProxyRequest(originalReq, upstream)
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
}

func TestForwardService_ErrorHandling(t *testing.T) {
	service := NewForwardServices()
	
	// Test with no upstreams
	ctx := context.Background()
	emptyUpstreams := []balance.Upstream{}
	
	_, err := service.loadBalancer.Select(ctx, emptyUpstreams)
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
	
	upstream := balance.Upstream{
		Name: "benchmark-upstream",
		URL:  "http://upstream.com",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.createProxyRequest(originalReq, upstream)
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