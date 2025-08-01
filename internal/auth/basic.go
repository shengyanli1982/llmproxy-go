package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/shengyanli1982/llmproxy-go/internal/constants"
)

// Basic认证相关错误定义
var (
	ErrEmptyUsername = errors.New("username cannot be empty")
	ErrEmptyPassword = errors.New("password cannot be empty")
)

// basicAuthenticator 代表Basic Auth认证实现
type basicAuthenticator struct {
	username string // 用户名
	password string // 密码
}

// NewBasicAuthenticator 创建新的Basic Auth认证器
// username: 用户名
// password: 密码
func NewBasicAuthenticator(username, password string) (Authenticator, error) {
	if strings.TrimSpace(username) == "" {
		return nil, ErrEmptyUsername
	}
	if strings.TrimSpace(password) == "" {
		return nil, ErrEmptyPassword
	}

	return &basicAuthenticator{
		username: strings.TrimSpace(username),
		password: strings.TrimSpace(password),
	}, nil
}

// Apply 将Basic Auth应用到HTTP请求的Authorization头部
// req: 要应用认证的HTTP请求
func (a *basicAuthenticator) Apply(req *http.Request) error {
	if req == nil {
		return errors.New("request cannot be nil")
	}

	// 构造Basic Auth凭据
	credentials := a.username + ":" + a.password
	encodedCredentials := base64.StdEncoding.EncodeToString([]byte(credentials))

	// 设置Authorization头部为Basic Auth格式
	req.Header.Set(constants.HeaderAuthorization, constants.BasicPrefix+encodedCredentials)
	return nil
}

// Type 获取认证器类型
func (a *basicAuthenticator) Type() string {
	return constants.AuthTypeBasic
}
