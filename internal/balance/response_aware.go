package balance

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/shengyanli1982/llmproxy-go/internal/breaker"
	"github.com/sony/gobreaker"
)

// LatencyRecord 延迟记录结构
type LatencyRecord struct {
	Latency   int64     // 延迟时间（毫秒）
	Timestamp time.Time // 记录时间
}

// SlidingWindow 滑动窗口结构
type SlidingWindow struct {
	records    []LatencyRecord // 延迟记录
	windowSize int             // 窗口大小
	mu         sync.Mutex      // 保护并发访问
}

// NewSlidingWindow 创建滑动窗口
func NewSlidingWindow(size int) *SlidingWindow {
	return &SlidingWindow{
		records:    make([]LatencyRecord, 0, size),
		windowSize: size,
	}
}

// Add 添加延迟记录
func (sw *SlidingWindow) Add(latency int64) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	record := LatencyRecord{
		Latency:   latency,
		Timestamp: now,
	}

	// 清理过期记录（超过5分钟）
	expireTime := now.Add(-5 * time.Minute)
	var validRecords []LatencyRecord
	for _, r := range sw.records {
		if r.Timestamp.After(expireTime) {
			validRecords = append(validRecords, r)
		}
	}

	// 添加新记录
	validRecords = append(validRecords, record)

	// 保持窗口大小
	if len(validRecords) > sw.windowSize {
		validRecords = validRecords[len(validRecords)-sw.windowSize:]
	}

	sw.records = validRecords
}

// Average 计算平均延迟
func (sw *SlidingWindow) Average() int64 {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if len(sw.records) == 0 {
		return 1000 // 默认1秒
	}

	var total int64
	for _, record := range sw.records {
		total += record.Latency
	}

	return total / int64(len(sw.records))
}

// responseAwareBalancer 实现响应感知负载均衡算法
// 根据上游服务的响应时间选择最快的服务，集成熔断器保护
type responseAwareBalancer struct {
	mu          sync.RWMutex                   // 读写锁，保护并发访问
	latencyMap  map[string]*SlidingWindow     // 延迟滑动窗口映射
	healthMap   map[string]bool               // 健康状态映射
	breakerMap  map[string]breaker.CircuitBreaker // 熔断器映射
	breakerFactory breaker.CircuitBreakerFactory // 熔断器工厂
}

// NewResponseAwareBalancer 创建新的响应感知负载均衡器实例
func NewResponseAwareBalancer() LoadBalancer {
	return &responseAwareBalancer{
		latencyMap:     make(map[string]*SlidingWindow),
		healthMap:      make(map[string]bool),
		breakerMap:     make(map[string]breaker.CircuitBreaker),
		breakerFactory: breaker.NewFactory(),
	}
}

// Select 使用响应感知算法选择上游服务
// 选择响应时间最短的健康服务，考虑熔断器状态
func (b *responseAwareBalancer) Select(ctx context.Context, upstreams []Upstream) (Upstream, error) {
	if upstreams == nil {
		return Upstream{}, ErrNilUpstreams
	}
	if len(upstreams) == 0 {
		return Upstream{}, ErrEmptyUpstreams
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	// 过滤健康且熔断器允许的上游服务
	var availableUpstreams []Upstream
	for _, upstream := range upstreams {
		// 检查健康状态
		if healthy, exists := b.healthMap[upstream.Name]; exists && !healthy {
			continue
		}

		// 检查熔断器状态
		if cb, exists := b.breakerMap[upstream.Name]; exists {
			if cb.State() == gobreaker.StateOpen {
				continue // 熔断器开启，跳过此上游
			}
		}

		availableUpstreams = append(availableUpstreams, upstream)
	}

	// 如果没有可用的服务，返回第一个作为最后尝试
	if len(availableUpstreams) == 0 {
		return upstreams[0], nil
	}

	// 按平均延迟排序选择最快的服务
	sort.Slice(availableUpstreams, func(i, j int) bool {
		latencyI := b.getAverageLatency(availableUpstreams[i].Name)
		latencyJ := b.getAverageLatency(availableUpstreams[j].Name)
		return latencyI < latencyJ
	})

	return availableUpstreams[0], nil
}

// getAverageLatency 获取上游服务的平均延迟
func (b *responseAwareBalancer) getAverageLatency(upstreamName string) int64 {
	if window, exists := b.latencyMap[upstreamName]; exists {
		return window.Average()
	}
	return 1000 // 默认延迟1秒
}

// UpdateHealth 更新上游服务的健康状态
func (b *responseAwareBalancer) UpdateHealth(upstreamName string, healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.healthMap[upstreamName] = healthy
}

// UpdateLatency 更新上游服务的延迟信息
func (b *responseAwareBalancer) UpdateLatency(upstreamName string, latency int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 获取或创建滑动窗口
	window, exists := b.latencyMap[upstreamName]
	if !exists {
		window = NewSlidingWindow(10) // 保持最近10次记录
		b.latencyMap[upstreamName] = window
	}

	window.Add(latency)
}

// CreateBreaker 为上游服务创建熔断器
func (b *responseAwareBalancer) CreateBreaker(upstreamName string, settings gobreaker.Settings) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	cb, err := b.breakerFactory.Create(upstreamName, settings)
	if err != nil {
		return err
	}

	b.breakerMap[upstreamName] = cb
	return nil
}

// GetBreaker 获取指定上游的熔断器
func (b *responseAwareBalancer) GetBreaker(upstreamName string) (breaker.CircuitBreaker, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	cb, exists := b.breakerMap[upstreamName]
	return cb, exists
}

// Type 获取负载均衡器类型
func (b *responseAwareBalancer) Type() string {
	return "response_aware"
}