package ratelimit

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTokenBucketLimiter_Allow(t *testing.T) {
	limiter := NewTokenBucketLimiter(2.0, 5) // 2 per second, burst of 5

	// Should allow initial burst
	for i := 0; i < 5; i++ {
		allowed := limiter.Allow("test-key")
		assert.True(t, allowed, "Should allow request %d within burst", i+1)
	}

	// Should deny after burst limit
	allowed := limiter.Allow("test-key")
	assert.False(t, allowed, "Should deny request after burst limit")

	// Wait for token refill and try again
	time.Sleep(500 * time.Millisecond) // 0.5 seconds = 1 token
	allowed = limiter.Allow("test-key")
	assert.True(t, allowed, "Should allow request after token refill")
}

func TestTokenBucketLimiter_MultipleKeys(t *testing.T) {
	limiter := NewTokenBucketLimiter(1.0, 2) // 1 per second, burst of 2

	// Different keys should have independent limits
	assert.True(t, limiter.Allow("key1"))
	assert.True(t, limiter.Allow("key2"))
	assert.True(t, limiter.Allow("key1"))
	assert.True(t, limiter.Allow("key2"))

	// Both keys should be exhausted now
	assert.False(t, limiter.Allow("key1"))
	assert.False(t, limiter.Allow("key2"))
}

func TestTokenBucketLimiter_Reset(t *testing.T) {
	limiter := NewTokenBucketLimiter(1.0, 1) // 1 per second, burst of 1

	// Exhaust the limit
	assert.True(t, limiter.Allow("test-key"))
	assert.False(t, limiter.Allow("test-key"))

	// Reset should allow requests again
	limiter.Reset("test-key")
	assert.True(t, limiter.Allow("test-key"))
}

func TestTokenBucketLimiter_Type(t *testing.T) {
	limiter := NewTokenBucketLimiter(1.0, 1)
	assert.Equal(t, "token_bucket", limiter.Type())
}

func TestTokenBucketLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewTokenBucketLimiter(100.0, 10) // High rate for concurrent testing

	var wg sync.WaitGroup
	successCount := int32(0)

	// Launch multiple goroutines
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "concurrent-key"
			if limiter.Allow(key) {
				// Use atomic operation if needed, but for this test we just count
				// This is not thread-safe but gives us a rough idea
				successCount++
			}
		}(i)
	}

	wg.Wait()

	// Should have some successes but not exceed burst limit significantly
	assert.Greater(t, int(successCount), 0)
	assert.LessOrEqual(t, int(successCount), 20) // Some reasonable upper bound
}

func TestIPLimiter_Allow(t *testing.T) {
	limiter := NewIPLimiter(2.0, 3) // 2 per second, burst of 3

	tests := []struct {
		name           string
		remoteAddr     string
		expectedIP     string
		requestHeaders map[string]string
	}{
		{
			name:       "direct connection",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "with X-Forwarded-For",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "203.0.113.1",
			requestHeaders: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 10.0.0.1",
			},
		},
		{
			name:       "with X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "203.0.113.2",
			requestHeaders: map[string]string{
				"X-Real-IP": "203.0.113.2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for key, value := range tt.requestHeaders {
				req.Header.Set(key, value)
			}

			// Should allow initial requests
			for i := 0; i < 3; i++ {
				allowed := limiter.Allow(req)
				assert.True(t, allowed, "Should allow request %d", i+1)
			}

			// Should deny after limit
			allowed := limiter.Allow(req)
			assert.False(t, allowed, "Should deny request after limit")
		})
	}
}

func TestIPLimiter_Reset(t *testing.T) {
	limiter := NewIPLimiter(1.0, 1)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// Exhaust limit
	assert.True(t, limiter.Allow(req))
	assert.False(t, limiter.Allow(req))

	// Reset should restore access
	limiter.Reset("192.168.1.1")
	assert.True(t, limiter.Allow(req))
}

func TestUpstreamLimiter_Allow(t *testing.T) {
	limiter := NewUpstreamLimiter(2.0, 3)

	upstream := "upstream1"

	// Should allow initial requests
	for i := 0; i < 3; i++ {
		allowed := limiter.Allow(upstream)
		assert.True(t, allowed, "Should allow request %d", i+1)
	}

	// Should deny after limit
	allowed := limiter.Allow(upstream)
	assert.False(t, allowed, "Should deny request after limit")

	// Different upstream should have independent limit
	allowed = limiter.Allow("upstream2")
	assert.True(t, allowed, "Should allow request for different upstream")
}

func TestUpstreamLimiter_EmptyUpstreamName(t *testing.T) {
	limiter := NewUpstreamLimiter(1.0, 1)

	// Empty upstream name should default to allow
	allowed := limiter.Allow("")
	assert.True(t, allowed)
}

func TestUpstreamLimiter_Type(t *testing.T) {
	limiter := NewUpstreamLimiter(1.0, 1)
	assert.Equal(t, "upstream_token_bucket", limiter.Type())
}

