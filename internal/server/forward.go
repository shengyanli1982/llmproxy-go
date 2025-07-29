package server

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/orbit"
)

// ForwardServer 代表转发服务器，负责处理客户端请求并转发到上游服务
type ForwardServer struct {
	name         string                // 服务器名称
	endpoint     string                // 服务器监听地址
	httpEngine   *orbit.Engine         // HTTP 引擎实例
	closeOnce    sync.Once             // 确保只关闭一次
	config       *config.ForwardConfig // 转发服务配置
	globalConfig *config.Config        // 全局配置
	debug        bool                  // 是否启用调试模式
	logger       *logr.Logger          // 日志记录器
	service      *ForwardService       // 转发服务实例
}

// NewForwardServer 创建新的转发服务器实例
// debug: 是否启用调试模式
// logger: 日志记录器
// config: 转发服务配置
// globalConfig: 全局配置
func NewForwardServer(debug bool, logger *logr.Logger, config *config.ForwardConfig, globalConfig *config.Config) *ForwardServer {
	endpoint := fmt.Sprintf("%s:%d", config.Address, config.Port)

	// 创建 Orbit 引擎配置
	cfg := orbit.NewConfig().
		WithLogger(logger).
		WithAddress(config.Address).
		WithPort(uint16(config.Port)).
		WithHttpIdleTimeout(uint32(config.Timeout.Idle)).
		WithHttpReadHeaderTimeout(uint32(config.Timeout.Read)).
		WithHttpReadTimeout(uint32(config.Timeout.Read)).
		WithHttpWriteTimeout(uint32(config.Timeout.Write))

	// 创建引擎选项
	opts := orbit.EmptyOptions()

	// 创建 HTTP 引擎
	engine := orbit.NewEngine(cfg, opts)

	// 创建转发服务实例
	svcs := NewForwardServices()

	// 初始化转发服务
	if err := svcs.Initialize(config, globalConfig, logger); err != nil {
		logger.Error(err, "Failed to initialize forward service")
		// 在实际项目中可能需要更好的错误处理
	}

	// 注册服务到引擎
	engine.RegisterService(svcs)

	return &ForwardServer{
		name:         config.Name,
		endpoint:     endpoint,
		httpEngine:   engine,
		config:       config,
		globalConfig: globalConfig,
		debug:        debug,
		logger:       logger,
		service:      svcs,
	}
}

// Start 启动转发服务器
func (s *ForwardServer) Start() {
	if s.httpEngine.IsRunning() {
		s.logger.Error(ErrServerAlreadyStarted, "Forward server is already started", "name", s.name)
		return
	}

	s.logger.Info("Starting forward server", "name", s.name, "endpoint", s.endpoint)

	// 启动转发服务
	s.service.Run()

	// 启动 HTTP 引擎
	s.httpEngine.Run()

	// 重置关闭标志
	s.closeOnce = sync.Once{}

	s.logger.Info("Forward server started successfully", "name", s.name)
}

// Stop 停止转发服务器
func (s *ForwardServer) Stop() {
	if !s.httpEngine.IsRunning() {
		s.logger.Info("Forward server is not running", "name", s.name)
		return
	}

	s.logger.Info("Stopping forward server", "name", s.name)

	s.closeOnce.Do(func() {
		// 停止 HTTP 引擎
		s.httpEngine.Stop()

		// 停止转发服务
		s.service.Stop()

		s.logger.Info("Forward server stopped successfully", "name", s.name)
	})
}

// IsRunning 检查转发服务器是否正在运行
func (s *ForwardServer) IsRunning() bool {
	return s.httpEngine.IsRunning()
}

// GetEndpoint 获取服务器监听地址
func (s *ForwardServer) GetEndpoint() string {
	return s.endpoint
}

// GetConfig 获取转发服务配置
func (s *ForwardServer) GetConfig() *config.ForwardConfig {
	return s.config
}

// GetService 获取转发服务实例
func (s *ForwardServer) GetService() *ForwardService {
	return s.service
}
