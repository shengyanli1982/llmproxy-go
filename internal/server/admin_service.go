package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/metrics"
	"github.com/shengyanli1982/llmproxy-go/internal/response"
)

// AdminService 代表管理服务，提供基本的管理功能
// prometheus metrics 和 health check 由 orbit 框架自动提供
type AdminService struct {
	mu              sync.RWMutex
	config          *config.AdminConfig
	globalConfig    *config.Config
	logger          *logr.Logger
	server          *Server                  // 引用主服务器以获取状态信息
	metricsRegistry *metrics.MetricsRegistry // 指标注册器
	startTime       time.Time
	running         bool
}

// NewAdminServices 创建新的管理服务实例
func NewAdminServices() *AdminService {
	return &AdminService{
		startTime: time.Now(),
	}
}

// Initialize 初始化管理服务
func (s *AdminService) Initialize(config *config.AdminConfig, globalConfig *config.Config, logger *logr.Logger, server *Server) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config
	s.globalConfig = globalConfig
	s.logger = logger
	s.server = server

	// 初始化指标注册器
	s.metricsRegistry = metrics.GetGlobalRegistry()
}

// RegisterGroup 注册路由组和处理器
// 注意: prometheus metrics 通过 /metrics 端点由 orbit 框架自动提供
// 注意: health check 通过 /ping 端点由 orbit 框架自动提供
func (s *AdminService) RegisterGroup(g *gin.RouterGroup) {
	// 统一指标端点（替代 orbit 框架的默认 /metrics）
	g.GET("/metrics", s.handleMetrics)

	// 保留自定义指标端点用于调试
	g.GET("/metrics/custom", s.handleCustomMetrics)
}

// Run 启动管理服务
func (s *AdminService) Run() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	s.running = true
	if s.logger != nil {
		s.logger.Info("Admin service started")
	}
}

// Stop 停止管理服务
func (s *AdminService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	if s.logger != nil {
		s.logger.Info("Admin service stopped")
	}
}

// IsRunning 检查服务是否运行中
func (s *AdminService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// handleMetrics 处理统一指标请求（替代 orbit 默认的 /metrics）
func (s *AdminService) handleMetrics(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.metricsRegistry == nil {
		response.Error(response.CodeNotFound, "metrics registry not available").JSON(c, http.StatusNotFound)
		return
	}

	// 获取全局指标注册器
	registry := s.metricsRegistry.GetRegistry()
	if registry == nil {
		response.Error(response.CodeInternalError, "metrics registry not initialized").JSON(c, http.StatusInternalServerError)
		return
	}

	// 使用 Prometheus HTTP 处理器
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})

	// 将 Gin 上下文转换为标准 HTTP 处理器
	handler.ServeHTTP(c.Writer, c.Request)
}

// handleCustomMetrics 处理自定义指标请求
func (s *AdminService) handleCustomMetrics(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.metricsRegistry == nil {
		response.Error(response.CodeNotFound, "metrics registry not available").JSON(c, http.StatusNotFound)
		return
	}

	// 获取自定义指标注册器
	registry := s.metricsRegistry.GetRegistry()
	if registry == nil {
		response.Error(response.CodeInternalError, "metrics registry not initialized").JSON(c, http.StatusInternalServerError)
		return
	}

	// 使用 Prometheus HTTP 处理器
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})

	// 将 Gin 上下文转换为标准 HTTP 处理器
	handler.ServeHTTP(c.Writer, c.Request)
}
