package auth

import (
	"net/http"
)

// noneAuthenticator 代表无认证实现
type noneAuthenticator struct{}

// NewNoneAuthenticator 创建新的无认证认证器
func NewNoneAuthenticator() Authenticator {
	return &noneAuthenticator{}
}

// Apply 对HTTP请求不进行任何认证操作
// req: HTTP请求（不会被修改）
func (a *noneAuthenticator) Apply(req *http.Request) error {
	// 无认证模式下不对请求进行任何修改
	return nil
}

// Type 获取认证器类型
func (a *noneAuthenticator) Type() string {
	return "none"
}