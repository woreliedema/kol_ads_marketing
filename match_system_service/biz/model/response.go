package model

// BaseResponse 全局基础响应体（带有泛型，专为 Swagger 文档生成器设计）
type BaseResponse[T any] struct {
	Code int    `json:"code" example:"200"`
	Msg  string `json:"msg" example:"success"`
	Data T      `json:"data"`
}
