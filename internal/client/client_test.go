package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shengyanli1982/llmproxy-go/internal/balance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpClient_Do(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/success":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success response"))
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error response"))
		case "/timeout":
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	factory := NewFactory()
	config := DefaultConfig()
	config.RequestTimeout = 1 // 1 second

	client, err := factory.Create(config)
	require.NoError(t, err)
	defer client.Close()

	upstream := &balance.Upstream{
		Name: "test-upstream",
		URL:  server.URL,
	}

	t.Run("successful request", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/success", nil)
		require.NoError(t, err)

		resp, err := client.Do(req, upstream)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "success response", string(body))
	})

	t.Run("error response", func(t *testing.T) {
		// Disable retry for this test
		config := DefaultConfig()
		config.EnableRetry = false
		client, err := factory.Create(config)
		require.NoError(t, err)
		defer client.Close()

		req, err := http.NewRequest("GET", "/error", nil)
		require.NoError(t, err)

		resp, err := client.Do(req, &balance.Upstream{Name: "test", URL: server.URL})
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("timeout request", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/timeout", nil)
		require.NoError(t, err)

		_, err = client.Do(req, upstream)
		assert.Error(t, err)
		// 新的retry库可能返回不同的错误消息，检查是否包含超时相关的错误
		assert.True(t, err.Error() == "retry attempts exceeded" ||
			strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "context deadline exceeded"))
	})
}

func TestHttpClient_DoWithRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success after retries"))
	}))
	defer server.Close()

	factory := NewFactory()
	config := DefaultConfig()
	config.EnableRetry = true
	config.MaxRetries = 3
	config.RetryDelay = 10 // 10 milliseconds

	client, err := factory.Create(config)
	require.NoError(t, err)
	defer client.Close()

	upstream := &balance.Upstream{
		Name: "test-upstream",
		URL:  server.URL,
	}

	req, err := http.NewRequest("GET", "/test", nil)
	require.NoError(t, err)

	resp, err := client.Do(req, upstream)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, attempts) // Should have retried twice before success
}

func TestHttpClient_WithProxy(t *testing.T) {
	// Test with proxy configuration
	factory := NewFactory()
	config := DefaultConfig()
	config.ProxyURL = "http://proxy.example.com:8080"

	client, err := factory.Create(config)
	require.NoError(t, err)
	defer client.Close()

	assert.NotNil(t, client)
}

func TestHttpClient_POST_WithBody(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("received"))
	}))
	defer server.Close()

	factory := NewFactory()
	config := DefaultConfig()

	client, err := factory.Create(config)
	require.NoError(t, err)
	defer client.Close()

	upstream := &balance.Upstream{
		Name: "test-upstream",
		URL:  server.URL,
	}

	requestBody := "test request body"
	req, err := http.NewRequest("POST", "/test", strings.NewReader(requestBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req, upstream)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, requestBody, receivedBody)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 10, config.ConnectTimeout)
	assert.Equal(t, 60, config.RequestTimeout)
	assert.Equal(t, 90, config.IdleConnTimeout)
	assert.Equal(t, 100, config.MaxIdleConns)
	assert.Equal(t, 10, config.MaxIdleConnsPerHost)
	assert.Equal(t, true, config.EnableRetry)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1000, config.RetryDelay)
}

func TestHttpClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	factory := NewFactory()
	config := DefaultConfig()

	client, err := factory.Create(config)
	require.NoError(t, err)
	defer client.Close()

	upstream := &balance.Upstream{
		Name: "test-upstream",
		URL:  server.URL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "/test", nil)
	require.NoError(t, err)

	_, err = client.Do(req, upstream)
	assert.Error(t, err)
	// 新的retry库可能返回不同的错误消息，检查是否包含上下文相关的错误
	assert.True(t, err.Error() == "retry attempts exceeded" ||
		strings.Contains(err.Error(), "context deadline exceeded") ||
		strings.Contains(err.Error(), "context canceled"))
}

func TestHttpClient_Close(t *testing.T) {
	factory := NewFactory()
	config := DefaultConfig()

	client, err := factory.Create(config)
	require.NoError(t, err)

	// Should not panic
	client.Close()

	// Should be able to call Close multiple times
	client.Close()
}

// Benchmark tests
func BenchmarkHttpClient_Do(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("benchmark response"))
	}))
	defer server.Close()

	factory := NewFactory()
	config := DefaultConfig()

	client, err := factory.Create(config)
	require.NoError(b, err)
	defer client.Close()

	upstream := &balance.Upstream{
		Name: "test-upstream",
		URL:  server.URL,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/test", nil)
		resp, err := client.Do(req, upstream)
		if err == nil {
			resp.Body.Close()
		}
	}
}

