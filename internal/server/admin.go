package server

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/metrics"
	"github.com/shengyanli1982/orbit"
)

// AdminServer 代表管理服务器，提供健康检查、监控指标等管理功能
type AdminServer struct {
	endpoint   string              // 服务器监听地址
	httpEngine *orbit.Engine       // HTTP 引擎实例
	closeOnce  sync.Once           // 确保只关闭一次
	config     *config.AdminConfig // 管理服务配置
	debug      bool                // 是否启用调试模式
	logger     *logr.Logger        // 日志记录器
	service    *AdminService       // 管理服务实例
}

// NewAdminServer 创建新的管理服务器实例
// debug: 是否启用调试模式
// logger: 日志记录器
// config: 管理服务配置
// globalConfig: 全局配置
// server: 主服务器引用
func NewAdminServer(debug bool, logger *logr.Logger, config *config.AdminConfig, globalConfig *config.Config, server *Server) *AdminServer {
	endpoint := fmt.Sprintf("%s:%d", config.Address, config.Port)

	// 创建 Orbit 引擎配置
	cfg := orbit.NewConfig().
		WithLogger(logger).
		WithAddress(config.Address).
		WithPort(uint16(config.Port)).
		WithHttpIdleTimeout(uint32(config.Timeout.Idle)). // 配置提供的单位是毫秒，直接使用
		WithHttpReadHeaderTimeout(uint32(config.Timeout.Read)).
		WithHttpReadTimeout(uint32(config.Timeout.Read)).
		WithHttpWriteTimeout(uint32(config.Timeout.Write))

	// 创建引擎选项
	opts := orbit.NewOptions().EnablePProf().EnableSwagger()
	if !debug {
		opts = orbit.NewOptions()
		cfg.WithRelease()
	}

	// 获取全局指标注册器并尝试集成到 orbit 中
	metricsRegistry := metrics.GetGlobalRegistry()
	if metricsRegistry != nil {
		// 尝试设置自定义的 Prometheus Registry
		// 注意：这可能需要 orbit 框架的特定支持
		// 如果 orbit 不支持自定义 registry，我们将通过自定义端点提供指标
		registry := metricsRegistry.GetRegistry()
		if registry != nil {
			// 这里我们尝试将自定义 registry 集成到 orbit 中
			// 由于 orbit 框架的限制，我们可能需要使用其他方法
			logger.Info("Custom metrics registry initialized", "collectors", metricsRegistry.CollectorCount())

			// 注意：orbit 框架可能不支持自定义 prometheus registry
			// 我们将通过 AdminService 的 /metrics 端点来暴露统一指标
		}
	}

	// 创建 HTTP 引擎
	engine := orbit.NewEngine(cfg, opts)

	// 创建管理服务实例
	svcs := NewAdminServices()

	// 初始化管理服务
	svcs.Initialize(config, globalConfig, logger, server)

	// 注册服务到引擎
	engine.RegisterService(svcs)

	return &AdminServer{
		endpoint:   endpoint,
		httpEngine: engine,
		config:     config,
		debug:      debug,
		logger:     logger,
		service:    svcs,
	}
}

// Start 启动管理服务器
func (s *AdminServer) Start() {
	if s.httpEngine.IsRunning() {
		s.logger.Error(ErrServerAlreadyStarted, "Admin server is already started")
		return
	}

	s.logger.Info("Starting admin server", "endpoint", s.endpoint)

	// 启动管理服务
	s.service.Run()

	// 启动 HTTP 引擎
	s.httpEngine.Run()

	// 重置关闭标志
	s.closeOnce = sync.Once{}

	s.logger.Info("Admin server started successfully", "endpoint", s.endpoint)
}

// Stop 停止管理服务器
func (s *AdminServer) Stop() {
	if !s.httpEngine.IsRunning() {
		s.logger.Info("Admin server is not running")
		return
	}

	s.logger.Info("Stopping admin server", "endpoint", s.endpoint)

	s.closeOnce.Do(func() {
		// 停止 HTTP 引擎
		s.httpEngine.Stop()

		// 停止管理服务
		s.service.Stop()

		s.logger.Info("Admin server stopped successfully", "endpoint", s.endpoint)
	})
}

// IsRunning 检查管理服务器是否正在运行
func (s *AdminServer) IsRunning() bool {
	return s.httpEngine.IsRunning()
}

// GetEndpoint 获取服务器实际监听地址（运行时分配的地址）
func (s *AdminServer) GetEndpoint() string {
	if s.httpEngine != nil {
		return s.httpEngine.GetListenEndpoint()
	}
	return s.endpoint
}

// GetConfig 获取管理服务配置
func (s *AdminServer) GetConfig() *config.AdminConfig {
	return s.config
}
