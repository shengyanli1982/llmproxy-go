package balance

import (
	"context"
	"testing"
	"time"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundRobinBalancer(t *testing.T) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}

	balancer := NewRoundRobinBalancer()
	ctx := context.Background()

	// Test round robin selection
	selections := make([]string, 6)
	for i := 0; i < 6; i++ {
		upstream, err := balancer.Select(ctx, upstreams)
		require.NoError(t, err)
		selections[i] = upstream.Name
	}

	// Should cycle through upstreams
	expected := []string{"upstream1", "upstream2", "upstream3", "upstream1", "upstream2", "upstream3"}
	assert.Equal(t, expected, selections)
}

func TestWeightedRoundRobinBalancer(t *testing.T) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 2},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}

	balancer := NewWeightedRoundRobinBalancer()
	ctx := context.Background()

	// Collect selections to verify weight distribution
	selections := make(map[string]int)
	for i := 0; i < 100; i++ {
		upstream, err := balancer.Select(ctx, upstreams)
		require.NoError(t, err)
		selections[upstream.Name]++
	}

	// upstream2 should be selected approximately twice as often
	assert.Greater(t, selections["upstream2"], selections["upstream1"])
	assert.Greater(t, selections["upstream2"], selections["upstream3"])
}

func TestRandomBalancer(t *testing.T) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}

	balancer := NewRandomBalancer()
	ctx := context.Background()

	// Test that all upstreams can be selected
	selectedUpstreams := make(map[string]bool)
	for i := 0; i < 100; i++ {
		upstream, err := balancer.Select(ctx, upstreams)
		require.NoError(t, err)
		selectedUpstreams[upstream.Name] = true
	}

	// All upstreams should be selected at least once
	assert.True(t, selectedUpstreams["upstream1"])
	assert.True(t, selectedUpstreams["upstream2"])
	assert.True(t, selectedUpstreams["upstream3"])
}

func TestResponseAwareBalancer(t *testing.T) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}

	balancer := NewResponseAwareBalancer()
	ctx := context.Background()

	// Initially should select randomly
	upstream, err := balancer.Select(ctx, upstreams)
	require.NoError(t, err)
	assert.Contains(t, []string{"upstream1", "upstream2", "upstream3"}, upstream.Name)

	// Update latencies to influence selection
	balancer.UpdateLatency("upstream1", 100) // Fast
	balancer.UpdateLatency("upstream2", 500) // Slow
	balancer.UpdateLatency("upstream3", 200) // Medium

	// After multiple selections, should prefer faster upstream
	selections := make(map[string]int)
	for i := 0; i < 100; i++ {
		upstream, err := balancer.Select(ctx, upstreams)
		require.NoError(t, err)
		selections[upstream.Name]++
	}

	// upstream1 (fastest) should be selected most often
	assert.Greater(t, selections["upstream1"], selections["upstream2"])
}

func TestFailoverBalancer(t *testing.T) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}

	balancer := NewFailoverBalancer()
	ctx := context.Background()

	// Initially should select first upstream
	upstream, err := balancer.Select(ctx, upstreams)
	require.NoError(t, err)
	assert.Equal(t, "upstream1", upstream.Name)

	// Mark first upstream as unhealthy
	if fb, ok := balancer.(*failoverBalancer); ok {
		fb.mu.Lock()
		fb.healthMap["upstream1"] = false
		fb.mu.Unlock()
	}

	// Should now select second upstream
	upstream, err = balancer.Select(ctx, upstreams)
	require.NoError(t, err)
	assert.Equal(t, "upstream2", upstream.Name)
}

func TestFactory_Create(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name      string
		config    *config.BalanceConfig
		wantType  string
		wantError bool
	}{
		{
			name:      "roundrobin",
			config:    &config.BalanceConfig{Strategy: "roundrobin"},
			wantType:  "roundrobin",
			wantError: false,
		},
		{
			name:      "weighted_roundrobin",
			config:    &config.BalanceConfig{Strategy: "weighted_roundrobin"},
			wantType:  "weighted_roundrobin",
			wantError: false,
		},
		{
			name:      "random",
			config:    &config.BalanceConfig{Strategy: "random"},
			wantType:  "random",
			wantError: false,
		},
		{
			name:      "response_aware",
			config:    &config.BalanceConfig{Strategy: "response_aware"},
			wantType:  "response_aware",
			wantError: false,
		},
		{
			name:      "failover",
			config:    &config.BalanceConfig{Strategy: "failover"},
			wantType:  "failover",
			wantError: false,
		},
		{
			name:      "unknown strategy",
			config:    &config.BalanceConfig{Strategy: "unknown"},
			wantType:  "",
			wantError: true,
		},
		{
			name:      "nil config",
			config:    nil,
			wantType:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balancer, err := factory.Create(tt.config)
			
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, balancer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, balancer)
				assert.Equal(t, tt.wantType, balancer.Type())
			}
		})
	}
}

