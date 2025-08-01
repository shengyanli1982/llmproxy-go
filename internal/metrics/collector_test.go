package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// createTestCollector 创建用于测试的 Prometheus 收集器
func createTestCollector(t *testing.T, namespace, subsystem string) MetricsCollector {
	config := &Config{
		Type:      "prometheus",
		Enabled:   true,
		Namespace: namespace,
		Subsystem: subsystem,
	}

	registry := prometheus.NewRegistry()
	collector, err := NewPrometheusCollectorWithRegistry(config, registry)
	if err != nil {
		t.Fatalf("Failed to create test collector: %v", err)
	}
	return collector
}

// TestNewPrometheusCollectorWithRegistry 测试创建使用指定注册器的 Prometheus 收集器
func TestNewPrometheusCollectorWithRegistry(t *testing.T) {
	config := &Config{
		Type:      "prometheus",
		Enabled:   true,
		Namespace: "test",
		Subsystem: "",
	}

	registry := prometheus.NewRegistry()
	collector, err := NewPrometheusCollectorWithRegistry(config, registry)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if collector == nil {
		t.Fatal("Expected collector to be created, got nil")
	}
	if collector.Name() != "prometheus" {
		t.Errorf("Expected collector name to be 'prometheus', got %s", collector.Name())
	}
}

// TestNewPrometheusCollectorWithRegistry_NilConfig 测试空配置
func TestNewPrometheusCollectorWithRegistry_NilConfig(t *testing.T) {
	registry := prometheus.NewRegistry()
	_, err := NewPrometheusCollectorWithRegistry(nil, registry)
	if err != ErrNilConfig {
		t.Errorf("Expected ErrNilConfig, got %v", err)
	}
}

// TestNewPrometheusCollectorWithRegistry_NilRegistry 测试空注册器
func TestNewPrometheusCollectorWithRegistry_NilRegistry(t *testing.T) {
	config := &Config{
		Type:      "prometheus",
		Enabled:   true,
		Namespace: "test",
		Subsystem: "",
	}

	_, err := NewPrometheusCollectorWithRegistry(config, nil)
	if err == nil {
		t.Error("Expected error for nil registry, got nil")
	}
}

// TestPrometheusCollector_HTTPMetrics 测试 HTTP 指标收集
func TestPrometheusCollector_HTTPMetrics(t *testing.T) {
	collector := createTestCollector(t, "test", "")

	// 记录 HTTP 响应
	collector.RecordResponse("test-forward", "POST", "/v1/chat", 200, 100*time.Millisecond, 1024, 2048)

	// 验证指标是否正确记录
	registry := collector.GetRegistry()
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// 检查是否有指标被记录
	found := false
	for _, mf := range metricFamilies {
		if strings.Contains(mf.GetName(), "http_requests_total") {
			found = true
			if len(mf.GetMetric()) == 0 {
				t.Error("Expected at least one metric sample")
			}
			break
		}
	}
	if !found {
		t.Error("Expected to find http_requests_total metric")
	}
}

// TestPrometheusCollector_UpstreamMetrics 测试上游指标收集
func TestPrometheusCollector_UpstreamMetrics(t *testing.T) {
	collector := createTestCollector(t, "test", "")

	// 记录上游响应
	collector.RecordUpstreamResponse("openai-group", "openai-primary", "POST", 200, 500*time.Millisecond)

	// 记录上游错误
	collector.RecordUpstreamError("openai-group", "openai-primary", "timeout")

	// 验证指标是否正确记录
	registry := collector.GetRegistry()
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// 检查上游请求指标
	foundRequests := false
	foundErrors := false
	for _, mf := range metricFamilies {
		if strings.Contains(mf.GetName(), "upstream_requests_total") {
			foundRequests = true
		}
		if strings.Contains(mf.GetName(), "upstream_errors_total") {
			foundErrors = true
		}
	}
	if !foundRequests {
		t.Error("Expected to find upstream_requests_total metric")
	}
	if !foundErrors {
		t.Error("Expected to find upstream_errors_total metric")
	}
}

