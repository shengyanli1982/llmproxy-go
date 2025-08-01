package constants

const (
	// HTTP schemes - HTTP协议方案

	// SchemeHTTP HTTP协议前缀
	SchemeHTTP = "http://"

	// SchemeHTTPS HTTPS协议前缀
	SchemeHTTPS = "https://"

	// DefaultScheme 默认协议方案
	DefaultScheme = SchemeHTTP

	// Protocol names for validation - 协议名称用于验证

	// ProtocolHTTP HTTP协议名称
	ProtocolHTTP = "http"

	// ProtocolHTTPS HTTPS协议名称
	ProtocolHTTPS = "https"
)

const (
	// HTTP headers - HTTP头部

	// HeaderUserAgent User-Agent头部名称
	HeaderUserAgent = "User-Agent"

	// HeaderConnection Connection头部名称
	HeaderConnection = "Connection"

	// HeaderXForwardedHost X-Forwarded-Host头部名称
	HeaderXForwardedHost = "X-Forwarded-Host"

	// HeaderXForwardedFor X-Forwarded-For头部名称
	HeaderXForwardedFor = "X-Forwarded-For"

	// HeaderXForwardedProto X-Forwarded-Proto头部名称
	HeaderXForwardedProto = "X-Forwarded-Proto"

	// HeaderXRealIP X-Real-IP头部名称
	HeaderXRealIP = "X-Real-IP"

	// HeaderXRequestID X-Request-ID头部名称
	HeaderXRequestID = "X-Request-ID"

	// HeaderAuthorization Authorization头部名称
	HeaderAuthorization = "Authorization"

	// HeaderContentType Content-Type头部名称
	HeaderContentType = "Content-Type"

	// HeaderTransferEncoding Transfer-Encoding头部名称
	HeaderTransferEncoding = "Transfer-Encoding"
)

const (
	// Connection values - 连接值

	// ConnectionClose 关闭连接值
	ConnectionClose = "close"

	// ConnectionKeepAlive 保持连接值
	ConnectionKeepAlive = "keep-alive"
)

const (
	// Authentication types - 认证类型

	// AuthTypeNone 无认证类型
	AuthTypeNone = "none"

	// AuthTypeBearer Bearer令牌认证类型
	AuthTypeBearer = "bearer"

	// AuthTypeBasic Basic认证类型
	AuthTypeBasic = "basic"
)

const (
	// Authentication prefixes - 认证前缀

	// BearerPrefix Bearer令牌前缀
	BearerPrefix = "Bearer "

	// BasicPrefix Basic认证前缀
	BasicPrefix = "Basic "
)

const (
	// Header operations - 头部操作类型

	// HeaderOpInsert 插入头部操作
	HeaderOpInsert = "insert"

	// HeaderOpReplace 替换头部操作
	HeaderOpReplace = "replace"

	// HeaderOpRemove 移除头部操作
	HeaderOpRemove = "remove"
)

const (
	// Content types - 内容类型

	// ContentTypeEventStream 服务器发送事件流内容类型
	ContentTypeEventStream = "text/event-stream"

	// ContentTypeStreamJSON JSON流内容类型
	ContentTypeStreamJSON = "application/stream+json"
)

const (
	// Transfer encodings - 传输编码

	// TransferEncodingChunked 分块传输编码
	TransferEncodingChunked = "chunked"
)
