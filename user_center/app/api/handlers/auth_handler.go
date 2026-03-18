package handlers

import (
	"context"
	"errors"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/models"
	service "kol_ads_marketing/user_center/app/service"
)

// 数据传输对象 (DTO) 定义

type RegisterReq struct {
	Username string          `json:"username" vd:"required,len>4;msg:'用户名必须大于4个字符'"`
	Password string          `json:"password" vd:"required,len>5;msg:'密码必须大于5个字符'"`
	Role     models.RoleType `json:"role" vd:"$==1||$==2;msg:'角色只能是1(红人)或2(品牌方)'"`
	// 手机号和邮箱字段 (可选加入正则校验)
	Phone string `json:"phone"`
	Email string `json:"email"`
}

type LoginReq struct {
	Username   string `json:"username" vd:"required;msg:'用户名不能为空'"`
	Account    string `json:"account"  vd:"required;msg:'邮箱/手机号不能为空'"`
	Password   string `json:"password" vd:"required;msg:'密码不能为空'"`
	ClientType string `json:"client_type" vd:"$=='pc'||$=='mobile';msg:'客户端类型必须为 pc 或 mobile'"`
	// 新增：前端需要告诉后端，用户是从哪个角色的专属页面发起的登录
	Role int `json:"role" vd:"$==1||$==2;msg:'登录角色参数不合法(必须为1或2)'"`
}

// ResetPasswordReq 密码重置请求参数
type ResetPasswordReq struct {
	OldPassword string `json:"old_password" vd:"required;msg:'旧密码不能为空'"`
	NewPassword string `json:"new_password" vd:"required,len>5;msg:'新密码必须大于5个字符'"`
}

// 核心路由控制逻辑

// Register 账号注册接口
// @Summary 用户注册
// @Description 注册红人(role:1)或品牌方(role:2)账号
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterReq true "注册参数"
// @Success 200 {object} map[string]interface{} "{"code":0,"message":"success","data":{"message":"注册成功，请登录"}}"
// @Router /api/v1/auth/register [post]
func Register(c context.Context, ctx *app.RequestContext) {
	var req RegisterReq
	// 绑定 JSON 并执行 Validate 校验
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}

	_, err := service.RegisterService(c, req.Username, req.Password, req.Phone, req.Email, req.Role)
	if err != nil {
		// 标准的错误拦截与断言
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			response.Error(ctx, apiErr)
		} else {
			hlog.CtxErrorf(c, "Register 未知异常: %v", err)
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}

	response.Success(ctx, map[string]string{"message": "注册成功，请登录"})
}

// Login 账号登录接口 (颁发 Opaque Token)
// @Summary 用户登录
// @Description 验证账号密码并返回 Opaque Token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginReq true "登录参数"
// @Success 200 {object} map[string]interface{} "{"code":0,"message":"success","data":{...}}"
// @Router /api/v1/auth/login [post]
func Login(c context.Context, ctx *app.RequestContext) {
	var req LoginReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}
	// 1. 业务层互斥校验
	if req.Username == "" && req.Account == "" {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "请输入用户名或手机号/邮箱")
		return
	}
	// 提取最终用来登录的账号标识
	loginAccount := req.Account
	//if req.Username != "" {
	//	loginAccount = req.Username
	//}
	// 直接调用 Service 层！Handler 里再也见不到 SQL 代码了
	token, user, err := service.LoginService(c, req.Username, loginAccount, req.Password, req.ClientType, req.Role, ctx.ClientIP())
	if err != nil {
		// 类型断言 (Type Assertion)
		// 判断 Service 返回的 err 是不是我们自定义的 *response.APIError
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			// 如果是标准的业务错误，直接扔给前端
			response.Error(ctx, apiErr)
		} else {
			// 如果是未知的野生 error (比如空指针、下标越界等)，一律按 500 兜底处理
			hlog.CtxErrorf(c, "Login 未知异常: %v", err)
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}
	hlog.CtxInfof(c, "用户[%s]在[%s]端登录成功", user.Username, req.ClientType)
	// 3. 组装数据，成功返回
	response.Success(ctx, map[string]interface{}{
		"token":       token,
		"user_id":     user.ID,
		"role":        user.Role,
		"client_type": req.ClientType,
	})
}

// ResetPassword 密码修改接口 (受保护)
// @Summary 修改登录密码
// @Description 验证旧密码并设置新密码。成功后将强制清除该用户在所有设备上的登录状态（全端踢下线）。
// @Tags Auth
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body ResetPasswordReq true "修改密码参数"
// @Success 200 {object} map[string]interface{} "{"code":0,"message":"success","data":{"message":"密码修改成功，请重新登录"}}"
// @Router /api/v1/user/password/reset [post]
func ResetPassword(c context.Context, ctx *app.RequestContext) {
	// 1. 从上下文中提取 user_id (由 AuthMiddleware 保证一定存在)
	userIDAny, exists := ctx.Get("user_id")
	if !exists {
		response.Error(ctx, response.ErrUnauthorized)
		return
	}
	userID := userIDAny.(uint64)

	// 2. 参数绑定与校验
	var req ResetPasswordReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}

	err := service.ResetPasswordService(c, userID, req.OldPassword, req.NewPassword)

	if err != nil {
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			response.Error(ctx, apiErr)
		} else {
			hlog.CtxErrorf(c, "ResetPassword 未知异常: %v", err)
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}

	// 8. 返回成功提示
	response.Success(ctx, map[string]string{
		"message": "密码修改成功，请重新登录",
	})
}