func TestSlidingWindow(t *testing.T) {
	window := NewSlidingWindow(3)

	// Test adding records
	window.Add(100)
	window.Add(200)
	window.Add(300)

	avg := window.Average()
	assert.Equal(t, int64(200), avg) // (100+200+300)/3 = 200

	// Test window sliding
	window.Add(400) // Should remove 100, window: [200, 300, 400]
	avg = window.Average()
	assert.Equal(t, int64(300), avg) // (200+300+400)/3 = 300
}

func TestEmptyUpstreams(t *testing.T) {
	balancers := []LoadBalancer{
		NewRoundRobinBalancer(),
		NewWeightedRoundRobinBalancer(),
		NewRandomBalancer(),
		NewResponseAwareBalancer(),
		NewFailoverBalancer(),
	}

	ctx := context.Background()
	emptyUpstreams := []Upstream{}

	for _, balancer := range balancers {
		t.Run(balancer.Type(), func(t *testing.T) {
			_, err := balancer.Select(ctx, emptyUpstreams)
			assert.Error(t, err)
		})
	}
}

func TestBalancerWithBreaker(t *testing.T) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
	}

	// Test with response aware balancer (which implements LoadBalancerWithBreaker)
	balancer := NewResponseAwareBalancer()
	ctx := context.Background()

	if breakerBalancer, ok := balancer.(LoadBalancerWithBreaker); ok {
		// Test creating a breaker
		settings := gobreaker.Settings{
			Name:         "test-breaker",
			MaxRequests:  3,
			Interval:     10 * time.Second,
			Timeout:      5 * time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures > 2
			},
		}

		err := breakerBalancer.CreateBreaker("upstream1", settings)
		assert.NoError(t, err)

		// Should still be able to select upstream
		upstream, err := balancer.Select(ctx, upstreams)
		assert.NoError(t, err)
		assert.Equal(t, "upstream1", upstream.Name)
	}
}

// Benchmark tests
func BenchmarkRoundRobinBalancer_Select(b *testing.B) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}
	
	balancer := NewRoundRobinBalancer()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = balancer.Select(ctx, upstreams)
	}
}

func BenchmarkResponseAwareBalancer_Select(b *testing.B) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}
	
	balancer := NewResponseAwareBalancer()
	ctx := context.Background()
	
	// Simulate some latency data
	balancer.UpdateLatency("upstream1", 100)
	balancer.UpdateLatency("upstream2", 200)
	balancer.UpdateLatency("upstream3", 150)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = balancer.Select(ctx, upstreams)
	}
}

func TestCreateFromConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.UpstreamGroupConfig
		wantType  string
		wantError bool
	}{
		{
			name:      "valid roundrobin config",
			config:    &config.UpstreamGroupConfig{Balance: &config.BalanceConfig{Strategy: "roundrobin"}},
			wantType:  "roundrobin",
			wantError: false,
		},
		{
			name:      "valid weighted_roundrobin config",
			config:    &config.UpstreamGroupConfig{Balance: &config.BalanceConfig{Strategy: "weighted_roundrobin"}},
			wantType:  "weighted_roundrobin",
			wantError: false,
		},
		{
			name:      "nil config",
			config:    nil,
			wantType:  "",
			wantError: true,
		},
		{
			name:      "nil balance config - should default to roundrobin",
			config:    &config.UpstreamGroupConfig{Balance: nil},
			wantType:  "roundrobin",
			wantError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balancer, err := CreateFromConfig(tt.config)
			
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, balancer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, balancer)
				assert.Equal(t, tt.wantType, balancer.Type())
			}
		})
	}
}

func TestUpdateHealthMethods(t *testing.T) {
	balancers := []LoadBalancer{
		NewRoundRobinBalancer(),
		NewWeightedRoundRobinBalancer(),
		NewRandomBalancer(),
		NewResponseAwareBalancer(),
		NewFailoverBalancer(),
	}
	
	for _, balancer := range balancers {
		t.Run(balancer.Type()+"_UpdateHealth", func(t *testing.T) {
			// Should not panic
			balancer.UpdateHealth("upstream1", false)
			balancer.UpdateHealth("upstream1", true)
		})
	}
}

func TestUpdateLatencyMethods(t *testing.T) {
	balancers := []LoadBalancer{
		NewRoundRobinBalancer(),
		NewWeightedRoundRobinBalancer(),
		NewRandomBalancer(),
		NewResponseAwareBalancer(),
		NewFailoverBalancer(),
	}
	
	for _, balancer := range balancers {
		t.Run(balancer.Type()+"_UpdateLatency", func(t *testing.T) {
			// Should not panic
			balancer.UpdateLatency("upstream1", 100)
			balancer.UpdateLatency("upstream1", 200)
		})
	}
}

