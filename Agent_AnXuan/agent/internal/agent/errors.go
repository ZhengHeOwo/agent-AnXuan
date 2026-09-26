package agent

import (
	"errors"
)

var (
	ErrModelInvocationFailed = errors.New("调用模型失败")

	ErrResponseRoleError = errors.New("响应信息角色错误")

	ErrEmptyContent = errors.New("响应内容为空")

	ErrInvalidToolCall = errors.New("无效工具调用")

	ErrMaxStepsExceeded = errors.New("超过最大模型执行步数")
)
