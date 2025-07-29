package server

import (
	"errors"
)

// 服务器相关错误定义
var (
	// 服务器状态错误
	ErrServerAlreadyStarted = errors.New("server already started")
	ErrServerNotStarted     = errors.New("server not started")
	ErrServerIsNotRunning   = errors.New("server is not running")

	// 服务状态错误
	ErrServiceAlreadyStarted = errors.New("service already started")
	ErrServiceNotStarted     = errors.New("service not started")
	ErrServiceIsNotRunning   = errors.New("service is not running")
)
