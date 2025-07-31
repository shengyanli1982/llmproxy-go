package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"

	"github.com/shengyanli1982/gs"
	"github.com/shengyanli1982/law"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/server"
	"github.com/shengyanli1982/orbit/utils/log"
)

// Version 通过 ldflags 在编译时设置
var Version = "0.2.0"

const ASCII_LOGO = `
██╗     ██╗     ███╗   ███╗██████╗ ██████╗  ██████╗ ██╗  ██╗██╗   ██╗
██║     ██║     ████╗ ████║██╔══██╗██╔══██╗██╔═══██╗╚██╗██╔╝╚██╗ ██╔╝
██║     ██║     ██╔████╔██║██████╔╝██████╔╝██║   ██║ ╚███╔╝  ╚████╔╝
██║     ██║     ██║╚██╔╝██║██╔═══╝ ██╔══██╗██║   ██║ ██╔██╗   ╚██╔╝
███████╗███████╗██║ ╚═╝ ██║██║     ██║  ██║╚██████╔╝██╔╝ ██╗   ██║
╚══════╝╚══════╝╚═╝     ╚═╝╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝

██╗  ██╗██╗ ██████╗ ██╗  ██╗    ██████╗ ███████╗██████╗ ███████╗
██║  ██║██║██╔════╝ ██║  ██║    ██╔══██╗██╔════╝██╔══██╗██╔════╝
███████║██║██║  ███╗███████║    ██████╔╝█████╗  ██████╔╝█████╗
██╔══██║██║██║   ██║██╔══██║    ██╔═══╝ ██╔══╝  ██╔══██╗██╔══╝
██║  ██║██║╚██████╔╝██║  ██║    ██║     ███████╗██║  ██║██║
╚═╝  ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═╝    ╚═╝     ╚══════╝╚═╝  ╚═╝╚═╝

██████╗ ██████╗  ██████╗ ██╗  ██╗██╗   ██╗    ███████╗███████╗██████╗ ██╗   ██╗██╗ ██████╗███████╗
██╔══██╗██╔══██╗██╔═══██╗╚██╗██╔╝╚██╗ ██╔╝    ██╔════╝██╔════╝██╔══██╗██║   ██║██║██╔════╝██╔════╝
██████╔╝██████╔╝██║   ██║ ╚███╔╝  ╚████╔╝     ███████╗█████╗  ██████╔╝██║   ██║██║██║     █████╗
██╔═══╝ ██╔══██╗██║   ██║ ██╔██╗   ╚██╔╝      ╚════██║██╔══╝  ██╔══██╗╚██╗ ██╔╝██║██║     ██╔══╝
██║     ██║  ██║╚██████╔╝██╔╝ ██╗   ██║       ███████║███████╗██║  ██║ ╚████╔╝ ██║╚██████╗███████╗
╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝       ╚══════╝╚══════╝╚═╝  ╚═╝  ╚═══╝  ╚═╝ ╚═════╝╚══════╝
	`

// ServiceContext 服务上下文结构体，用于管理服务所需的所有组件
type ServiceContext struct {
	logger      *logr.Logger      // 日志记录器
	asyncWriter *law.WriteAsyncer // 异步写入器
	config      *config.Config    // 服务配置
	configMgr   *config.Manager   // 配置管理器
	proxyServer *server.Server    // 代理服务器
}

// isReleaseMode 判断是否为发布模式
// releaseMode: 是否为发布模式
func isReleaseMode(releaseMode bool) bool {
	return releaseMode || gin.Mode() == gin.ReleaseMode
}

// initLogger 初始化日志系统
// releaseMode: 是否为发布模式
// jsonOutput: 是否输出 JSON 格式日志
func initLogger(releaseMode, jsonOutput bool) (*logr.Logger, *law.WriteAsyncer) {
	var (
		logger      *logr.Logger
		asyncWriter *law.WriteAsyncer
	)

	// 在发布模式下使用异步写入器
	if isReleaseMode(releaseMode) {
		asyncWriter = law.NewWriteAsyncer(os.Stdout, law.DefaultConfig())
		if jsonOutput {
			// JSON 格式输出使用 ZapLogger
			logger = log.NewZapLogger(zapcore.AddSync(asyncWriter)).GetLogrLogger()
		} else {
			// 普通格式输出使用 LogrLogger
			logger = log.NewLogrLogger(asyncWriter).GetLogrLogger()
		}
		return logger, asyncWriter
	}

	// 开发模式直接使用标准输出
	logger = log.NewLogrLogger(os.Stdout).GetLogrLogger()
	return logger, nil
}

