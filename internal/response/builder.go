// Package response 提供基于httptool.BaseHttpResponse的统一HTTP响应格式
//
// 这个包实现了统一的响应构建器，支持：
//   - 标准成功和错误响应格式
//   - 链式调用和灵活配置
//   - 分页响应支持
//   - 与gin框架无缝集成
//
// 基本用法：
//
//	// 成功响应
//	response.Success(data).JSON(c, http.StatusOK)
//
//	// 错误响应
//	response.Error(CodeBadRequest, "参数错误").WithDetail(details).JSON(c, http.StatusBadRequest)
//
//	// 便捷方法
//	response.OK(c, data)
//	response.BadRequest(c, "参数错误")
//
//	// 分页响应
//	response.Paginated(data, totalCount, pageIndex, pageSize, desc).JSON(c, http.StatusOK)
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shengyanli1982/toolkit/pkg/httptool"
)

// 响应代码常量定义
const (
	// CodeSuccess 表示操作成功
	CodeSuccess = 0

	// 1000-1999: 客户端错误
	CodeBadRequest   = 1000 // 请求参数错误
	CodeUnauthorized = 1001 // 未授权访问
	CodeForbidden    = 1002 // 禁止访问
	CodeNotFound     = 1003 // 资源未找到
	CodeRateLimit    = 1004 // 请求频率限制

	// 2000-2999: 服务器错误
	CodeInternalError      = 2000 // 服务器内部错误
	CodeBadGateway         = 2001 // 网关错误
	CodeServiceUnavailable = 2002 // 服务不可用
	CodeGatewayTimeout     = 2003 // 网关超时

	// 3000-3999: 业务逻辑错误
	CodeCircuitBreaker = 3000 // 熔断器开启
	CodeUpstreamLimit  = 3001 // 上游服务限流
)

// ResponseBuilder 是基于httptool.BaseHttpResponse的统一响应构建器
type ResponseBuilder struct {
	response *httptool.BaseHttpResponse
}

// Success 创建成功响应构建器
func Success(data interface{}) *ResponseBuilder {
	return &ResponseBuilder{
		response: &httptool.BaseHttpResponse{
			Code: CodeSuccess,
			Data: data,
		},
	}
}

// Error 创建错误响应构建器
func Error(code int64, message string) *ResponseBuilder {
	return &ResponseBuilder{
		response: &httptool.BaseHttpResponse{
			Code:         code,
			ErrorMessage: message,
		},
	}
}

// WithDetail 添加错误详细信息，支持链式调用
func (r *ResponseBuilder) WithDetail(detail interface{}) *ResponseBuilder {
	r.response.ErrorDetail = detail
	return r
}

// WithData 设置响应数据，支持链式调用
func (r *ResponseBuilder) WithData(data interface{}) *ResponseBuilder {
	r.response.Data = data
	return r
}

// JSON 将响应输出为JSON格式到gin.Context
func (r *ResponseBuilder) JSON(c *gin.Context, httpStatus int) {
	c.JSON(httpStatus, r.response)
}

// GetResponse 获取底层的BaseHttpResponse对象
func (r *ResponseBuilder) GetResponse() *httptool.BaseHttpResponse {
	return r.response
}

// 便捷方法：常见的成功响应

// OK 返回标准的成功响应（HTTP 200）
func OK(c *gin.Context, data interface{}) {
	Success(data).JSON(c, http.StatusOK)
}

// Created 返回创建成功响应（HTTP 201）
func Created(c *gin.Context, data interface{}) {
	Success(data).JSON(c, http.StatusCreated)
}

// NoContent 返回无内容响应（HTTP 204）
func NoContent(c *gin.Context) {
	Success(nil).JSON(c, http.StatusNoContent)
}

// 便捷方法：常见的错误响应

// BadRequest 返回客户端请求错误响应（HTTP 400）
func BadRequest(c *gin.Context, message string) {
	Error(CodeBadRequest, message).JSON(c, http.StatusBadRequest)
}

// Unauthorized 返回未授权错误响应（HTTP 401）
func Unauthorized(c *gin.Context, message string) {
	Error(CodeUnauthorized, message).JSON(c, http.StatusUnauthorized)
}

// Forbidden 返回禁止访问错误响应（HTTP 403）
func Forbidden(c *gin.Context, message string) {
	Error(CodeForbidden, message).JSON(c, http.StatusForbidden)
}

// NotFound 返回资源未找到错误响应（HTTP 404）
func NotFound(c *gin.Context, message string) {
	Error(CodeNotFound, message).JSON(c, http.StatusNotFound)
}

// TooManyRequests 返回请求过多错误响应（HTTP 429）
func TooManyRequests(c *gin.Context, message string) {
	Error(CodeRateLimit, message).JSON(c, http.StatusTooManyRequests)
}

// InternalServerError 返回服务器内部错误响应（HTTP 500）
func InternalServerError(c *gin.Context, message string) {
	Error(CodeInternalError, message).JSON(c, http.StatusInternalServerError)
}

// BadGateway 返回网关错误响应（HTTP 502）
func BadGateway(c *gin.Context, message string) {
	Error(CodeBadGateway, message).JSON(c, http.StatusBadGateway)
}

// ServiceUnavailable 返回服务不可用错误响应（HTTP 503）
func ServiceUnavailable(c *gin.Context, message string) {
	Error(CodeServiceUnavailable, message).JSON(c, http.StatusServiceUnavailable)
}

// GatewayTimeout 返回网关超时错误响应（HTTP 504）
func GatewayTimeout(c *gin.Context, message string) {
	Error(CodeGatewayTimeout, message).JSON(c, http.StatusGatewayTimeout)
}

// PaginatedResponseBuilder 是分页响应构建器
type PaginatedResponseBuilder struct {
	response *httptool.HttpResponsePaginated
}

// Paginated 创建分页响应构建器
func Paginated(data interface{}, totalCount int64, pageIndex int64, pageSize int64, desc bool) *PaginatedResponseBuilder {
	return &PaginatedResponseBuilder{
		response: &httptool.HttpResponsePaginated{
			HttpResponseItemsTotal: httptool.HttpResponseItemsTotal{
				TotalCount: totalCount,
			},
			HttpQueryPaginated: httptool.HttpQueryPaginated{
				PageIndex: pageIndex,
				PageSize:  pageSize,
				Desc:      desc,
			},
			BaseHttpResponse: httptool.BaseHttpResponse{
				Code: CodeSuccess,
				Data: data,
			},
		},
	}
}

// WithError 为分页响应设置错误信息
func (p *PaginatedResponseBuilder) WithError(code int64, message string) *PaginatedResponseBuilder {
	p.response.BaseHttpResponse.Code = code
	p.response.BaseHttpResponse.ErrorMessage = message
	return p
}

// WithDetail 为分页响应添加错误详细信息
func (p *PaginatedResponseBuilder) WithDetail(detail interface{}) *PaginatedResponseBuilder {
	p.response.BaseHttpResponse.ErrorDetail = detail
	return p
}

// JSON 将分页响应输出为JSON格式到gin.Context
func (p *PaginatedResponseBuilder) JSON(c *gin.Context, httpStatus int) {
	c.JSON(httpStatus, p.response)
}

// GetResponse 获取底层的HttpResponsePaginated对象
func (p *PaginatedResponseBuilder) GetResponse() *httptool.HttpResponsePaginated {
	return p.response
}
