package handlers

import (
	"context"
	"errors"

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

// 核心路由控制逻辑

// Register 账号注册接口
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
