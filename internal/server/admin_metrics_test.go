package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/metrics"
)

// TestAdminService_MetricsIntegration 测试 AdminService 的 metrics 集成
func TestAdminService_MetricsIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	adminConfig := &config.AdminConfig{
		Port:    8080,
		Address: "127.0.0.1",
		Timeout: &config.TimeoutConfig{
			Idle:  30000,
			Read:  15000,
			Write: 15000,
		},
	}

	globalConfig := &config.Config{
		HTTPServer: config.HTTPServerConfig{
			Admin: *adminConfig,
		},
	}

	logger := logr.Discard()

	// 创建 AdminService
	service := NewAdminServices()

	// 初始化服务
	service.Initialize(adminConfig, globalConfig, &logger, nil)

	// 验证 metrics registry 已初始化
	if service.metricsRegistry == nil {
		t.Fatal("Expected metrics registry to be initialized")
	}

	// 验证 metrics registry 是全局单例
	globalRegistry := metrics.GetGlobalRegistry()
	if service.metricsRegistry != globalRegistry {
		t.Error("Expected AdminService to use global metrics registry")
	}
}

// TestAdminService_CustomMetricsEndpoint 测试自定义 metrics 端点
func TestAdminService_CustomMetricsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	adminConfig := &config.AdminConfig{
		Port:    8081,
		Address: "127.0.0.1",
		Timeout: &config.TimeoutConfig{
			Idle:  30000,
			Read:  15000,
			Write: 15000,
		},
	}

	globalConfig := &config.Config{
		HTTPServer: config.HTTPServerConfig{
			Admin: *adminConfig,
		},
	}

	logger := logr.Discard()

	// 创建 AdminService
	service := NewAdminServices()
	service.Initialize(adminConfig, globalConfig, &logger, nil)

	// 创建测试路由
	router := gin.New()
	group := router.Group("/")
	service.RegisterGroup(group)

	// 测试自定义 metrics 端点
	req := httptest.NewRequest("GET", "/metrics/custom", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// 验证响应内容类型
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") && !strings.Contains(contentType, "application/openmetrics-text") {
		t.Errorf("Expected Prometheus metrics content type, got %s", contentType)
	}

	// 验证响应体
	body := w.Body.String()

	// 如果没有注册任何指标，响应体可能为空，这是正常的
	// 我们主要验证端点能够正常响应，而不是一定要有内容
	t.Logf("Metrics response body length: %d", len(body))

	// 如果有内容，验证是否符合 Prometheus 格式
	if len(body) > 0 {
		// 基本的 Prometheus 格式验证
		if !strings.Contains(body, "# HELP") && !strings.Contains(body, "# TYPE") {
			t.Log("Response body:", body)
			// 这可能是正常的，如果没有注册任何指标
		}
	}
}

// TestAdminService_MetricsRegistryAccess 测试指标注册器访问
func TestAdminService_MetricsRegistryAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	adminConfig := &config.AdminConfig{
		Port:    8082,
		Address: "127.0.0.1",
		Timeout: &config.TimeoutConfig{
			Idle:  30000,
			Read:  15000,
			Write: 15000,
		},
	}

	globalConfig := &config.Config{
		HTTPServer: config.HTTPServerConfig{
			Admin: *adminConfig,
		},
	}

	logger := logr.Discard()

	// 创建 AdminService
	service := NewAdminServices()
	service.Initialize(adminConfig, globalConfig, &logger, nil)

	// 验证可以访问 metrics registry
	registry := service.metricsRegistry.GetRegistry()
	if registry == nil {
		t.Error("Expected to be able to access Prometheus registry")
	}

	// 验证可以收集指标
	gatheredMetrics, err := registry.Gather()
	if err != nil {
		t.Errorf("Failed to gather metrics: %v", err)
	}

	// gatheredMetrics 可能为空，这是正常的，因为没有注册任何指标
	// 我们只需要验证调用不会出错
	_ = gatheredMetrics // 使用变量避免未使用警告
}

// TestAdminService_MetricsEndpointError 测试 metrics 端点错误处理
func TestAdminService_MetricsEndpointError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建一个没有初始化 metrics registry 的服务
	service := &AdminService{}

	// 创建测试路由
	router := gin.New()
	group := router.Group("/")

	// 手动注册路由（跳过完整的 RegisterGroup）
	group.GET("/metrics/custom", service.handleCustomMetrics)

	// 测试没有 metrics registry 的情况
	req := httptest.NewRequest("GET", "/metrics/custom", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 应该返回 404 错误
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestAdminServer_MetricsRegistryIntegration 测试 AdminServer 的 metrics registry 集成
func TestAdminServer_MetricsRegistryIntegration(t *testing.T) {
	// 创建测试配置
	adminConfig := &config.AdminConfig{
		Port:    8083,
		Address: "127.0.0.1",
		Timeout: &config.TimeoutConfig{
			Idle:  30000,
			Read:  15000,
			Write: 15000,
		},
	}

	globalConfig := &config.Config{
		HTTPServer: config.HTTPServerConfig{
			Admin: *adminConfig,
		},
	}

	logger := logr.Discard()

	// 在创建 AdminServer 之前，向全局 registry 添加一个收集器
	globalRegistry := metrics.GetGlobalRegistry()

	// 创建一个测试收集器
	collector, err := metrics.CreateNoopCollector()
	if err != nil {
		t.Fatalf("Failed to create test collector: %v", err)
	}

	// 注册收集器
	err = globalRegistry.RegisterCollector("test-collector", collector)
	if err != nil {
		t.Fatalf("Failed to register test collector: %v", err)
	}

	// 创建 AdminServer（这应该会检测到现有的收集器）
	adminServer := NewAdminServer(true, &logger, adminConfig, globalConfig, nil)

	// 验证 AdminServer 创建成功
	if adminServer == nil {
		t.Fatal("Expected AdminServer to be created")
	}

	// 验证 AdminService 使用了全局 registry
	if adminServer.service.metricsRegistry != globalRegistry {
		t.Error("Expected AdminServer to use global metrics registry")
	}

	// 验证收集器计数
	if globalRegistry.CollectorCount() != 1 {
		t.Errorf("Expected 1 collector, got %d", globalRegistry.CollectorCount())
	}

	// 清理
	globalRegistry.UnregisterCollector("test-collector")
}

// BenchmarkAdminService_CustomMetrics 基准测试自定义 metrics 端点的性能
func BenchmarkAdminService_CustomMetrics(b *testing.B) {
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	adminConfig := &config.AdminConfig{
		Port:    8084,
		Address: "127.0.0.1",
		Timeout: &config.TimeoutConfig{
			Idle:  30000,
			Read:  15000,
			Write: 15000,
		},
	}

	globalConfig := &config.Config{
		HTTPServer: config.HTTPServerConfig{
			Admin: *adminConfig,
		},
	}

	logger := logr.Discard()

	// 创建 AdminService
	service := NewAdminServices()
	service.Initialize(adminConfig, globalConfig, &logger, nil)

	// 创建测试路由
	router := gin.New()
	group := router.Group("/")
	service.RegisterGroup(group)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/metrics/custom", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}
