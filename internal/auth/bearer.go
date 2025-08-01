package auth

import (
	"errors"
	"net/http"
	"strings"
)

// 认证相关错误定义
var (
	ErrEmptyToken = errors.New("bearer token cannot be empty")
)

// bearerAuthenticator 代表Bearer Token认证实现
type bearerAuthenticator struct {
	token string // Bearer Token值
}

// NewBearerAuthenticator 创建新的Bearer Token认证器
// token: Bearer Token值
func NewBearerAuthenticator(token string) (Authenticator, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrEmptyToken
	}

	return &bearerAuthenticator{
		token: strings.TrimSpace(token),
	}, nil
}

// Apply 将Bearer Token应用到HTTP请求的Authorization头部
// req: 要应用认证的HTTP请求
func (a *bearerAuthenticator) Apply(req *http.Request) error {
	if req == nil {
		return errors.New("request cannot be nil")
	}

	// 设置Authorization头部为Bearer Token格式
	req.Header.Set("Authorization", "Bearer "+a.token)
	return nil
}

// Type 获取认证器类型
func (a *bearerAuthenticator) Type() string {
	return "bearer"
}
