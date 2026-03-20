package response

import (
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// Response 统一的 API JSON 返回结构体
type Response struct {
	Code    int         `json:"code"`    // 业务状态码 (非HTTP状态码，例如 0 表示成功，40001 表示参数错误)
	Message string      `json:"message"` // 给前端展示或排查问题的提示语
	Data    interface{} `json:"data"`    // 实际的业务数据载荷 (如果没有数据，最好返回空结构体或 nil)
}

// Success 成功响应
func Success(ctx *app.RequestContext, data interface{}) {
	ctx.JSON(consts.StatusOK, Response{
		Code:    0, // 0 约定为全局业务成功码
		Message: "success",
		Data:    data,
	})
}

// Error 失败响应 (结合业务错误字典使用)
func Error(ctx *app.RequestContext, apiErr *APIError) {
	ctx.JSON(apiErr.HTTPCode, Response{
		Code:    apiErr.BizCode,
		Message: apiErr.Message,
		Data:    nil, // 错误时没有业务数据
	})
}

// ErrorWithMsg 动态覆盖错误信息的失败响应
func ErrorWithMsg(ctx *app.RequestContext, apiErr *APIError, customMsg string) {
	ctx.JSON(apiErr.HTTPCode, Response{
		Code:    apiErr.BizCode,
		Message: customMsg,
		Data:    nil,
	})
}
