package metrics

import (
	"errors"
	"fmt"
)

// 工厂相关错误定义
var (
	ErrInvalidMetricsType    = errors.New("invalid metrics type")
	ErrNilConfig             = errors.New("metrics config cannot be nil")
	ErrInvalidConfig         = errors.New("invalid metrics config")
	ErrMetricsDisabled       = errors.New("metrics collection is disabled")
	ErrMetricsTypeEmpty      = errors.New("metrics type cannot be empty")
	ErrMetricsNamespaceEmpty = errors.New("metrics namespace cannot be empty")
)

const NoopType = "noop"

// metricsFactory 代表指标收集器工厂实现
type metricsFactory struct{}

// NewFactory 创建新的指标收集器工厂实例
func NewFactory() MetricsCollectorFactory {
	return &metricsFactory{}
}

// Create 根据配置创建对应的指标收集器
// config: 指标收集器配置信息
func (f *metricsFactory) Create(config *Config) (MetricsCollector, error) {
	if config == nil {
		return nil, ErrNilConfig
	}

	// 检查是否启用指标收集
	if !config.Enabled {
		return NewNoopCollector(), nil
	}

	// 验证配置
	if err := f.validateConfig(config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	switch config.Type {
	case NoopType, "":
		// 默认使用空操作收集器
		return NewNoopCollector(), nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidMetricsType, config.Type)
	}
}

// validateConfig 验证配置的有效性
func (f *metricsFactory) validateConfig(config *Config) error {
	if config.Type == "" {
		return ErrMetricsTypeEmpty
	}

	if config.Namespace == "" {
		return ErrMetricsNamespaceEmpty
	}

	// 验证命名空间格式（只允许字母、数字和下划线）
	for _, r := range config.Namespace {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return fmt.Errorf("invalid namespace format: %s", config.Namespace)
		}
	}

	// 验证子系统格式（如果提供）
	if config.Subsystem != "" {
		for _, r := range config.Subsystem {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return fmt.Errorf("invalid subsystem format: %s", config.Subsystem)
			}
		}
	}

	return nil
}

// CreateFromDefaults 使用默认配置创建指标收集器的便捷方法
func CreateFromDefaults() (MetricsCollector, error) {
	factory := NewFactory()
	return factory.Create(DefaultConfig())
}

// CreateNoopCollector 创建空操作收集器的便捷方法
func CreateNoopCollector() (MetricsCollector, error) {
	config := DefaultConfig()
	config.Type = NoopType

	factory := NewFactory()
	return factory.Create(config)
}