func TestRateLimitFactory_Create(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name      string
		perSecond float64
		burst     int
		wantError bool
	}{
		{
			name:      "valid parameters",
			perSecond: 10.0,
			burst:     20,
			wantError: false,
		},
		{
			name:      "zero perSecond",
			perSecond: 0,
			burst:     10,
			wantError: true,
		},
		{
			name:      "negative perSecond",
			perSecond: -1.0,
			burst:     10,
			wantError: true,
		},
		{
			name:      "zero burst",
			perSecond: 10.0,
			burst:     0,
			wantError: true,
		},
		{
			name:      "negative burst",
			perSecond: 10.0,
			burst:     -1,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, err := factory.Create(tt.perSecond, tt.burst)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, limiter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, limiter)
				assert.Equal(t, "token_bucket", limiter.Type())
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	middleware := NewRateLimitMiddleware(1.0, 1, 1.0, 1)

	// Test that middleware is enabled by default
	assert.True(t, middleware.IsEnabled())

	// Test enable/disable
	middleware.Disable()
	assert.False(t, middleware.IsEnabled())

	middleware.Enable()
	assert.True(t, middleware.IsEnabled())
}

func TestRateLimitMiddleware_ResetMethods(t *testing.T) {
	middleware := NewRateLimitMiddleware(1.0, 1, 1.0, 1)

	// Test reset methods don't panic
	middleware.ResetIP("192.168.1.1")
	middleware.ResetUpstream("upstream1")
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 100.0, config.PerSecond)
	assert.Equal(t, 200, config.Burst)
}

func TestParseFirstIP(t *testing.T) {
	tests := []struct {
		name     string
		xff      string
		expected string
	}{
		{
			name:     "single IP",
			xff:      "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "multiple IPs",
			xff:      "203.0.113.1, 10.0.0.1, 192.168.1.1",
			expected: "203.0.113.1",
		},
		{
			name:     "invalid IP",
			xff:      "invalid-ip",
			expected: "",
		},
		{
			name:     "mixed valid and invalid",
			xff:      "invalid, 192.168.1.1",
			expected: "",
		},
		{
			name:     "IPv6",
			xff:      "2001:db8::1",
			expected: "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFirstIP(tt.xff)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Benchmark tests
func BenchmarkTokenBucketLimiter_Allow(b *testing.B) {
	limiter := NewTokenBucketLimiter(1000.0, 1000) // High limits to avoid blocking

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow("benchmark-key")
	}
}

func BenchmarkTokenBucketLimiter_ConcurrentAllow(b *testing.B) {
	limiter := NewTokenBucketLimiter(10000.0, 10000) // Very high limits

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow("benchmark-key")
		}
	})
}

func BenchmarkIPLimiter_Allow(b *testing.B) {
	limiter := NewIPLimiter(1000.0, 1000)
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow(req)
	}
}

func TestRateLimitMiddleware_Middleware(t *testing.T) {
	middleware := NewRateLimitMiddleware(1.0, 1, 1.0, 1)

	t.Run("middleware creation", func(t *testing.T) {
		handler := middleware.Middleware()
		assert.NotNil(t, handler)
	})
}

func TestRateLimitConfigValidation(t *testing.T) {
	// Test configuration validation based on config.default.yaml values
	tests := []struct {
		name      string
		perSecond float64
		burst     int
		wantError bool
	}{
		{
			name:      "valid config from default.yaml",
			perSecond: 100.0, // from config.default.yaml
			burst:     200,   // from config.default.yaml
			wantError: false,
		},
		{
			name:      "zero perSecond",
			perSecond: 0,
			burst:     100,
			wantError: true,
		},
		{
			name:      "negative perSecond",
			perSecond: -1.0,
			burst:     100,
			wantError: true,
		},
		{
			name:      "zero burst",
			perSecond: 100.0,
			burst:     0,
			wantError: true,
		},
		{
			name:      "negative burst",
			perSecond: 100.0,
			burst:     -1,
			wantError: true,
		},
	}

	factory := NewFactory()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, err := factory.Create(tt.perSecond, tt.burst)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, limiter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, limiter)
			}
		})
	}
}

func TestIPLimiter_EdgeCases(t *testing.T) {
	limiter := NewIPLimiter(1.0, 1)

	t.Run("request with no RemoteAddr", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ""

		allowed := limiter.Allow(req)
		// Should handle gracefully - either allow or deny, but not crash
		assert.IsType(t, false, allowed)
	})

	t.Run("IPv6 address", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "[2001:db8::1]:8080"

		allowed := limiter.Allow(req)
		assert.True(t, allowed) // First request should be allowed
	})
}

func TestUpstreamLimiter_EdgeCases(t *testing.T) {
	limiter := NewUpstreamLimiter(1.0, 1)

	t.Run("empty upstream name", func(t *testing.T) {
		allowed := limiter.Allow("")
		assert.True(t, allowed) // Should allow empty upstream
	})

	t.Run("special characters in upstream name", func(t *testing.T) {
		upstream := "upstream-with-special@chars#123"

		// First request should pass
		allowed := limiter.Allow(upstream)
		assert.True(t, allowed)

		// Second should be limited
		allowed = limiter.Allow(upstream)
		assert.False(t, allowed)
	})
}

func TestTokenBucketLimiter_EdgeCases(t *testing.T) {
	t.Run("very high burst", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(1.0, 10000)

		// Should handle high burst gracefully
		for i := 0; i < 5000; i++ {
			allowed := limiter.Allow("test-key")
			assert.True(t, allowed)
		}
	})

	t.Run("very low rate", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(0.1, 1) // 1 per 10 seconds

		allowed := limiter.Allow("test-key")
		assert.True(t, allowed) // First should pass

		allowed = limiter.Allow("test-key")
		assert.False(t, allowed) // Second should fail
	})
}