// initConfig 初始化配置管理器
// configPath: 配置文件路径
func initConfig(configPath string) (*config.Manager, *config.Config, error) {
	configManager, err := config.NewManager()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create configuration manager: %w", err)
	}
	if err := configManager.LoadFromFile(configPath); err != nil {
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	cfg := configManager.GetConfig()
	return configManager, cfg, nil
}

// setupGracefulShutdown 设置优雅关闭机制
// ctx: 服务上下文
// releaseMode: 是否为发布模式
func setupGracefulShutdown(ctx *ServiceContext, releaseMode bool) {
	// 创建服务器终止信号
	serverSignal := gs.NewTerminateSignal()
	serverSignal.RegisterCancelHandles(ctx.proxyServer.Stop)

	// 创建写入器终止信号
	writerSignal := gs.NewTerminateSignal()
	if isReleaseMode(releaseMode) && ctx.asyncWriter != nil {
		writerSignal.RegisterCancelHandles(ctx.asyncWriter.Stop)
	}

	// 等待所有终止信号完成
	gs.WaitForSync(serverSignal, writerSignal)
}

func main() {
	// 定义命令行参数
	var (
		configPath  string
		releaseMode bool
		jsonOutput  bool
	)

	// 设置命令行参数
	cmd := cobra.Command{
		Use:     "llmproxy",
		Version: Version,
		Short:   "LLMProxy is a high-performance HTTP proxy for LLM services",
		Long: `LLMProxy is a high-performance HTTP proxy service designed for LLM APIs.

Core Features:
- High-performance LLM API proxy service
- Intelligent load balancing and circuit breaking
- Configurable timeout and retry mechanisms
- Real-time request rate limiting
- Graceful shutdown support
- JSON/Plain log output support

Performance Characteristics:
- Connection pool optimized for LLM APIs
- Memory-efficient design
- Production-grade stability
- Asynchronous logging system
- Smart request routing

Technical Specifications:
- Multi-upstream LLM service support
- Automatic failover handling
- Real-time metrics monitoring
- Configurable compression support

Author: shengyanli1982
Repository: https://github.com/shengyanli1982/llmproxy-go`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 创建服务上下文
			ctx := &ServiceContext{}

			// 初始化日志系统
			ctx.logger, ctx.asyncWriter = initLogger(releaseMode, jsonOutput)

			// 加载服务配置
			var err error
			ctx.configMgr, ctx.config, err = initConfig(configPath)
			if err != nil {
				ctx.logger.Error(err, "Failed to load service configuration")
				return err
			}

			ctx.logger.Info("Configuration loaded successfully", "path", ctx.configMgr.GetConfigPath())

			// 输出 ASCII 标志（只有在配置加载成功后才显示）
			fmt.Println(ASCII_LOGO)

			// 创建代理服务器
			ctx.proxyServer = server.NewServer(!releaseMode, ctx.logger, &ctx.config.HTTPServer, ctx.config)

			// 启动代理服务
			ctx.proxyServer.Start()
			ctx.logger.Info("LLMProxy started successfully")

			// 设置优雅关闭机制
			setupGracefulShutdown(ctx, releaseMode)

			ctx.logger.Info("LLMProxy stopped")
			return nil
		},
	}

	// 注册命令行参数
	cmd.Flags().StringVarP(&configPath, "config", "c", "./config.yaml", "Path to configuration file")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Enable JSON format logging output (only effective in release mode)")
	cmd.Flags().BoolVarP(&releaseMode, "release", "r", false, "Enable release mode for performance optimizations and async logging")

	// 执行命令
	if err := cmd.Execute(); err != nil {
		fmt.Printf("Failed to execute command: %v\n", err)
		os.Exit(-1)
	}
}