// TestPrometheusCollector_CircuitBreakerMetrics 测试断路器指标收集
func TestPrometheusCollector_CircuitBreakerMetrics(t *testing.T) {
	collector := createTestCollector(t, "test", "")

	// 记录断路器状态
	collector.RecordCircuitBreakerState("openai-group", "openai-primary", 1) // half-open

	// 记录断路器请求
	collector.RecordCircuitBreakerRequest("openai-group", "openai-primary", "success")

	// 记录断路器状态变化
	collector.RecordCircuitBreakerStateChange("openai-group", "openai-primary", "closed", "half-open")

	// 验证指标是否正确记录
	registry := collector.GetRegistry()
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	foundState := false
	foundRequests := false
	foundChanges := false
	for _, mf := range metricFamilies {
		name := mf.GetName()
		if strings.Contains(name, "circuit_breaker_state") && !strings.Contains(name, "changes") {
			foundState = true
		}
		if strings.Contains(name, "circuit_breaker_requests_total") {
			foundRequests = true
		}
		if strings.Contains(name, "circuit_breaker_state_changes_total") {
			foundChanges = true
		}
	}
	if !foundState {
		t.Error("Expected to find circuit_breaker_state metric")
	}
	if !foundRequests {
		t.Error("Expected to find circuit_breaker_requests_total metric")
	}
	if !foundChanges {
		t.Error("Expected to find circuit_breaker_state_changes_total metric")
	}
}

// TestPrometheusCollector_LoadBalancerMetrics 测试负载均衡器指标收集
func TestPrometheusCollector_LoadBalancerMetrics(t *testing.T) {
	collector := createTestCollector(t, "test", "")

	// 记录负载均衡器选择
	collector.RecordLoadBalancerSelection("openai-group", "openai-primary", "round_robin")

	// 记录上游健康状态
	collector.RecordUpstreamHealthStatus("openai-group", "openai-primary", true)

	// 验证指标是否正确记录
	registry := collector.GetRegistry()
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	foundSelections := false
	foundHealth := false
	for _, mf := range metricFamilies {
		name := mf.GetName()
		if strings.Contains(name, "load_balancer_selections_total") {
			foundSelections = true
		}
		if strings.Contains(name, "upstream_health_status") {
			foundHealth = true
		}
	}
	if !foundSelections {
		t.Error("Expected to find load_balancer_selections_total metric")
	}
	if !foundHealth {
		t.Error("Expected to find upstream_health_status metric")
	}
}

// TestPrometheusCollector_SystemMetrics 测试系统级指标收集
func TestPrometheusCollector_SystemMetrics(t *testing.T) {
	collector := createTestCollector(t, "test", "")

	// 记录活跃连接数
	collector.RecordActiveConnections("test-forward", 10)

	// 记录限流拒绝
	collector.RecordRateLimitRejection("test-forward", "ip")

	// 验证指标是否正确记录
	registry := collector.GetRegistry()
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	foundConnections := false
	foundRejections := false
	for _, mf := range metricFamilies {
		name := mf.GetName()
		if strings.Contains(name, "active_connections") {
			foundConnections = true
		}
		if strings.Contains(name, "rate_limit_rejections_total") {
			foundRejections = true
		}
	}
	if !foundConnections {
		t.Error("Expected to find active_connections metric")
	}
	if !foundRejections {
		t.Error("Expected to find rate_limit_rejections_total metric")
	}
}

// TestPrometheusCollector_MetricNaming 测试指标命名
func TestPrometheusCollector_MetricNaming(t *testing.T) {
	collector := createTestCollector(t, "llmproxy", "test")

	// 记录一些指标
	collector.RecordResponse("test-forward", "POST", "/v1/chat", 200, 100*time.Millisecond, 1024, 2048)

	// 验证指标命名
	registry := collector.GetRegistry()
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// 检查指标名称是否包含正确的前缀
	for _, mf := range metricFamilies {
		name := mf.GetName()
		if !strings.HasPrefix(name, "llmproxy_test_") {
			t.Errorf("Expected metric name to start with 'llmproxy_test_', got %s", name)
		}
	}
}

// TestPrometheusCollector_Close 测试关闭收集器
func TestPrometheusCollector_Close(t *testing.T) {
	collector := createTestCollector(t, "test", "")

	err := collector.Close()
	if err != nil {
		t.Errorf("Expected no error when closing collector, got %v", err)
	}
}

// TestPrometheusCollector_ConcurrentAccess 测试并发安全
func TestPrometheusCollector_ConcurrentAccess(t *testing.T) {
	collector := createTestCollector(t, "test", "")

	// 并发写入指标
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				collector.RecordResponse("test-forward", "POST", "/v1/chat", 200, 100*time.Millisecond, 1024, 2048)
				collector.RecordUpstreamResponse("test-group", "test-upstream", "POST", 200, 500*time.Millisecond)
				collector.RecordCircuitBreakerState("test-group", "test-upstream", 0)
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证指标是否正确记录
	registry := collector.GetRegistry()
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(metricFamilies) == 0 {
		t.Error("Expected metrics to be recorded")
	}
}
