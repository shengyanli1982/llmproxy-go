package server

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/response"
)

// AdminService 代表管理服务，提供基本的管理功能
// prometheus metrics 和 health check 由 orbit 框架自动提供
type AdminService struct {
	mu           sync.RWMutex
	config       *config.AdminConfig
	globalConfig *config.Config
	logger       *logr.Logger
	server       *Server // 引用主服务器以获取状态信息
	startTime    time.Time
	running      bool
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
}

// RegisterGroup 注册路由组和处理器
// 注意: prometheus metrics 通过 /metrics 端点由 orbit 框架自动提供
// 注意: health check 通过 /health 端点由 orbit 框架自动提供
func (s *AdminService) RegisterGroup(g *gin.RouterGroup) {
	// 详细状态端点
	g.GET("/status", s.handleStatus)

	// 配置查看端点
	g.GET("/config", s.handleConfig)

	// 运行时信息端点
	g.GET("/info", s.handleInfo)
}

// handleStatus 处理详细状态请求
func (s *AdminService) handleStatus(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statusInfo := map[string]interface{}{
		"service": map[string]interface{}{
			"name":      "LLMProxy",
			"version":   "1.0.0",
			"uptime":    time.Since(s.startTime).Seconds(),
			"startTime": s.startTime.Format(time.RFC3339),
		},
		"runtime": map[string]interface{}{
			"goVersion":   runtime.Version(),
			"goroutines":  runtime.NumGoroutine(),
			"memoryAlloc": getMemoryStats(),
		},
	}

	if s.server != nil {
		// 添加转发服务器信息
		forwardServers := make(map[string]interface{})
		for name, forwardServer := range s.server.forwardServers {
			forwardServers[name] = map[string]interface{}{
				"running":  forwardServer.IsRunning(),
				"endpoint": forwardServer.GetEndpoint(),
				"config":   forwardServer.GetConfig(),
			}
		}
		statusInfo["forwardServers"] = forwardServers

		// 添加管理服务器信息
		statusInfo["adminServer"] = map[string]interface{}{
			"running":  s.server.adminServer.IsRunning(),
			"endpoint": s.server.adminServer.GetEndpoint(),
		}
	}

	response.Success(statusInfo).JSON(c, http.StatusOK)
}

// handleConfig 处理配置查看请求
func (s *AdminService) handleConfig(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.globalConfig == nil {
		response.Error(response.CodeNotFound, "configuration not available").JSON(c, http.StatusNotFound)
		return
	}

	// 为了安全考虑，移除敏感信息
	configCopy := s.sanitizeConfig(s.globalConfig)

	configData := map[string]interface{}{
		"config":    configCopy,
		"timestamp": time.Now().Unix(),
	}

	response.Success(configData).JSON(c, http.StatusOK)
}

// handleInfo 处理运行时信息请求
func (s *AdminService) handleInfo(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info := map[string]interface{}{
		"application": map[string]interface{}{
			"name":        "LLMProxy",
			"version":     "1.0.0",
			"description": "High-performance LLM HTTP request proxy",
		},
		"build": map[string]interface{}{
			"goVersion": runtime.Version(),
			"os":        runtime.GOOS,
			"arch":      runtime.GOARCH,
			"timestamp": s.startTime.Format(time.RFC3339),
		},
		"runtime": map[string]interface{}{
			"uptime":     time.Since(s.startTime).Seconds(),
			"goroutines": runtime.NumGoroutine(),
			"memory":     getMemoryStats(),
		},
	}

	response.Success(info).JSON(c, http.StatusOK)
}

// sanitizeConfig 清理配置中的敏感信息
func (s *AdminService) sanitizeConfig(cfg *config.Config) *config.Config {
	// 创建配置副本
	data, err := json.Marshal(cfg)
	if err != nil {
		return cfg
	}

	var configCopy config.Config
	if err := json.Unmarshal(data, &configCopy); err != nil {
		return cfg
	}

	// 清理敏感信息
	for i := range configCopy.Upstreams {
		if configCopy.Upstreams[i].Auth != nil {
			configCopy.Upstreams[i].Auth.Token = "***"
			configCopy.Upstreams[i].Auth.Password = "***"
		}
	}

	return &configCopy
}

// getMemoryStats 获取内存统计信息
func getMemoryStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"alloc":       m.Alloc,
		"totalAlloc":  m.TotalAlloc,
		"sys":         m.Sys,
		"heapAlloc":   m.HeapAlloc,
		"heapSys":     m.HeapSys,
		"heapObjects": m.HeapObjects,
		"gcCycles":    m.NumGC,
		"nextGc":      m.NextGC,
	}
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
