package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

// 全局验证器实例，用于配置验证
var validate = validator.New()

// Manager 代表配置管理器，负责配置文件的加载、验证和管理
type Manager struct {
	config     *Config             // 当前加载的配置实例
	configPath string              // 配置文件的绝对路径
	validator  *validator.Validate // 配置验证器
}

// NewManager 创建新的配置管理器实例
func NewManager() *Manager {
	return &Manager{
		validator: validate,
	}
}

// LoadFromFile 从指定路径加载配置文件并进行验证
// configPath: 配置文件路径
func (m *Manager) LoadFromFile(configPath string) error {
	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", configPath)
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析 YAML 配置
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// 设置默认值
	m.SetDefaults(&config)

	// 验证配置结构
	if err := m.validator.Struct(&config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// 验证引用关系
	if err := m.validateReferences(&config); err != nil {
		return fmt.Errorf("config reference validation failed: %w", err)
	}

	// 保存配置和路径
	m.config = &config
	m.configPath, _ = filepath.Abs(configPath)

	// 配置加载成功，日志记录由调用者负责
	return nil
}

// validateReferences 验证配置中的引用关系是否正确
// config: 待验证的配置实例
func (m *Manager) validateReferences(config *Config) error {
	// 构建上游服务名称映射，用于快速查找
	upstreamNames := make(map[string]bool)
	for _, upstream := range config.Upstreams {
		upstreamNames[upstream.Name] = true
	}

	// 构建上游组名称映射，并验证组内上游服务引用
	groupNames := make(map[string]bool)
	for _, group := range config.UpstreamGroups {
		groupNames[group.Name] = true

		// 验证上游组中引用的上游服务是否存在
		for _, upstreamRef := range group.Upstreams {
			if !upstreamNames[upstreamRef.Name] {
				return fmt.Errorf("upstream group '%s' references unknown upstream '%s'",
					group.Name, upstreamRef.Name)
			}
		}
	}

	// 验证转发服务中引用的上游组是否存在
	for _, forward := range config.HTTPServer.Forwards {
		if !groupNames[forward.DefaultGroup] {
			return fmt.Errorf("forward service '%s' references unknown upstream group '%s'",
				forward.Name, forward.DefaultGroup)
		}
	}

	return nil
}

// GetConfig 返回当前加载的配置实例
func (m *Manager) GetConfig() *Config {
	return m.config
}

// GetConfigPath 返回当前配置文件的绝对路径
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// SetDefaults 为配置设置默认值，确保所有必需字段都有合理的默认值
// config: 待设置默认值的配置实例
func (m *Manager) SetDefaults(config *Config) {
	// 设置 HTTP 服务器转发服务默认值
	m.setForwardDefaults(config)

	// 设置管理服务默认值
	m.setAdminDefaults(config)

	// 设置上游服务默认值
	m.setUpstreamDefaults(config)

	// 设置上游组默认值
	m.setUpstreamGroupDefaults(config)
}

// setForwardDefaults 设置转发服务的默认值
func (m *Manager) setForwardDefaults(config *Config) {
	for i := range config.HTTPServer.Forwards {
		forward := &config.HTTPServer.Forwards[i]
		if forward.Address == "" {
			forward.Address = "0.0.0.0"
		}
		if forward.RateLimit == nil {
			forward.RateLimit = &RateLimitConfig{
				PerSecond: 100,
				Burst:     200,
			}
		}
		if forward.Timeout == nil {
			forward.Timeout = &TimeoutConfig{
				Idle:  60,
				Read:  30,
				Write: 30,
			}
		}
	}
}

// setAdminDefaults 设置管理服务的默认值
func (m *Manager) setAdminDefaults(config *Config) {
	if config.HTTPServer.Admin.Port == 0 {
		config.HTTPServer.Admin.Port = 9000
	}
	if config.HTTPServer.Admin.Address == "" {
		config.HTTPServer.Admin.Address = "0.0.0.0"
	}
	if config.HTTPServer.Admin.Timeout == nil {
		config.HTTPServer.Admin.Timeout = &TimeoutConfig{
			Idle:  60,
			Read:  30,
			Write: 30,
		}
	}
}

// setUpstreamDefaults 设置上游服务的默认值
func (m *Manager) setUpstreamDefaults(config *Config) {
	for i := range config.Upstreams {
		upstream := &config.Upstreams[i]
		if upstream.Auth == nil {
			upstream.Auth = &AuthConfig{Type: "none"}
		}
		if upstream.Breaker != nil {
			if upstream.Breaker.Threshold == 0 {
				upstream.Breaker.Threshold = 0.5
			}
			if upstream.Breaker.Cooldown == 0 {
				upstream.Breaker.Cooldown = 30
			}
		}
	}
}

// setUpstreamGroupDefaults 设置上游组的默认值
func (m *Manager) setUpstreamGroupDefaults(config *Config) {
	for i := range config.UpstreamGroups {
		group := &config.UpstreamGroups[i]
		if group.Balance == nil {
			group.Balance = &BalanceConfig{Strategy: "roundrobin"}
		}
		if group.HTTPClient == nil {
			group.HTTPClient = &HTTPClientConfig{
				Agent:     "LLMProxy/1.0",
				KeepAlive: 60,
				Timeout: &TimeoutConfig{
					Connect: 10,
					Request: 300,
					Idle:    60,
				},
			}
		}

		// 设置上游引用权重默认值
		for j := range group.Upstreams {
			if group.Upstreams[j].Weight == 0 {
				group.Upstreams[j].Weight = 1
			}
		}
	}
}
