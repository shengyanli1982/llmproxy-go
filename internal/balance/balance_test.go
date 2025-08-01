package balance

import (
	"context"
	"testing"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundRobinBalancer(t *testing.T) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}

	balancer := NewRRBalancer()
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

	balancer := NewWeightedRRBalancer()
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

func TestClientIPContext(t *testing.T) {
	ctx := context.Background()

	t.Run("WithClientIP and GetClientIP", func(t *testing.T) {
		testIP := "192.168.1.100"

		// 测试存储客户端 IP
		ctxWithIP := WithClientIP(ctx, testIP)
		assert.NotNil(t, ctxWithIP)

		// 测试获取客户端 IP
		retrievedIP, ok := GetClientIP(ctxWithIP)
		assert.True(t, ok)
		assert.Equal(t, testIP, retrievedIP)
	})

	t.Run("GetClientIP from empty context", func(t *testing.T) {
		// 测试从空 context 获取客户端 IP
		retrievedIP, ok := GetClientIP(ctx)
		assert.False(t, ok)
		assert.Empty(t, retrievedIP)
	})

	t.Run("WithClientIP empty string", func(t *testing.T) {
		// 测试存储空字符串
		ctxWithEmptyIP := WithClientIP(ctx, "")
		assert.NotNil(t, ctxWithEmptyIP)

		retrievedIP, ok := GetClientIP(ctxWithEmptyIP)
		assert.True(t, ok)
		assert.Empty(t, retrievedIP)
	})
}

func TestIPHashBalancer(t *testing.T) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}

	balancer := NewIPHashBalancer()
	assert.Equal(t, "iphash", balancer.Type())

	t.Run("consistent hashing with client IP", func(t *testing.T) {
		testIP := "192.168.1.100"
		ctx := WithClientIP(context.Background(), testIP)

		// 多次选择应该返回相同的上游服务
		var selectedUpstream string
		for i := 0; i < 10; i++ {
			upstream, err := balancer.Select(ctx, upstreams)
			assert.NoError(t, err)

			if i == 0 {
				selectedUpstream = upstream.Name
			} else {
				assert.Equal(t, selectedUpstream, upstream.Name, "Same IP should always select same upstream")
			}
		}
	})

	t.Run("no client IP fallback to random", func(t *testing.T) {
		ctx := context.Background() // 没有客户端 IP

		upstream, err := balancer.Select(ctx, upstreams)
		assert.NoError(t, err)
		assert.Contains(t, []string{"upstream1", "upstream2", "upstream3"}, upstream.Name)
	})

	t.Run("UpdateHealth and UpdateLatency are no-op", func(t *testing.T) {
		// 这些方法应该不会导致 panic 或错误
		balancer.UpdateHealth("upstream1", false)
		balancer.UpdateLatency("upstream1", 100)

		// 仍然应该能够正常选择
		ctx := WithClientIP(context.Background(), "192.168.1.100")
		upstream, err := balancer.Select(ctx, upstreams)
		assert.NoError(t, err)
		assert.NotEmpty(t, upstream.Name)
	})
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
			name:      "iphash",
			config:    &config.BalanceConfig{Strategy: "iphash"},
			wantType:  "iphash",
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

func TestEmptyUpstreams(t *testing.T) {
	balancers := []LoadBalancer{
		NewRRBalancer(),
		NewWeightedRRBalancer(),
		NewRandomBalancer(),
		NewIPHashBalancer(),
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

// Benchmark tests
func BenchmarkRoundRobinBalancer_Select(b *testing.B) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}

	balancer := NewRRBalancer()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = balancer.Select(ctx, upstreams)
	}
}

// BenchmarkRoundRobinBalancer_Select_Concurrent 并发性能测试
func BenchmarkRoundRobinBalancer_Select_Concurrent(b *testing.B) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}

	balancer := NewRRBalancer()
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = balancer.Select(ctx, upstreams)
		}
	})
}

// BenchmarkWeightedRRBalancer_Select_Concurrent 加权轮询并发性能测试
func BenchmarkWeightedRRBalancer_Select_Concurrent(b *testing.B) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 2},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}

	balancer := NewWeightedRRBalancer()
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = balancer.Select(ctx, upstreams)
		}
	})
}

// BenchmarkRandomBalancer_Select_Concurrent 随机负载均衡器并发性能测试
func BenchmarkRandomBalancer_Select_Concurrent(b *testing.B) {
	upstreams := []Upstream{
		{Name: "upstream1", URL: "http://example1.com", Weight: 1},
		{Name: "upstream2", URL: "http://example2.com", Weight: 1},
		{Name: "upstream3", URL: "http://example3.com", Weight: 1},
	}

	balancer := NewRandomBalancer()
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = balancer.Select(ctx, upstreams)
		}
	})
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
		NewRRBalancer(),
		NewWeightedRRBalancer(),
		NewRandomBalancer(),
		NewIPHashBalancer(),
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
		NewRRBalancer(),
		NewWeightedRRBalancer(),
		NewRandomBalancer(),
		NewIPHashBalancer(),
	}

	for _, balancer := range balancers {
		t.Run(balancer.Type()+"_UpdateLatency", func(t *testing.T) {
			// Should not panic
			balancer.UpdateLatency("upstream1", 100)
			balancer.UpdateLatency("upstream1", 200)
		})
	}
}

func TestBalancersWithConfigDefaults(t *testing.T) {
	// Test balancers with config.default.yaml strategy values
	strategies := []string{
		"roundrobin",
		"weighted_roundrobin",
		"random",
		"iphash",
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
