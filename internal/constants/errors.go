package constants

const (
	// Error messages - 错误消息

	// ErrMsgServerAlreadyStarted 服务器已启动错误消息
	ErrMsgServerAlreadyStarted = "server already started"

	// ErrMsgServerNotStarted 服务器未启动错误消息
	ErrMsgServerNotStarted = "server not started"

	// ErrMsgServerNotRunning 服务器未运行错误消息
	ErrMsgServerNotRunning = "server is not running"

	// ErrMsgServiceAlreadyStarted 服务已启动错误消息
	ErrMsgServiceAlreadyStarted = "service already started"

	// ErrMsgServiceNotStarted 服务未启动错误消息
	ErrMsgServiceNotStarted = "service not started"

	// ErrMsgServiceNotRunning 服务未运行错误消息
	ErrMsgServiceNotRunning = "service is not running"

	// ErrMsgNilRequest 空请求错误消息
	ErrMsgNilRequest = "request cannot be nil"

	// ErrMsgNilUpstream 空上游错误消息
	ErrMsgNilUpstream = "upstream cannot be nil"

	// ErrMsgClientClosed 客户端已关闭错误消息
	ErrMsgClientClosed = "client is closed"

	// ErrMsgInvalidTimeout 无效超时配置错误消息
	ErrMsgInvalidTimeout = "invalid timeout configuration"

	// ErrMsgNoAvailableUpstream 无可用上游错误消息
	ErrMsgNoAvailableUpstream = "no available upstream"

	// ErrMsgUnknownStrategy 未知策略错误消息
	ErrMsgUnknownStrategy = "unknown load balance strategy"

	// ErrMsgNilUpstreams 空上游列表错误消息
	ErrMsgNilUpstreams = "upstreams cannot be nil"

	// ErrMsgEmptyUpstreams 空上游列表错误消息
	ErrMsgEmptyUpstreams = "upstreams cannot be empty"

	// ErrMsgNilBalanceConfig 空负载均衡配置错误消息
	ErrMsgNilBalanceConfig = "balance config cannot be nil"
)

const (
	// Error types for metrics - 指标错误类型

	// ErrorTypeProcessing 处理错误类型
	ErrorTypeProcessing = "processing_error"

	// ErrorTypeSelection 选择错误类型
	ErrorTypeSelection = "selection_failed"

	// ErrorTypeExecution 执行错误类型
	ErrorTypeExecution = "execution_error"

	// ErrorTypeUnknown 未知错误类型
	ErrorTypeUnknown = "unknown"
)
