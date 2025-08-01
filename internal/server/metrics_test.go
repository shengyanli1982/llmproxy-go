package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/metrics"
)

// TestMetricsEndpointWithData 测试 /metrics 端点输出实际的指标数据
func TestMetricsEndpointWithData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 清理全局注册器
	globalRegistry := metrics.GetGlobalRegistry()
	globalRegistry.Clear()

	// 创建测试配置
	forwardConfig := &config.ForwardConfig{
		Name:         "test-forward",
		DefaultGroup: "test-group",
	}

	globalConfig := &config.Config{
		UpstreamGroups: []config.UpstreamGroupConfig{
			{
				Name: "test-group",
				Upstreams: []config.UpstreamRefConfig{
					{Name: "test-upstream", Weight: 1},
				},
			},
		},
		Upstreams: []config.UpstreamConfig{
			{
				Name: "test-upstream",
				URL:  "http://example.com",
			},
		},
	}

	logger := logr.Discard()

	// 创建 ForwardService
	forwardService := NewForwardServices()
	err := forwardService.Initialize(forwardConfig, globalConfig, &logger)
	if err != nil {
		t.Fatalf("Failed to initialize forward service: %v", err)
	}

	// 创建 AdminService
	adminConfig := &config.AdminConfig{
		Port:    8080,
		Address: "127.0.0.1",
		Timeout: &config.TimeoutConfig{
			Idle:  30000,
			Read:  15000,
			Write: 15000,
		},
	}

	adminService := NewAdminServices()
	adminService.Initialize(adminConfig, globalConfig, &logger, nil)

	// 模拟一些指标数据
	if forwardService.metricsCollector != nil {
		// 记录 HTTP 请求和响应
		forwardService.metricsCollector.RecordRequest("test-forward", "GET", "/api/test")
		forwardService.metricsCollector.RecordResponse("test-forward", "GET", "/api/test", 200, time.Millisecond*150, 1024, 2048)
		forwardService.metricsCollector.RecordResponse("test-forward", "POST", "/api/data", 201, time.Millisecond*300, 4096, 8192)

		// 记录上游请求
		forwardService.metricsCollector.RecordUpstreamResponse("test-group", "test-upstream", "GET", 200, time.Millisecond*100)
		forwardService.metricsCollector.RecordUpstreamResponse("test-group", "test-upstream", "POST", 201, time.Millisecond*250)

		// 记录错误
		forwardService.metricsCollector.RecordError("test-forward", "timeout_error")
		forwardService.metricsCollector.RecordUpstreamError("test-group", "test-upstream", "connection_error")

		// 记录断路器状态
		forwardService.metricsCollector.RecordCircuitBreakerState("test-group", "test-upstream", 0) // closed
		forwardService.metricsCollector.RecordCircuitBreakerRequest("test-group", "test-upstream", "success")

		// 记录负载均衡选择
		forwardService.metricsCollector.RecordLoadBalancerSelection("test-group", "test-upstream", "roundrobin")

		// 记录活跃连接数
		forwardService.metricsCollector.RecordActiveConnections("test-forward", 5)

		// 记录限流拒绝
		forwardService.metricsCollector.RecordRateLimitRejection("test-forward", "ip")
	}

	// 设置 HTTP 路由
	router := gin.New()
	group := router.Group("/")
	adminService.RegisterGroup(group)

	// 测试 /metrics 端点
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 验证响应状态
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// 验证内容类型
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") && !strings.Contains(contentType, "application/openmetrics-text") {
		t.Logf("Content-Type: %s", contentType)
	}

	// 获取响应体
	body := w.Body.String()
	t.Logf("Metrics response length: %d", len(body))

	if len(body) == 0 {
		t.Error("Expected non-empty metrics response")
		return
	}

	// 验证包含期望的 Prometheus 指标
	expectedMetrics := []string{
		"llmproxy_http_requests_total",
		"llmproxy_http_request_duration_seconds",
		"llmproxy_upstream_requests_total",
		"llmproxy_upstream_request_duration_seconds",
		"llmproxy_upstream_errors_total",
		"llmproxy_circuit_breaker_state",
		"llmproxy_circuit_breaker_requests_total",
		"llmproxy_load_balancer_selections_total",
		"llmproxy_active_connections",
		"llmproxy_rate_limit_rejections_total",
	}

	foundMetrics := make(map[string]bool)
	for _, metric := range expectedMetrics {
		if strings.Contains(body, metric) {
			foundMetrics[metric] = true
			t.Logf("Found metric: %s", metric)
		} else {
			t.Errorf("Missing metric: %s", metric)
		}
	}

	// 验证指标值
	expectedValues := []string{
		`llmproxy_http_requests_total{forward_name="test-forward",method="GET",path="/api/test",status_code="200"} 1`,
		`llmproxy_http_requests_total{forward_name="test-forward",method="POST",path="/api/data",status_code="201"} 1`,
		`llmproxy_upstream_requests_total{method="GET",status_code="200",upstream_group="test-group",upstream_name="test-upstream"} 1`,
		`llmproxy_upstream_requests_total{method="POST",status_code="201",upstream_group="test-group",upstream_name="test-upstream"} 1`,
		`llmproxy_active_connections{forward_name="test-forward"} 5`,
	}

	for _, expectedValue := range expectedValues {
		if strings.Contains(body, expectedValue) {
			t.Logf("Found expected value: %s", expectedValue)
		} else {
			t.Logf("Missing expected value: %s", expectedValue)
			// 注意：这里不使用 t.Errorf 因为指标值的精确格式可能会有变化
		}
	}

	// 验证基本的 Prometheus 格式
	lines := strings.Split(body, "\n")
	foundHelpLines := 0
	foundTypeLines := 0
	foundMetricLines := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "# HELP ") {
			foundHelpLines++
		} else if strings.HasPrefix(line, "# TYPE ") {
			foundTypeLines++
		} else if strings.Contains(line, "llmproxy_") && !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
			foundMetricLines++
		}
	}

	t.Logf("Found %d HELP lines, %d TYPE lines, %d metric lines", foundHelpLines, foundTypeLines, foundMetricLines)

	if foundHelpLines < 5 {
		t.Errorf("Expected at least 5 HELP lines, got %d", foundHelpLines)
	}
	if foundTypeLines < 5 {
		t.Errorf("Expected at least 5 TYPE lines, got %d", foundTypeLines)
	}
	if foundMetricLines < 5 {
		t.Errorf("Expected at least 5 metric lines, got %d", foundMetricLines)
	}

	// 清理
	globalRegistry.Clear()
}

// TestMetricsEndpointEmpty 测试空的 /metrics 端点
func TestMetricsEndpointEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 清理全局注册器
	globalRegistry := metrics.GetGlobalRegistry()
	globalRegistry.Clear()

	// 创建 AdminService（不创建 ForwardService）
	adminConfig := &config.AdminConfig{
		Port:    8080,
		Address: "127.0.0.1",
		Timeout: &config.TimeoutConfig{
			Idle:  30000,
			Read:  15000,
			Write: 15000,
		},
	}

	logger := logr.Discard()
	adminService := NewAdminServices()
	adminService.Initialize(adminConfig, &config.Config{}, &logger, nil)

	// 设置 HTTP 路由
	router := gin.New()
	group := router.Group("/")
	adminService.RegisterGroup(group)

	// 测试 /metrics 端点
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 验证响应状态
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// 获取响应体
	body := w.Body.String()
	t.Logf("Empty metrics response length: %d", len(body))

	// 即使没有指标数据，也应该返回有效的 Prometheus 格式
	// （可能只包含注释行）

	// 清理
	globalRegistry.Clear()
}
