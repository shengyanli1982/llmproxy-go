// Package constants 定义项目中使用的应用级常量
package constants

const (
	// Application metadata - 应用程序元数据

	// DefaultVersion 应用程序默认版本号
	DefaultVersion = "0.0.0"

	// AppName 应用程序名称
	AppName = "LLMProxy"

	// UserAgent 默认HTTP用户代理字符串
	UserAgent = "LLMProxy/1.0"

	// DefaultConfigPath 默认配置文件路径
	DefaultConfigPath = "./config.yaml"
)

const (
	// Exit codes - 程序退出码

	// ExitFailure 程序异常退出码
	ExitFailure = -1

	// ExitSuccess 程序正常退出码
	ExitSuccess = 0
)
