// match_system_service/pkg/response/errors.go
package response

import (
	"fmt"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type APIError struct {
	HTTPCode int
	BizCode  int
	Message  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("HTTPCode: %d, BizCode: %d, Message: %s", e.HTTPCode, e.BizCode, e.Message)
}

// 匹配服务错误字典 (服务前缀约定为: 2xxxxxx)
var (
	// --- 通用与基础类错误 (沿用逻辑，修改BizCode加上前缀2) ---
	ErrInvalidParams = &APIError{HTTPCode: consts.StatusBadRequest, BizCode: 2400001, Message: "请求参数错误或格式非法"}
	ErrUnauthorized  = &APIError{HTTPCode: consts.StatusUnauthorized, BizCode: 2401001, Message: "Token无效或已过期，请重新登录"}
	ErrPermission    = &APIError{HTTPCode: consts.StatusForbidden, BizCode: 2403001, Message: "权限不足，拒绝访问"}
	ErrSystemError   = &APIError{HTTPCode: consts.StatusInternalServerError, BizCode: 2500000, Message: "匹配系统内部异常，请稍后再试"}

	// --- 匹配系统专属业务错误 ---
	// ES 搜索类
	ErrESQueryFailed = &APIError{HTTPCode: consts.StatusInternalServerError, BizCode: 2500101, Message: "搜索引擎服务异常"}
	ErrInvalidTags   = &APIError{HTTPCode: consts.StatusBadRequest, BizCode: 2400101, Message: "包含不支持的筛选标签"}

	// 红人/品牌方业务类
	ErrMatchRuleConflict = &APIError{HTTPCode: consts.StatusBadRequest, BizCode: 2400201, Message: "匹配规则冲突（如粉丝量下限大于上限）"}
	ErrDemandNotFound    = &APIError{HTTPCode: consts.StatusNotFound, BizCode: 2404001, Message: "该营销需求不存在或已下架"}
)