func TestHttpClient_Name(t *testing.T) {
	factory := NewFactory()
	config := DefaultConfig()

	client, err := factory.Create(config)
	require.NoError(t, err)
	defer client.Close()

	name := client.Name()
	assert.NotEmpty(t, name)
	assert.Contains(t, name, "http-client-")
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid config",
			config:    DefaultConfig(),
			wantError: false,
		},
		{
			name: "zero connect timeout",
			config: &Config{
				ConnectTimeout: 0,
				RequestTimeout: 60,
				MaxRetries:     3,
				RetryDelay:     1000,
			},
			wantError: true,
			errorMsg:  "connect timeout must be positive",
		},
		{
			name: "negative connect timeout",
			config: &Config{
				ConnectTimeout: -5,
				RequestTimeout: 60,
				MaxRetries:     3,
				RetryDelay:     1000,
			},
			wantError: true,
			errorMsg:  "connect timeout must be positive",
		},
		{
			name: "zero request timeout",
			config: &Config{
				ConnectTimeout: 10,
				RequestTimeout: 0,
				MaxRetries:     3,
				RetryDelay:     1000,
			},
			wantError: true,
			errorMsg:  "request timeout must be positive",
		},
		{
			name: "negative request timeout",
			config: &Config{
				ConnectTimeout: 10,
				RequestTimeout: -30,
				MaxRetries:     3,
				RetryDelay:     1000,
			},
			wantError: true,
			errorMsg:  "request timeout must be positive",
		},
		{
			name: "negative max retries",
			config: &Config{
				ConnectTimeout: 10,
				RequestTimeout: 60,
				MaxRetries:     -1,
				RetryDelay:     1000,
			},
			wantError: true,
			errorMsg:  "max retries must be non-negative",
		},
		{
			name: "negative retry delay",
			config: &Config{
				ConnectTimeout: 10,
				RequestTimeout: 60,
				MaxRetries:     3,
				RetryDelay:     -500,
			},
			wantError: true,
			errorMsg:  "retry delay must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create factory and test config validation through Create method
			factory := NewFactory()
			client, err := factory.Create(tt.config)

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				if client != nil {
					client.Close()
				}
			}
		})
	}
}

func TestProxyHandler(t *testing.T) {
	t.Run("empty proxy URL", func(t *testing.T) {
		handler := NewProxyHandlerFromURL("")

		assert.False(t, handler.IsEnabled())
		assert.Equal(t, "", handler.GetProxyURL())

		proxyFunc := handler.GetProxyFunc()
		assert.NotNil(t, proxyFunc)
	})

	t.Run("valid proxy URL", func(t *testing.T) {
		proxyURL := "http://proxy.example.com:8080"
		handler := NewProxyHandlerFromURL(proxyURL)

		assert.True(t, handler.IsEnabled())
		assert.Equal(t, proxyURL, handler.GetProxyURL())

		proxyFunc := handler.GetProxyFunc()
		assert.NotNil(t, proxyFunc)

		req := httptest.NewRequest("GET", "http://test.com", nil)
		resultURL, err := proxyFunc(req)
		assert.NoError(t, err)
		assert.Equal(t, proxyURL, resultURL.String())
	})

	t.Run("invalid proxy URL", func(t *testing.T) {
		handler := NewProxyHandlerFromURL(":/invalid-url")

		assert.False(t, handler.IsEnabled())
		assert.Equal(t, "", handler.GetProxyURL())
	})

	t.Run("update proxy URL", func(t *testing.T) {
		handler := NewProxyHandlerFromURL("")

		// Update to valid URL
		newURL := "http://new-proxy.example.com:9090"
		err := handler.Update(newURL)
		assert.NoError(t, err)
		assert.True(t, handler.IsEnabled())
		assert.Equal(t, newURL, handler.GetProxyURL())

		// Update to empty URL
		err = handler.Update("")
		assert.NoError(t, err)
		assert.False(t, handler.IsEnabled())
		assert.Equal(t, "", handler.GetProxyURL())

		// Update to invalid URL
		err = handler.Update(":/invalid")
		assert.Error(t, err)
	})
}

func TestRetryHandler_Getters(t *testing.T) {
	config := &Config{
		EnableRetry: true,
		MaxRetries:  5,
		RetryDelay:  2000,
	}

	handler := NewRetryHandler(config)

	assert.True(t, handler.IsEnabled())

	retryConfig := handler.GetConfig()
	assert.NotNil(t, retryConfig)
	assert.Equal(t, true, retryConfig["enabled"])
	assert.Equal(t, 5, retryConfig["max_retries"])
	assert.Equal(t, 2000, retryConfig["retry_delay"])
}

func TestRetryableError(t *testing.T) {
	err := &RetryableError{
		Message: "network timeout occurred",
	}

	errorMsg := err.Error()
	assert.Equal(t, "network timeout occurred", errorMsg)
}

func TestHttpClient_ConfigValidation(t *testing.T) {
	factory := NewFactory()

	// Test config based on config.default.yaml
	tests := []struct {
		name      string
		config    *Config
		wantError bool
	}{
		{
			name: "config from default.yaml - keepalive in range",
			config: &Config{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90,
				ConnectTimeout:      10,
				RequestTimeout:      300,
				EnableKeepAlive:     true,
				EnableRetry:         true,
				MaxRetries:          3,
				RetryDelay:          500,
			},
			wantError: false,
		},
		{
			name: "config from default.yaml - with proxy",
			config: &Config{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90,
				ConnectTimeout:      10,
				RequestTimeout:      300,
				EnableKeepAlive:     true,
				EnableRetry:         true,
				MaxRetries:          2,
				RetryDelay:          1000,
				ProxyURL:            "http://proxy.example.com:8080",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := factory.Create(tt.config)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				if client != nil {
					client.Close()
				}
			}
		})
	}
}
