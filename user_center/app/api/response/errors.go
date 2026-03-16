package response

import (
	"fmt"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// APIError 自定义 API 错误类型，实现 error 接口

type APIError struct {
	HTTPCode int    // 对应的底层 HTTP 状态码 (如 400, 401, 500)
	BizCode  int    // 约定的业务专属错误码 (如 401001)
	Message  string // 错误描述
}

func (e *APIError) Error() string {
	return fmt.Sprintf("HTTPCode: %d, BizCode: %d, Message: %s", e.HTTPCode, e.BizCode, e.Message)
}

// 业务错误字典 (Error Dictionary)
// 团队开发中，所有的错误类型在此处收敛，严禁在业务代码里硬编码错误码和错误信息。

var (
	// 400 参数校验与客户端类错误
	ErrInvalidParams = &APIError{HTTPCode: consts.StatusBadRequest, BizCode: 400001, Message: "请求参数错误或格式非法"}
	//ErrMissingHeader     = &APIError{HTTPCode: consts.StatusBadRequest, BizCode: 400002, Message: "缺失必要的请求头参数"}
	ErrInvalidPassword   = &APIError{HTTPCode: consts.StatusBadRequest, BizCode: 400003, Message: "账号或密码错误"}
	ErrUserAlreadyExists = &APIError{HTTPCode: consts.StatusBadRequest, BizCode: 400004, Message: "账号已被注册"}

	// 401/403 认证与权限类错误 (用户中心核心使用)
	ErrUnauthorized = &APIError{HTTPCode: consts.StatusUnauthorized, BizCode: 401001, Message: "Token无效或已过期，请重新登录"}
	ErrPermission   = &APIError{HTTPCode: consts.StatusForbidden, BizCode: 403001, Message: "权限不足，拒绝访问"}
	ErrUserBanned   = &APIError{HTTPCode: consts.StatusForbidden, BizCode: 403002, Message: "该账号已被封禁或未激活"}

	// 404 资源不存在
	ErrUserNotFound = &APIError{HTTPCode: consts.StatusNotFound, BizCode: 404001, Message: "该账号/用户不存在"}

	// 500 服务器内部错误
	ErrSystemError   = &APIError{HTTPCode: consts.StatusInternalServerError, BizCode: 500000, Message: "服务器内部异常，请稍后再试"}
	ErrDatabaseError = &APIError{HTTPCode: consts.StatusInternalServerError, BizCode: 500001, Message: "数据库操作异常"}
)
