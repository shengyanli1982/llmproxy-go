package server

import (
	"sync"

	"github.com/go-logr/logr"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
)

// Server 代表主服务器，管理转发服务器和管理服务器
type Server struct {
	lock           sync.RWMutex              // 读写锁，保护并发访问
	forwardServers map[string]*ForwardServer // 转发服务器映射
	adminServer    *AdminServer              // 管理服务器实例
	logger         *logr.Logger              // 日志记录器
}

// NewServer 创建新的服务器实例
// debug: 是否启用调试模式
// logger: 日志记录器
// config: HTTP 服务器配置
func NewServer(debug bool, logger *logr.Logger, config *config.HTTPServerConfig, globalConfig *config.Config) *Server {
	srv := &Server{
		forwardServers: make(map[string]*ForwardServer),
		logger:         logger,
	}

	// 创建转发服务器实例
	for _, forward := range config.Forwards {
		forwardServer := NewForwardServer(debug, logger, &forward, globalConfig)
		srv.forwardServers[forward.Name] = forwardServer
	}

	// 创建管理服务器实例
	srv.adminServer = NewAdminServer(debug, logger, &config.Admin, globalConfig, srv)

	return srv
}

// Start 启动所有服务器（转发服务器和管理服务器）
func (s *Server) Start() {
	s.logger.Info("Starting all servers")

	// 启动所有转发服务器
	for name, forwardServer := range s.forwardServers {
		s.logger.Info("Starting forward server", "name", name)
		forwardServer.Start()
	}

	// 启动管理服务器
	s.logger.Info("Starting admin server")
	s.adminServer.Start()
}

// Stop 停止所有服务器（转发服务器和管理服务器）
func (s *Server) Stop() {
	s.logger.Info("Stopping all servers")

	// 停止所有转发服务器
	for name, forwardServer := range s.forwardServers {
		s.logger.Info("Stopping forward server", "name", name)
		forwardServer.Stop()
	}

	// 停止管理服务器
	s.logger.Info("Stopping admin server")
	s.adminServer.Stop()
}

// AddForwardServer 添加新的转发服务器
// forwardServer: 要添加的转发服务器实例
func (s *Server) AddForwardServer(forwardServer *ForwardServer) {
	if forwardServer == nil {
		s.logger.Error(nil, "Cannot add nil forward server")
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	name := forwardServer.GetConfig().Name
	if _, exists := s.forwardServers[name]; !exists {
		s.forwardServers[name] = forwardServer
		s.logger.Info("Forward server added", "name", name)
	} else {
		s.logger.Info("Forward server already exists", "name", name)
	}
}

// RemoveForwardServer 移除指定的转发服务器
// forwardServer: 要移除的转发服务器实例
func (s *Server) RemoveForwardServer(forwardServer *ForwardServer) {
	if forwardServer == nil {
		s.logger.Error(nil, "Cannot remove nil forward server")
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	name := forwardServer.GetConfig().Name
	if _, exists := s.forwardServers[name]; exists {
		forwardServer.Stop()
		delete(s.forwardServers, name)
		s.logger.Info("Forward server removed", "name", name)
	} else {
		s.logger.Info("Forward server not found", "name", name)
	}
}

// GetForwardServer 根据名称获取转发服务器实例
// name: 转发服务器名称
func (s *Server) GetForwardServer(name string) *ForwardServer {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if forwardServer, exists := s.forwardServers[name]; exists {
		return forwardServer
	}
	return nil
}