func TestResponseAwareBalancer_WithBreaker(t *testing.T) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
	}
	
	balancer := NewResponseAwareBalancer()
	ctx := context.Background()
	
	// Test implementing LoadBalancerWithBreaker interface
	if breakerBalancer, ok := balancer.(LoadBalancerWithBreaker); ok {
		// Test GetBreaker before creating one
		_, exists := breakerBalancer.GetBreaker("upstream1")
		assert.False(t, exists)
		
		// Create breaker
		settings := gobreaker.Settings{
			Name:         "test-breaker",
			MaxRequests:  3,
			Interval:     10 * time.Second,
			Timeout:      5 * time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures > 2
			},
		}
		
		err := breakerBalancer.CreateBreaker("upstream1", settings)
		assert.NoError(t, err)
		
		// Test GetBreaker after creating one
		breaker, exists := breakerBalancer.GetBreaker("upstream1")
		assert.True(t, exists)
		assert.NotNil(t, breaker)
		
		// Test creating breaker for same upstream again (should not error)
		err = breakerBalancer.CreateBreaker("upstream1", settings)
		assert.NoError(t, err)
		
		// Should still be able to select upstream
		upstream, err := balancer.Select(ctx, upstreams)
		assert.NoError(t, err)
		assert.Equal(t, "upstream1", upstream.Name)
	}
}

func TestFailoverBalancer_HealthCheck(t *testing.T) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}
	
	balancer := NewFailoverBalancer()
	ctx := context.Background()
	
	// Initially should select first upstream
	upstream, err := balancer.Select(ctx, upstreams)
	require.NoError(t, err)
	assert.Equal(t, "upstream1", upstream.Name)
	
	// Update health for first upstream
	balancer.UpdateHealth("upstream1", false)
	
	// Should now select second upstream
	upstream, err = balancer.Select(ctx, upstreams)
	require.NoError(t, err)
	assert.Equal(t, "upstream2", upstream.Name)
	
	// Mark second as unhealthy too
	balancer.UpdateHealth("upstream2", false)
	
	// Should select third upstream
	upstream, err = balancer.Select(ctx, upstreams)
	require.NoError(t, err)
	assert.Equal(t, "upstream3", upstream.Name)
	
	// Mark all as unhealthy
	balancer.UpdateHealth("upstream3", false)
	
	// Should still return first upstream as last resort (failover behavior)
	upstream, err = balancer.Select(ctx, upstreams)
	require.NoError(t, err)
	assert.Equal(t, "upstream1", upstream.Name) // Returns first upstream as fallback
}

func TestFailoverBalancer_WithBreaker(t *testing.T) {
	balancer := NewFailoverBalancer()
	
	// Test implementing LoadBalancerWithBreaker interface 
	if breakerBalancer, ok := balancer.(LoadBalancerWithBreaker); ok {
		// Test GetBreaker
		_, exists := breakerBalancer.GetBreaker("upstream1")
		assert.False(t, exists)
		
		// Test CreateBreaker
		settings := gobreaker.Settings{
			Name: "test-breaker",
		}
		
		err := breakerBalancer.CreateBreaker("upstream1", settings)
		assert.NoError(t, err)
		
		// Test GetBreaker after creation
		breaker, exists := breakerBalancer.GetBreaker("upstream1")
		assert.True(t, exists)
		assert.NotNil(t, breaker)
	}
}

func TestSlidingWindow_EdgeCases(t *testing.T) {
	t.Run("empty window average", func(t *testing.T) {
		window := NewSlidingWindow(3)
		avg := window.Average()
		assert.Equal(t, int64(1000), avg) // Default value for empty window
	})
	
	t.Run("single value", func(t *testing.T) {
		window := NewSlidingWindow(1)
		window.Add(100)
		avg := window.Average()
		assert.Equal(t, int64(100), avg)
	})
	
	t.Run("zero size window", func(t *testing.T) {
		window := NewSlidingWindow(0)
		window.Add(100)
		avg := window.Average()
		assert.Equal(t, int64(1000), avg) // Default value for zero size window
	})
}

func TestBalancersWithConfigDefaults(t *testing.T) {
	// Test balancers with config.default.yaml strategy values
	strategies := []string{
		"roundrobin",
		"weighted_roundrobin", 
		"random",
		"response_aware",
		"failover",
	}
	
	factory := NewFactory()
	
	for _, strategy := range strategies {
		t.Run("strategy_"+strategy, func(t *testing.T) {
			config := &config.BalanceConfig{Strategy: strategy}
			balancer, err := factory.Create(config)
			
			assert.NoError(t, err)
			assert.NotNil(t, balancer)
			assert.Equal(t, strategy, balancer.Type())
		})
	}
}