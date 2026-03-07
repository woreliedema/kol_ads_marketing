package handlers

import (
	"context"
	"errors"
	"fmt"

	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/models"
	"kol_ads_marketing/user_center/app/utils"
	"kol_ads_marketing/user_center/app/utils/auth"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

// 数据传输对象 (DTO) 定义

type RegisterReq struct {
	Username string          `json:"username" vd:"required,len>4;msg:'用户名必须大于4个字符'"`
	Password string          `json:"password" vd:"required,len>5;msg:'密码必须大于5个字符'"`
	Role     models.RoleType `json:"role" vd:"$==1||$==2;msg:'角色只能是1(红人)或2(品牌方)'"`
}

type LoginReq struct {
	Username   string `json:"username" vd:"required;msg:'用户名不能为空'"`
	Password   string `json:"password" vd:"required;msg:'密码不能为空'"`
	ClientType string `json:"client_type" vd:"$=='pc'||$=='mobile';msg:'客户端类型必须为 pc 或 mobile'"`
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

	// 1. 检查用户是否已存在
	var count int64
	db.DB.Model(&models.SysUser{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "该用户名已被注册")
		return
	}

	// 2. 密码加密 (合规性底线)
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		hlog.CtxErrorf(c, "密码加密失败: %v", err)
		response.Error(ctx, response.ErrSystemError)
		return
	}

	// 3. 构建用户实体，开启数据库事务写入核心表和扩展表
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		newUser := models.SysUser{
			Username:     req.Username,
			PasswordHash: hashedPassword,
			Role:         req.Role,
			Status:       1,
		}

		if err := tx.Create(&newUser).Error; err != nil {
			return err
		}

		// 根据角色初始化对应的业务空白扩展表
		if req.Role == models.RoleKOL {
			if err := tx.Create(&models.KOLProfile{
				UserID: newUser.ID,
				Tags:   "[]",
			}).Error; err != nil {
				return err
			}
		} else if req.Role == models.RoleBrand {
			if err := tx.Create(&models.BrandProfile{UserID: newUser.ID, CompanyName: "未命名企业"}).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		hlog.CtxErrorf(c, "数据库事务写入失败: %v", err)
		response.Error(ctx, response.ErrDatabaseError)
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

	// 1. 查询数据库核对账号
	var user models.SysUser
	err := db.DB.Where("username = ?", req.Username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(ctx, response.ErrUserNotFound)
		} else {
			response.Error(ctx, response.ErrDatabaseError)
		}
		return
	}

	// 2. 校验账号状态
	if user.Status != 1 {
		response.ErrorWithMsg(ctx, response.ErrPermission, "该账号已被封禁或未激活")
		return
	}

	// 3. bcrypt 哈希比对
	if !utils.CheckPasswordHash(req.Password, user.PasswordHash) {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "密码错误")
		return
	}

	// 4. 更新最后登录 IP
	db.DB.Model(&user).Update("last_login_ip", ctx.ClientIP())

	// 5. 调用工具类生成 Token，并执行 Redis 状态同步 (核心状态引擎介入)
	token, err := auth.GenerateAndSaveToken(c, user.ID, user.Role, req.ClientType)
	if err != nil {
		hlog.CtxErrorf(c, "Token 生成失败: %v", err)
		response.Error(ctx, response.ErrSystemError)
		return
	}

	hlog.CtxInfof(c, "用户[%s]在端[%s]登录成功", req.Username, req.ClientType)

	// 6. 返回结果给前端
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

	// 不能新旧密码一样
	if req.OldPassword == req.NewPassword {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "新密码不能与旧密码相同")
		return
	}

	// 3. 查库获取当前用户信息
	var user models.SysUser
	if err := db.DB.First(&user, userID).Error; err != nil {
		response.Error(ctx, response.ErrUserNotFound)
		return
	}

	// 4. 校验旧密码是否正确
	if !utils.CheckPasswordHash(req.OldPassword, user.PasswordHash) {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "旧密码错误")
		return
	}

	// 5. 对新密码进行 bcrypt 加密
	hashedPassword, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		hlog.CtxErrorf(c, "新密码加密失败: %v", err)
		response.Error(ctx, response.ErrSystemError)
		return
	}

	// 6. 更新数据库中的密码
	if err := db.DB.Model(&user).Update("password_hash", hashedPassword).Error; err != nil {
		hlog.CtxErrorf(c, "数据库更新密码失败: %v", err)
		response.Error(ctx, response.ErrDatabaseError)
		return
	}

	// 核心状态引擎：全端踢人逻辑 (强制下线)
	userHashKey := fmt.Sprintf("auth:user:%d", userID)

	// 7.1 从 Redis 获取该用户在所有端 (pc, mobile) 的当前有效 Token 列表
	tokensMap, err := db.RDB.HGetAll(c, userHashKey).Result()
	if err == nil && len(tokensMap) > 0 {
		// 7.2 开启 Redis 管道批量删除，极致性能
		pipe := db.RDB.TxPipeline()

		// 遍历抹除所有的正向映射 (让拿着旧 Token 正在请求的人瞬间报 401)
		for _, token := range tokensMap {
			tokenKey := fmt.Sprintf("auth:token:%s", token)
			pipe.Del(c, tokenKey)
		}

		// 抹除反向映射表本身
		pipe.Del(c, userHashKey)

		// 执行管道清理操作
		if _, err := pipe.Exec(c); err != nil {
			hlog.CtxErrorf(c, "Redis 清除用户 Token 失败: %v", err)
			// 这里即使 Redis 报错也不阻断返回，因为数据库已经改了，旧 Token 早晚会失效
		} else {
			hlog.CtxInfof(c, "用户 [%d] 密码修改成功，已强制清除 %d 个终端的登录状态", userID, len(tokensMap))
		}
	}

	// 8. 返回成功提示
	response.Success(ctx, map[string]string{
		"message": "密码修改成功，请重新登录",
	})
}
