package headers

import (
	"net/http"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/constants"
)

// Processor 代表HTTP头部处理器，提供便捷的头部操作方法
type Processor struct {
	operator HeaderOperator // 头部操作器
}

// NewProcessor 创建新的HTTP头部处理器
func NewProcessor() *Processor {
	return &Processor{
		operator: NewOperator(),
	}
}

// NewProcessorWithOperator 使用指定操作器创建处理器
// operator: 自定义头部操作器
func NewProcessorWithOperator(operator HeaderOperator) *Processor {
	return &Processor{
		operator: operator,
	}
}

// ApplyToRequest 将头部操作应用到HTTP请求
// req: 要操作的HTTP请求
// ops: 头部操作配置列表
func (p *Processor) ApplyToRequest(req *http.Request, ops []config.HeaderOpConfig) error {
	if req == nil {
		return ErrNilHeader
	}

	return p.operator.Process(req.Header, ops)
}

// ApplyFromUpstreamConfig 从上游配置应用头部操作
// req: 要操作的HTTP请求
// upstreamConfig: 上游配置
func (p *Processor) ApplyFromUpstreamConfig(req *http.Request, upstreamConfig *config.UpstreamConfig) error {
	if req == nil {
		return ErrNilHeader
	}

	if upstreamConfig == nil || len(upstreamConfig.Headers) == 0 {
		// 没有头部操作配置，直接返回
		return nil
	}

	return p.operator.Process(req.Header, upstreamConfig.Headers)
}

// InsertHeader 插入单个头部
// req: HTTP请求
// key: 头部键名
// value: 头部值
func (p *Processor) InsertHeader(req *http.Request, key, value string) error {
	if req == nil {
		return ErrNilHeader
	}

	op := config.HeaderOpConfig{
		Op:    constants.HeaderOpInsert,
		Key:   key,
		Value: value,
	}

	return p.operator.ProcessSingle(req.Header, op)
}

// ReplaceHeader 替换单个头部
// req: HTTP请求
// key: 头部键名
// value: 头部值
func (p *Processor) ReplaceHeader(req *http.Request, key, value string) error {
	if req == nil {
		return ErrNilHeader
	}

	op := config.HeaderOpConfig{
		Op:    constants.HeaderOpReplace,
		Key:   key,
		Value: value,
	}

	return p.operator.ProcessSingle(req.Header, op)
}

// RemoveHeader 删除单个头部
// req: HTTP请求
// key: 头部键名
func (p *Processor) RemoveHeader(req *http.Request, key string) error {
	if req == nil {
		return ErrNilHeader
	}

	op := config.HeaderOpConfig{
		Op:  constants.HeaderOpRemove,
		Key: key,
	}

	return p.operator.ProcessSingle(req.Header, op)
}

// GetOperator 获取内部使用的头部操作器
func (p *Processor) GetOperator() HeaderOperator {
	return p.operator
}
