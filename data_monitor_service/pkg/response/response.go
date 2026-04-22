package response

import (
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(ctx *app.RequestContext, data interface{}) {
	ctx.JSON(consts.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// APIError 定义一个基础错误体
type APIError struct {
	HTTPCode int
	BizCode  int
	Message  string
}

// 预定义错误枚举
var (
	ErrUnauthorized = &APIError{HTTPCode: consts.StatusUnauthorized, BizCode: 40100, Message: "未授权"}
	ErrSystemError  = &APIError{HTTPCode: consts.StatusInternalServerError, BizCode: 50000, Message: "系统内部错误"}
)

func ErrorWithMsg(ctx *app.RequestContext, apiErr *APIError, customMsg string) {
	ctx.JSON(apiErr.HTTPCode, Response{
		Code:    apiErr.BizCode,
		Message: customMsg,
		Data:    nil,
	})
}
