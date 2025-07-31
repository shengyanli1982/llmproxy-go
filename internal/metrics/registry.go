package metrics

import (
	"errors"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// 注册器相关错误定义
var (
	ErrCollectorAlreadyRegistered = errors.New("collector already registered")
	ErrCollectorNotFound          = errors.New("collector not found")
	ErrEmptyCollectorName         = errors.New("collector name cannot be empty")
	ErrNilCollector               = errors.New("collector cannot be nil")
)

// MetricsRegistry 代表指标注册管理器，负责管理多个指标收集器
type MetricsRegistry struct {
	mu         sync.RWMutex
	registry   *prometheus.Registry
	collectors map[string]MetricsCollector
}

// 全局单例实例
var (
	globalRegistry *MetricsRegistry
	registryOnce   sync.Once
)

// GetGlobalRegistry 获取全局单例注册器实例
func GetGlobalRegistry() *MetricsRegistry {
	registryOnce.Do(func() {
		globalRegistry = &MetricsRegistry{
			registry:   prometheus.NewRegistry(),
			collectors: make(map[string]MetricsCollector),
		}
	})
	return globalRegistry
}

// NewMetricsRegistry 创建新的指标注册器实例（用于测试或特殊场景）
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		registry:   prometheus.NewRegistry(),
		collectors: make(map[string]MetricsCollector),
	}
}

// RegisterCollector 注册指标收集器
// name: 收集器名称，必须唯一
// collector: 指标收集器实例
func (r *MetricsRegistry) RegisterCollector(name string, collector MetricsCollector) error {
	if name == "" {
		return ErrEmptyCollectorName
	}
	if collector == nil {
		return ErrNilCollector
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否已经注册
	if _, exists := r.collectors[name]; exists {
		return fmt.Errorf("%w: %s", ErrCollectorAlreadyRegistered, name)
	}

	// 注意：我们不直接合并注册器，而是让收集器使用共享的注册器
	// 这样可以避免复杂的合并逻辑，并确保指标的一致性

	// 注册收集器
	r.collectors[name] = collector

	return nil
}

// GetCollector 获取指定名称的指标收集器
// name: 收集器名称
// 返回收集器实例和是否存在的标志
func (r *MetricsRegistry) GetCollector(name string) (MetricsCollector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	collector, exists := r.collectors[name]
	return collector, exists
}

// UnregisterCollector 注销指标收集器
// name: 收集器名称
func (r *MetricsRegistry) UnregisterCollector(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	collector, exists := r.collectors[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrCollectorNotFound, name)
	}

	// 关闭收集器
	if err := collector.Close(); err != nil {
		return fmt.Errorf("failed to close collector %s: %w", name, err)
	}

	// 从映射中删除
	delete(r.collectors, name)

	return nil
}

// GetRegistry 获取 Prometheus 注册器
func (r *MetricsRegistry) GetRegistry() *prometheus.Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.registry
}

// ListCollectors 获取所有已注册收集器的名称列表
func (r *MetricsRegistry) ListCollectors() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.collectors))
	for name := range r.collectors {
		names = append(names, name)
	}
	return names
}

// CollectorCount 获取已注册收集器的数量
func (r *MetricsRegistry) CollectorCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.collectors)
}

// Clear 清除所有已注册的收集器
func (r *MetricsRegistry) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 关闭所有收集器
	var errs []error
	for name, collector := range r.collectors {
		if err := collector.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close collector %s: %w", name, err))
		}
	}

	// 清空映射
	r.collectors = make(map[string]MetricsCollector)

	// 创建新的注册器
	r.registry = prometheus.NewRegistry()

	// 如果有错误，返回第一个错误
	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// CreateSharedCollector 创建一个使用共享注册器的收集器
// 这是推荐的方式，避免了复杂的注册器合并逻辑
func (r *MetricsRegistry) CreateSharedCollector(name string, config *Config) (MetricsCollector, error) {
	if name == "" {
		return nil, ErrEmptyCollectorName
	}
	if config == nil {
		return nil, ErrNilConfig
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否已经注册
	if _, exists := r.collectors[name]; exists {
		return nil, fmt.Errorf("%w: %s", ErrCollectorAlreadyRegistered, name)
	}

	// 创建一个使用共享注册器的收集器
	collector, err := r.createCollectorWithSharedRegistry(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create collector %s: %w", name, err)
	}

	// 注册收集器
	r.collectors[name] = collector

	return collector, nil
}

// createCollectorWithSharedRegistry 创建使用共享注册器的收集器
func (r *MetricsRegistry) createCollectorWithSharedRegistry(config *Config) (MetricsCollector, error) {
	// 使用新的构造函数创建使用共享注册器的收集器
	return NewPrometheusCollectorWithRegistry(config, r.registry)
}

// GatherAll 收集所有注册收集器的指标
func (r *MetricsRegistry) GatherAll() (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.registry.Gather()
}

// HasCollector 检查是否存在指定名称的收集器
func (r *MetricsRegistry) HasCollector(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.collectors[name]
	return exists
}

// IsEmpty 检查注册器是否为空
func (r *MetricsRegistry) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.collectors) == 0
}
