package metrics

import (
	"testing"
)

// TestNewFactory 测试工厂创建
func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	if factory == nil {
		t.Fatal("Expected factory to be created, got nil")
	}
}

// TestFactory_Create_NoopCollector 测试创建空操作收集器
func TestFactory_Create_NoopCollector(t *testing.T) {
	factory := NewFactory()

	// 测试 noop 类型
	config := &Config{
		Type:      "noop",
		Enabled:   true,
		Namespace: "test",
		Subsystem: "",
	}

	collector, err := factory.Create(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if collector == nil {
		t.Fatal("Expected collector to be created, got nil")
	}
	if collector.Name() != "noop" {
		t.Errorf("Expected collector name to be 'noop', got %s", collector.Name())
	}
}

// TestFactory_Create_DisabledCollector 测试禁用指标收集
func TestFactory_Create_DisabledCollector(t *testing.T) {
	factory := NewFactory()

	config := &Config{
		Type:      "prometheus",
		Enabled:   false, // 禁用指标收集
		Namespace: "test",
		Subsystem: "",
	}

	collector, err := factory.Create(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if collector == nil {
		t.Fatal("Expected collector to be created, got nil")
	}
	// 禁用时应该返回 noop 收集器
	if collector.Name() != "noop" {
		t.Errorf("Expected collector name to be 'noop' when disabled, got %s", collector.Name())
	}
}

// TestFactory_Create_ValidationErrors 测试配置验证错误
func TestFactory_Create_ValidationErrors(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorType   error
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorType:   ErrNilConfig,
		},
		{
			name: "empty namespace",
			config: &Config{
				Type:      "prometheus",
				Enabled:   true,
				Namespace: "",
				Subsystem: "",
			},
			expectError: true,
			errorType:   ErrInvalidConfig,
		},
		{
			name: "invalid namespace format",
			config: &Config{
				Type:      "prometheus",
				Enabled:   true,
				Namespace: "test-invalid",
				Subsystem: "",
			},
			expectError: true,
			errorType:   ErrInvalidConfig,
		},
		{
			name: "invalid subsystem format",
			config: &Config{
				Type:      "prometheus",
				Enabled:   true,
				Namespace: "test",
				Subsystem: "sub-invalid",
			},
			expectError: true,
			errorType:   ErrInvalidConfig,
		},
		{
			name: "unknown type",
			config: &Config{
				Type:      "unknown",
				Enabled:   true,
				Namespace: "test",
				Subsystem: "",
			},
			expectError: true,
			errorType:   ErrInvalidMetricsType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory.Create(tt.config)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config == nil {
		t.Fatal("Expected default config to be created, got nil")
	}
	if config.Type != "noop" {
		t.Errorf("Expected default type to be 'noop', got %s", config.Type)
	}
	if !config.Enabled {
		t.Error("Expected default enabled to be true")
	}
	if config.Namespace != "llmproxy" {
		t.Errorf("Expected default namespace to be 'llmproxy', got %s", config.Namespace)
	}
}

// TestCreateFromDefaults 测试使用默认配置创建收集器
func TestCreateFromDefaults(t *testing.T) {
	collector, err := CreateFromDefaults()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if collector == nil {
		t.Fatal("Expected collector to be created, got nil")
	}
}

// TestCreateNoopCollector 测试创建空操作收集器的便捷方法
func TestCreateNoopCollector(t *testing.T) {
	collector, err := CreateNoopCollector()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if collector == nil {
		t.Fatal("Expected collector to be created, got nil")
	}
	if collector.Name() != "noop" {
		t.Errorf("Expected collector name to be 'noop', got %s", collector.Name())
	}
}

// TestFactory_Create_PrometheusCollector 测试创建 Prometheus 收集器
func TestFactory_Create_PrometheusCollector(t *testing.T) {
	factory := NewFactory()

	config := &Config{
		Type:      "prometheus",
		Enabled:   true,
		Namespace: "test",
		Subsystem: "",
	}

	// Since factory no longer supports prometheus type directly,
	// it should return noop collector
	collector, err := factory.Create(config)
	if err == nil {
		t.Error("Expected error for unsupported prometheus type, got nil")
	}
	if collector != nil {
		t.Error("Expected nil collector for unsupported prometheus type")
	}
}

