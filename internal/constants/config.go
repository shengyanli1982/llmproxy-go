package constants

const (
	// Command line flags - 命令行标志

	// FlagConfig 配置文件路径参数名
	FlagConfig = "config"

	// FlagJSON JSON日志格式参数名
	FlagJSON = "json"

	// FlagRelease 发布模式参数名
	FlagRelease = "release"

	// Flag short aliases - 短参数别名

	// FlagConfigShort 配置文件路径短参数
	FlagConfigShort = "c"

	// FlagJSONShort JSON日志格式短参数
	FlagJSONShort = "j"

	// FlagReleaseShort 发布模式短参数
	FlagReleaseShort = "r"
)

const (
	// Limits and constraints - 限制和约束

	// MinTimeout 最小超时时间（毫秒）
	MinTimeout = 1000

	// MaxTimeout 最大超时时间（毫秒，24小时）
	MaxTimeout = 86400000

	// MinPort 最小端口号
	MinPort = 1

	// MaxPort 最大端口号
	MaxPort = 65535

	// MinWeight 最小权重值
	MinWeight = 1

	// MaxWeight 最大权重值
	MaxWeight = 65535

	// MinPerSecond 最小每秒请求数
	MinPerSecond = 1

	// MaxPerSecond 最大每秒请求数
	MaxPerSecond = 65535

	// MinBurst 最小突发请求数
	MinBurst = 1

	// MaxBurst 最大突发请求数
	MaxBurst = 65535

	// MaxRequests 熔断器半开状态最大请求数
	MaxRequests = 100
)

const (
	// Default configuration values - 配置默认值

	// DefaultAddress 默认绑定地址
	DefaultAddress = "0.0.0.0"

	// DefaultAdminPort 默认管理端口
	DefaultAdminPort = 9000

	// DefaultRequestTimeout 默认HTTP请求超时时间（毫秒）
	DefaultRequestTimeout = 60000

	// DefaultIdleTimeout 默认空闲超时（毫秒）
	DefaultIdleTimeout = 60000

	// DefaultReadTimeout 默认读取超时（毫秒）
	DefaultReadTimeout = 30000

	// DefaultWriteTimeout 默认写入超时（毫秒）
	DefaultWriteTimeout = 30000

	// DefaultConnectTimeout 默认连接超时（毫秒）
	DefaultConnectTimeout = 10000

	// DefaultForwardRequestTimeout 默认转发请求超时（毫秒）
	DefaultForwardRequestTimeout = 300000

	// DefaultKeepAlive 默认Keep-Alive时间（毫秒）
	DefaultKeepAlive = 60000

	// DefaultRatePerSecond 默认每秒请求数
	DefaultRatePerSecond = 100

	// DefaultRateBurst 默认突发请求数
	DefaultRateBurst = 1

	// DefaultBreakerThreshold 默认熔断器阈值
	DefaultBreakerThreshold = 0.5

	// DefaultBreakerCooldown 默认熔断器冷却时间（毫秒）
	DefaultBreakerCooldown = 30000

	// DefaultBreakerMaxRequests 默认熔断器最大请求数
	DefaultBreakerMaxRequests = 3

	// DefaultBreakerInterval 默认熔断器间隔（毫秒）
	DefaultBreakerInterval = 10000

	// DefaultIdleTotal 默认总空闲连接数
	DefaultIdleTotal = 100

	// DefaultIdlePerHost 默认每主机空闲连接数
	DefaultIdlePerHost = 10

	// DefaultMaxPerHost 默认每主机最大连接数
	DefaultMaxPerHost = 50

	// DefaultWeight 默认权重
	DefaultWeight = 1
)
