package metrics

import (
	"fmt"
	"sync"
	"testing"
)

// TestGetGlobalRegistry 测试全局单例注册器
func TestGetGlobalRegistry(t *testing.T) {
	registry1 := GetGlobalRegistry()
	registry2 := GetGlobalRegistry()

	if registry1 != registry2 {
		t.Error("Expected same instance for global registry")
	}

	if registry1 == nil {
		t.Error("Expected global registry to be initialized")
	}
}

// TestNewMetricsRegistry 测试创建新的注册器实例
func TestNewMetricsRegistry(t *testing.T) {
	registry := NewMetricsRegistry()
	if registry == nil {
		t.Fatal("Expected registry to be created")
	}

	if registry.CollectorCount() != 0 {
		t.Error("Expected new registry to be empty")
	}

	if !registry.IsEmpty() {
		t.Error("Expected new registry to be empty")
	}
}

// TestMetricsRegistry_RegisterCollector 测试注册收集器
func TestMetricsRegistry_RegisterCollector(t *testing.T) {
	registry := NewMetricsRegistry()

	// 创建测试收集器
	collector, err := CreateNoopCollector()
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	// 注册收集器
	err = registry.RegisterCollector("test-collector", collector)
	if err != nil {
		t.Fatalf("Failed to register collector: %v", err)
	}

	// 验证注册成功
	if registry.CollectorCount() != 1 {
		t.Errorf("Expected 1 collector, got %d", registry.CollectorCount())
	}

	if !registry.HasCollector("test-collector") {
		t.Error("Expected collector to be registered")
	}
}

// TestMetricsRegistry_RegisterCollector_Errors 测试注册收集器的错误情况
func TestMetricsRegistry_RegisterCollector_Errors(t *testing.T) {
	registry := NewMetricsRegistry()

	tests := []struct {
		name          string
		collectorName string
		collector     MetricsCollector
		expectError   error
	}{
		{
			name:          "empty name",
			collectorName: "",
			collector:     &noopCollector{name: "test"},
			expectError:   ErrEmptyCollectorName,
		},
		{
			name:          "nil collector",
			collectorName: "test",
			collector:     nil,
			expectError:   ErrNilCollector,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.RegisterCollector(tt.collectorName, tt.collector)
			if err != tt.expectError {
				t.Errorf("Expected error %v, got %v", tt.expectError, err)
			}
		})
	}
}

// TestMetricsRegistry_RegisterCollector_Duplicate 测试重复注册
func TestMetricsRegistry_RegisterCollector_Duplicate(t *testing.T) {
	registry := NewMetricsRegistry()

	collector, err := CreateNoopCollector()
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	// 第一次注册
	err = registry.RegisterCollector("test-collector", collector)
	if err != nil {
		t.Fatalf("Failed to register collector: %v", err)
	}

	// 第二次注册相同名称
	err = registry.RegisterCollector("test-collector", collector)
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}
}

// TestMetricsRegistry_GetCollector 测试获取收集器
func TestMetricsRegistry_GetCollector(t *testing.T) {
	registry := NewMetricsRegistry()

	collector, err := CreateNoopCollector()
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	// 注册收集器
	err = registry.RegisterCollector("test-collector", collector)
	if err != nil {
		t.Fatalf("Failed to register collector: %v", err)
	}

	// 获取存在的收集器
	retrieved, exists := registry.GetCollector("test-collector")
	if !exists {
		t.Error("Expected collector to exist")
	}
	if retrieved != collector {
		t.Error("Expected same collector instance")
	}

	// 获取不存在的收集器
	_, exists = registry.GetCollector("non-existent")
	if exists {
		t.Error("Expected collector to not exist")
	}
}

// TestMetricsRegistry_UnregisterCollector 测试注销收集器
func TestMetricsRegistry_UnregisterCollector(t *testing.T) {
	registry := NewMetricsRegistry()

	collector, err := CreateNoopCollector()
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	// 注册收集器
	err = registry.RegisterCollector("test-collector", collector)
	if err != nil {
		t.Fatalf("Failed to register collector: %v", err)
	}

	// 注销收集器
	err = registry.UnregisterCollector("test-collector")
	if err != nil {
		t.Errorf("Failed to unregister collector: %v", err)
	}

	// 验证注销成功
	if registry.HasCollector("test-collector") {
		t.Error("Expected collector to be unregistered")
	}

	// 注销不存在的收集器
	err = registry.UnregisterCollector("non-existent")
	if err == nil {
		t.Error("Expected error for unregistering non-existent collector")
	}
}

