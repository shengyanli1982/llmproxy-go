package headers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
)

// 头部操作相关错误定义
var (
	ErrInvalidOperation = errors.New("invalid header operation")
	ErrEmptyHeaderKey   = errors.New("header key cannot be empty")
	ErrNilHeader        = errors.New("header cannot be nil")
)

// HeaderOperator 代表HTTP头部操作器接口
type HeaderOperator interface {
	// Process 批量处理HTTP头部操作
	// headers: 要操作的HTTP头部
	// ops: 操作配置列表
	Process(headers http.Header, ops []config.HeaderOpConfig) error

	// ProcessSingle 处理单个HTTP头部操作
	// headers: 要操作的HTTP头部
	// op: 单个操作配置
	ProcessSingle(headers http.Header, op config.HeaderOpConfig) error
}

// defaultOperator 代表默认头部操作器实现
type defaultOperator struct{}

// NewOperator 创建新的HTTP头部操作器
func NewOperator() HeaderOperator {
	return &defaultOperator{}
}

// Process 批量处理HTTP头部操作，按配置顺序执行
// headers: 要操作的HTTP头部
// ops: 操作配置列表
func (o *defaultOperator) Process(headers http.Header, ops []config.HeaderOpConfig) error {
	if headers == nil {
		return ErrNilHeader
	}

	// 按顺序执行每个操作
	for i, op := range ops {
		if err := o.ProcessSingle(headers, op); err != nil {
			return errors.New("operation " + string(rune(i)) + " failed: " + err.Error())
		}
	}

	return nil
}

// ProcessSingle 处理单个HTTP头部操作
// headers: 要操作的HTTP头部
// op: 单个操作配置
func (o *defaultOperator) ProcessSingle(headers http.Header, op config.HeaderOpConfig) error {
	if headers == nil {
		return ErrNilHeader
	}

	// 验证头部键名
	if strings.TrimSpace(op.Key) == "" {
		return ErrEmptyHeaderKey
	}

	key := strings.TrimSpace(op.Key)

	switch strings.ToLower(op.Op) {
	case "insert":
		return o.insertHeader(headers, key, op.Value)
	case "replace":
		return o.replaceHeader(headers, key, op.Value)
	case "remove":
		return o.removeHeader(headers, key)
	default:
		return errors.New(ErrInvalidOperation.Error() + ": " + op.Op)
	}
}

// insertHeader 插入头部，如果头部不存在则插入，存在则不操作
// headers: HTTP头部
// key: 头部键名
// value: 头部值
func (o *defaultOperator) insertHeader(headers http.Header, key, value string) error {
	// 如果头部不存在，则插入
	if headers.Get(key) == "" {
		headers.Set(key, value)
	}
	// 如果已存在，则不进行任何操作
	return nil
}

// replaceHeader 替换头部，如果头部存在则替换，不存在则插入
// headers: HTTP头部
// key: 头部键名
// value: 头部值
func (o *defaultOperator) replaceHeader(headers http.Header, key, value string) error {
	// 无论是否存在都直接设置值
	headers.Set(key, value)
	return nil
}

// removeHeader 删除指定头部
// headers: HTTP头部
// key: 头部键名
func (o *defaultOperator) removeHeader(headers http.Header, key string) error {
	headers.Del(key)
	return nil
}
