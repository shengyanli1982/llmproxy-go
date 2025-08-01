package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
func NewManager() (*Manager, error) {
	var err error
	// 注册自定义验证器
	err = validate.RegisterValidation("auth_conditional", validateAuthConditional)
	if err != nil {
		return nil, err
	}
	err = validate.RegisterValidation("header_conditional", validateHeaderConditional)
	if err != nil {
		return nil, err
	}
	err = validate.RegisterValidation("http_url", validateHTTPURL)
	if err != nil {
		return nil, err
	}

	return &Manager{
		validator: validate,
	}, nil
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
				Idle:    60000,
				Read:    30000,
				Write:   30000,
				Connect: 10000,
				Request: 300000,
			}
		} else {
			// 如果Timeout存在但某些字段为0，设置默认值
			if forward.Timeout.Idle == 0 {
				forward.Timeout.Idle = 60000
			}
			if forward.Timeout.Read == 0 {
				forward.Timeout.Read = 30000
			}
			if forward.Timeout.Write == 0 {
				forward.Timeout.Write = 30000
			}
			if forward.Timeout.Connect == 0 {
				forward.Timeout.Connect = 10000
			}
			if forward.Timeout.Request == 0 {
				forward.Timeout.Request = 300000
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
			Idle:    60000,
			Read:    30000,
			Write:   30000,
			Connect: 10000,
			Request: 300000,
		}
	} else {
		// 如果Timeout存在但某些字段为0，设置默认值
		if config.HTTPServer.Admin.Timeout.Idle == 0 {
			config.HTTPServer.Admin.Timeout.Idle = 60000
		}
		if config.HTTPServer.Admin.Timeout.Read == 0 {
			config.HTTPServer.Admin.Timeout.Read = 30000
		}
		if config.HTTPServer.Admin.Timeout.Write == 0 {
			config.HTTPServer.Admin.Timeout.Write = 30000
		}
		if config.HTTPServer.Admin.Timeout.Connect == 0 {
			config.HTTPServer.Admin.Timeout.Connect = 10000
		}
		if config.HTTPServer.Admin.Timeout.Request == 0 {
			config.HTTPServer.Admin.Timeout.Request = 300000
		}
	}
}

// setUpstreamDefaults 设置上游服务的默认值
func (m *Manager) setUpstreamDefaults(config *Config) {
	for i := range config.Upstreams {
		upstream := &config.Upstreams[i]
		if upstream.Auth == nil {
			upstream.Auth = &AuthConfig{Type: "none"}
		} else if upstream.Auth.Type == "" {
			upstream.Auth.Type = "none"
		}
		if upstream.Breaker != nil {
			if upstream.Breaker.Threshold == 0 {
				upstream.Breaker.Threshold = 0.5
			}
			if upstream.Breaker.Cooldown == 0 {
				upstream.Breaker.Cooldown = 30000
			}
			if upstream.Breaker.MaxRequests == 0 {
				upstream.Breaker.MaxRequests = 3
			}
			if upstream.Breaker.Interval == 0 {
				upstream.Breaker.Interval = 10000
			}
		}
		if upstream.RateLimit != nil {
			if upstream.RateLimit.Burst == 0 {
				upstream.RateLimit.Burst = 1
			}
			if upstream.RateLimit.PerSecond == 0 {
				upstream.RateLimit.PerSecond = 100
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
				KeepAlive: 60000,
				Connect: &ConnectConfig{
					IdleTotal:   100,
					IdlePerHost: 10,
					MaxPerHost:  50,
				},
				Timeout: &TimeoutConfig{
					Connect: 10000,
					Request: 300000,
					Idle:    60000,
					Read:    30000,
					Write:   30000,
				},
			}
		} else {
			// 如果HTTPClient存在但KeepAlive为0，设置默认值
			if group.HTTPClient.KeepAlive == 0 {
				group.HTTPClient.KeepAlive = 60000
			}

			// 如果HTTPClient存在但Connect为nil，设置默认的Connect配置
			if group.HTTPClient.Connect == nil {
				group.HTTPClient.Connect = &ConnectConfig{
					IdleTotal:   100,
					IdlePerHost: 10,
					MaxPerHost:  50,
				}
			} else {
				// 如果Connect存在但某些字段为0，设置默认值
				if group.HTTPClient.Connect.IdleTotal == 0 {
					group.HTTPClient.Connect.IdleTotal = 100
				}
				if group.HTTPClient.Connect.IdlePerHost == 0 {
					group.HTTPClient.Connect.IdlePerHost = 10
				}
				if group.HTTPClient.Connect.MaxPerHost == 0 {
					group.HTTPClient.Connect.MaxPerHost = 50
				}
			}

			// 如果HTTPClient存在但Timeout为nil，设置默认的Timeout配置
			if group.HTTPClient.Timeout == nil {
				group.HTTPClient.Timeout = &TimeoutConfig{
					Connect: 10000,
					Request: 300000,
					Idle:    60000,
					Read:    30000,
					Write:   30000,
				}
			} else {
				// 如果Timeout存在但某些字段为0，设置默认值
				if group.HTTPClient.Timeout.Connect == 0 {
					group.HTTPClient.Timeout.Connect = 10000
				}
				if group.HTTPClient.Timeout.Request == 0 {
					group.HTTPClient.Timeout.Request = 300000
				}
				if group.HTTPClient.Timeout.Idle == 0 {
					group.HTTPClient.Timeout.Idle = 60000
				}
				if group.HTTPClient.Timeout.Read == 0 {
					group.HTTPClient.Timeout.Read = 30000
				}
				if group.HTTPClient.Timeout.Write == 0 {
					group.HTTPClient.Timeout.Write = 30000
				}
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

// validateAuthConditional 验证认证配置的条件必填字段
func validateAuthConditional(fl validator.FieldLevel) bool {
	auth, ok := fl.Parent().Interface().(AuthConfig)
	if !ok {
		return true // 如果不是AuthConfig类型，跳过验证
	}

	switch auth.Type {
	case "bearer":
		// 当type为bearer时，token必填
		return auth.Token != ""
	case "basic":
		// 当type为basic时，username和password必填
		return auth.Username != "" && auth.Password != ""
	case "none", "":
		// 当type为none或空时，不需要其他字段
		return true
	default:
		return false // 未知的认证类型
	}
}

// validateHeaderConditional 验证头部操作配置的条件必填字段
func validateHeaderConditional(fl validator.FieldLevel) bool {
	header, ok := fl.Parent().Interface().(HeaderOpConfig)
	if !ok {
		return true // 如果不是HeaderOpConfig类型，跳过验证
	}

	switch header.Op {
	case "insert", "replace":
		// 当op为insert或replace时，value必填
		return header.Value != ""
	case "remove":
		// 当op为remove时，value可选
		return true
	default:
		return false // 未知的操作类型
	}
}

// validateHTTPURL 验证URL必须使用HTTP或HTTPS协议
func validateHTTPURL(fl validator.FieldLevel) bool {
	urlStr := fl.Field().String()
	if urlStr == "" {
		return false // 空URL无效
	}

	// 解析URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false // URL格式无效
	}

	// 检查协议必须是http或https（大小写不敏感）
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return false // 协议必须是http或https
	}

	// 检查必须包含有效的host
	if parsedURL.Host == "" {
		return false // 必须包含host
	}

	return true
}
