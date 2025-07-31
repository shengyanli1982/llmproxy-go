package balance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

	t.Run("WithClientIP IPv6 address", func(t *testing.T) {
		testIPv6 := "2001:db8::1"

		ctxWithIPv6 := WithClientIP(ctx, testIPv6)
		assert.NotNil(t, ctxWithIPv6)

		retrievedIP, ok := GetClientIP(ctxWithIPv6)
		assert.True(t, ok)
		assert.Equal(t, testIPv6, retrievedIP)
	})

	t.Run("Context key isolation", func(t *testing.T) {
		// 测试 context key 的隔离性，确保不会与其他 key 冲突
		testIP := "10.0.0.1"

		// 使用字符串作为 key 存储其他值
		ctxWithOtherValue := context.WithValue(ctx, "clientIP", "other-value")
		ctxWithIP := WithClientIP(ctxWithOtherValue, testIP)

		// 应该能正确获取我们存储的 IP
		retrievedIP, ok := GetClientIP(ctxWithIP)
		assert.True(t, ok)
		assert.Equal(t, testIP, retrievedIP)

		// 其他 key 的值不应该受影响
		otherValue := ctxWithIP.Value("clientIP")
		assert.Equal(t, "other-value", otherValue)
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

	t.Run("different IPs select different upstreams", func(t *testing.T) {
		selections := make(map[string]string)

		// 测试多个不同的 IP
		testIPs := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3", "10.0.0.1", "10.0.0.2"}

		for _, ip := range testIPs {
			ctx := WithClientIP(context.Background(), ip)
			upstream, err := balancer.Select(ctx, upstreams)
			assert.NoError(t, err)
			selections[ip] = upstream.Name
		}

		// 验证至少有一些不同的选择（不是所有IP都选择同一个上游）
		uniqueUpstreams := make(map[string]bool)
		for _, upstream := range selections {
			uniqueUpstreams[upstream] = true
		}

		// 应该有多个不同的上游被选择
		assert.Greater(t, len(uniqueUpstreams), 1, "Different IPs should distribute across multiple upstreams")
	})

	t.Run("no client IP fallback to random", func(t *testing.T) {
		ctx := context.Background() // 没有客户端 IP

		upstream, err := balancer.Select(ctx, upstreams)
		assert.NoError(t, err)
		assert.Contains(t, []string{"upstream1", "upstream2", "upstream3"}, upstream.Name)
	})

	t.Run("empty client IP fallback to random", func(t *testing.T) {
		ctx := WithClientIP(context.Background(), "")

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