// TestMetricsRegistry_ListCollectors 测试列出收集器
func TestMetricsRegistry_ListCollectors(t *testing.T) {
	registry := NewMetricsRegistry()

	// 空注册器
	names := registry.ListCollectors()
	if len(names) != 0 {
		t.Errorf("Expected 0 collectors, got %d", len(names))
	}

	// 添加收集器
	collector1, _ := CreateNoopCollector()
	collector2, _ := CreateNoopCollector()

	registry.RegisterCollector("collector1", collector1)
	registry.RegisterCollector("collector2", collector2)

	names = registry.ListCollectors()
	if len(names) != 2 {
		t.Errorf("Expected 2 collectors, got %d", len(names))
	}

	// 验证名称
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	if !nameMap["collector1"] || !nameMap["collector2"] {
		t.Error("Expected both collector names to be present")
	}
}

// TestMetricsRegistry_Clear 测试清除所有收集器
func TestMetricsRegistry_Clear(t *testing.T) {
	registry := NewMetricsRegistry()

	// 添加收集器
	collector1, _ := CreateNoopCollector()
	collector2, _ := CreateNoopCollector()

	registry.RegisterCollector("collector1", collector1)
	registry.RegisterCollector("collector2", collector2)

	// 清除所有收集器
	err := registry.Clear()
	if err != nil {
		t.Errorf("Failed to clear registry: %v", err)
	}

	// 验证清除成功
	if registry.CollectorCount() != 0 {
		t.Errorf("Expected 0 collectors after clear, got %d", registry.CollectorCount())
	}

	if !registry.IsEmpty() {
		t.Error("Expected registry to be empty after clear")
	}
}

// TestMetricsRegistry_ConcurrentAccess 测试并发访问
func TestMetricsRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewMetricsRegistry()

	// 并发注册收集器
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			collector, err := CreateNoopCollector()
			if err != nil {
				t.Errorf("Failed to create collector: %v", err)
				return
			}

			collectorName := fmt.Sprintf("collector-%d", id)
			err = registry.RegisterCollector(collectorName, collector)
			if err != nil {
				t.Errorf("Failed to register collector %s: %v", collectorName, err)
				return
			}

			// 验证注册成功
			if !registry.HasCollector(collectorName) {
				t.Errorf("Collector %s not found after registration", collectorName)
			}
		}(i)
	}

	wg.Wait()

	// 验证所有收集器都已注册
	if registry.CollectorCount() != numGoroutines {
		t.Errorf("Expected %d collectors, got %d", numGoroutines, registry.CollectorCount())
	}
}

// TestMetricsRegistry_CreateSharedCollector 测试创建共享收集器
func TestMetricsRegistry_CreateSharedCollector(t *testing.T) {
	registry := NewMetricsRegistry()

	config := &Config{
		Type:      "noop",
		Enabled:   true,
		Namespace: "test",
		Subsystem: "",
	}

	collector, err := registry.CreateSharedCollector("shared-collector", config)
	if err != nil {
		t.Fatalf("Failed to create shared collector: %v", err)
	}

	if collector == nil {
		t.Fatal("Expected collector to be created")
	}

	// 验证收集器已注册
	if !registry.HasCollector("shared-collector") {
		t.Error("Expected shared collector to be registered")
	}
}

// TestMetricsRegistry_GatherAll 测试收集所有指标
func TestMetricsRegistry_GatherAll(t *testing.T) {
	registry := NewMetricsRegistry()

	// 收集空注册器的指标
	metrics, err := registry.GatherAll()
	if err != nil {
		t.Errorf("Failed to gather metrics: %v", err)
	}

	if metrics == nil {
		t.Error("Expected metrics to be returned")
	}
}

// TestMetricsRegistry_MemoryLeak 测试内存泄漏
func TestMetricsRegistry_MemoryLeak(t *testing.T) {
	registry := NewMetricsRegistry()

	// 大量注册和注销操作
	for i := 0; i < 1000; i++ {
		collector, err := CreateNoopCollector()
		if err != nil {
			t.Fatalf("Failed to create collector: %v", err)
		}

		collectorName := fmt.Sprintf("collector-%d", i)

		// 注册
		err = registry.RegisterCollector(collectorName, collector)
		if err != nil {
			t.Fatalf("Failed to register collector: %v", err)
		}

		// 立即注销
		err = registry.UnregisterCollector(collectorName)
		if err != nil {
			t.Fatalf("Failed to unregister collector: %v", err)
		}
	}

	// 验证注册器为空
	if !registry.IsEmpty() {
		t.Error("Expected registry to be empty after all operations")
	}

	if registry.CollectorCount() != 0 {
		t.Errorf("Expected 0 collectors, got %d", registry.CollectorCount())
	}
}
