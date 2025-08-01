package server

import (
	"errors"

	"github.com/shengyanli1982/llmproxy-go/internal/constants"
)

// 服务器相关错误定义
var (
	// 服务器状态错误
	ErrServerAlreadyStarted = errors.New(constants.ErrMsgServerAlreadyStarted)
	ErrServerNotStarted     = errors.New(constants.ErrMsgServerNotStarted)
	ErrServerIsNotRunning   = errors.New(constants.ErrMsgServerNotRunning)

	// 服务状态错误
	ErrServiceAlreadyStarted = errors.New(constants.ErrMsgServiceAlreadyStarted)
	ErrServiceNotStarted     = errors.New(constants.ErrMsgServiceNotStarted)
	ErrServiceIsNotRunning   = errors.New(constants.ErrMsgServiceNotRunning)
)
