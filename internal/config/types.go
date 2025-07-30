package config

// Config 代表主配置结构体，包含HTTP服务器、上游服务和上游组的完整配置
type Config struct {
	HTTPServer     HTTPServerConfig      `yaml:"httpServer" validate:"required"`
	Upstreams      []UpstreamConfig      `yaml:"upstreams" validate:"required,dive"`
	UpstreamGroups []UpstreamGroupConfig `yaml:"upstreamGroups" validate:"required,dive"`
}

// HTTPServerConfig 代表HTTP服务器配置，包含转发服务和管理服务设置
type HTTPServerConfig struct {
	Forwards []ForwardConfig `yaml:"forwards" validate:"required,dive"`
	Admin    AdminConfig     `yaml:"admin"`
}

// ForwardConfig 代表转发服务配置，定义单个代理转发实例的参数
type ForwardConfig struct {
	Name         string           `yaml:"name" validate:"required"`
	Port         int              `yaml:"port" validate:"required,min=1,max=65535"`
	Address      string           `yaml:"address"`
	DefaultGroup string           `yaml:"defaultGroup" validate:"required"`
	RateLimit    *RateLimitConfig `yaml:"ratelimit,omitempty"`
	Timeout      *TimeoutConfig   `yaml:"timeout,omitempty"`
}

// AdminConfig 代表管理服务配置，用于健康检查和监控指标暴露
type AdminConfig struct {
	Port    int            `yaml:"port" validate:"min=1,max=65535"`
	Address string         `yaml:"address"`
	Timeout *TimeoutConfig `yaml:"timeout,omitempty"`
}

// RateLimitConfig 代表限流配置，控制请求频率和突发流量
type RateLimitConfig struct {
	PerSecond int `yaml:"perSecond" validate:"omitempty,min=1"`
	Burst     int `yaml:"burst" validate:"omitempty,min=1"`
}

// TimeoutConfig 代表超时配置，定义各种操作的超时时间
type TimeoutConfig struct {
	Idle    int `yaml:"idle" validate:"min=1"`
	Read    int `yaml:"read" validate:"min=1"`
	Write   int `yaml:"write" validate:"min=1"`
	Connect int `yaml:"connect,omitempty" validate:"min=1"`
	Request int `yaml:"request,omitempty" validate:"min=1"`
}

// UpstreamConfig 代表上游服务配置，定义后端LLM API服务的连接参数
type UpstreamConfig struct {
	Name      string           `yaml:"name" validate:"required"`
	URL       string           `yaml:"url" validate:"required,url"`
	Auth      *AuthConfig      `yaml:"auth,omitempty"`
	Headers   []HeaderOpConfig `yaml:"headers,omitempty"`
	Breaker   *BreakerConfig   `yaml:"breaker,omitempty"`
	RateLimit *RateLimitConfig `yaml:"ratelimit,omitempty"`
}

// AuthConfig 代表认证配置，支持Bearer Token和Basic Auth
type AuthConfig struct {
	Type     string `yaml:"type" validate:"oneof=none bearer basic"`
	Token    string `yaml:"token,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// HeaderOpConfig 代表HTTP头部操作配置，用于修改转发请求的头部信息
type HeaderOpConfig struct {
	Op    string `yaml:"op" validate:"required,oneof=insert replace remove"`
	Key   string `yaml:"key" validate:"required"`
	Value string `yaml:"value,omitempty"`
}

// BreakerConfig 代表熔断器配置，用于保护上游服务避免过载
type BreakerConfig struct {
	Threshold float64 `yaml:"threshold" validate:"min=0.01,max=1.0"`
	Cooldown  int     `yaml:"cooldown" validate:"min=1,max=3600"`
}

// UpstreamGroupConfig 代表上游组配置，将多个上游服务组织为一个逻辑单元
type UpstreamGroupConfig struct {
	Name       string              `yaml:"name" validate:"required"`
	Upstreams  []UpstreamRefConfig `yaml:"upstreams" validate:"required,dive"`
	Balance    *BalanceConfig      `yaml:"balance,omitempty"`
	HTTPClient *HTTPClientConfig   `yaml:"httpClient,omitempty"`
}

// UpstreamRefConfig 代表上游引用配置，在上游组中引用具体的上游服务
type UpstreamRefConfig struct {
	Name   string `yaml:"name" validate:"required"`
	Weight int    `yaml:"weight,omitempty" validate:"min=1"`
}

// BalanceConfig 代表负载均衡配置，定义选择上游服务的策略
type BalanceConfig struct {
	Strategy string `yaml:"strategy" validate:"oneof=roundrobin weighted_roundrobin random response_aware failover"`
}

// HTTPClientConfig 代表HTTP客户端配置，控制与上游服务的连接行为
type HTTPClientConfig struct {
	Agent     string         `yaml:"agent"`
	KeepAlive int            `yaml:"keepalive" validate:"min=0,max=600"`
	Timeout   *TimeoutConfig `yaml:"timeout,omitempty"`
	Retry     *RetryConfig   `yaml:"retry,omitempty"`
	Proxy     *ProxyConfig   `yaml:"proxy,omitempty"`
}

// RetryConfig 代表重试配置，定义失败请求的重试策略
type RetryConfig struct {
	Attempts int `yaml:"attempts" validate:"min=1"`
	Initial  int `yaml:"initial" validate:"min=1"`
}

// ProxyConfig 代表代理配置，设置HTTP代理服务器
type ProxyConfig struct {
	URL string `yaml:"url" validate:"required,url"`
}
