package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/cloudwego/hertz/pkg/common/hlog"

	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/models"
	"kol_ads_marketing/user_center/app/utils"
	"kol_ads_marketing/user_center/app/utils/auth"

	"gorm.io/gorm"
)

// LoginService 处理登录核心业务逻辑
// 接收纯粹的参数，返回生成的 Token 和用户信息，不关心 HTTP 请求怎么来的
func LoginService(ctx context.Context, username, account, password, clientType string, expectedRole int, clientIP string) (string, *models.SysUser, error) {
	var user models.SysUser

	// 1. 动态查询数据库
	query := db.DB.Model(&models.SysUser{})
	// 无论前端传的是什么，统统去匹配这三个字段
	err := query.Where("username = ? OR phone = ? OR email = ?", username, account, account).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, response.ErrUserNotFound
		}
		hlog.CtxErrorf(ctx, "登录查询数据库失败: %v", err)
		return "", nil, response.ErrDatabaseError
	}

	if int(user.Role) != expectedRole {
		hlog.CtxWarnf(ctx, "账号越权登录拦截: 用户[%s] 真实角色[%d] 尝试从入口[%d] 登录", user.Username, user.Role, expectedRole)

		// 动态生成友好的提示语
		errMsg := "您是品牌方，请前往品牌方专属入口登录"
		if expectedRole == 2 {
			errMsg = "您是红人，请前往红人专属入口登录"
		}

		return "", nil, &response.APIError{HTTPCode: 403, BizCode: 403003, Message: errMsg}
	}

	// 2. 校验账号状态
	if user.Status != 1 {
		return "", nil, response.ErrUserBanned
	}

	// 3. bcrypt 哈希比对
	if !utils.CheckPasswordHash(password, user.PasswordHash) {
		return "", nil, response.ErrInvalidPassword
	}

	// 4. 更新最后登录 IP (甚至可以用协程异步去更新，不阻塞登录流程)
	//db.DB.Model(&user).Update("last_login_ip", clientIP)
	go db.DB.Model(&user).Update("last_login_ip", clientIP)

	// 5. 生成 Token 并存入 Redis
	token, err := auth.GenerateAndSaveToken(ctx, user.ID, user.Role, clientType)
	if err != nil {
		hlog.CtxErrorf(ctx, "Token 生成失败: %v", err)
		return "", nil, response.ErrSystemError
	}

	// 返回 token 和用户信息
	return token, &user, nil
}

// RegisterService 处理注册核心业务逻辑
func RegisterService(ctx context.Context, username, password, phone, email string, role models.RoleType) (uint64, error) {
	// 1. 多字段全局防重校验
	var count int64
	query := db.DB.Model(&models.SysUser{}).Where("username = ?", username)
	if phone != "" {
		query = query.Or("phone = ?", phone)
	}
	if email != "" {
		query = query.Or("email = ?", email)
	}

	query.Count(&count)
	if count > 0 {
		// 抛出标准的业务错误
		return 0, response.ErrUserAlreadyExists
	}

	// 2. 密码加密
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		hlog.CtxErrorf(ctx, "密码加密失败: %v", err)
		return 0, response.ErrSystemError
	}

	var userID uint64

	// 3. 开启数据库事务写入
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		var phonePtr *string
		if phone != "" {
			phonePtr = &phone
		}

		var emailPtr *string
		if email != "" {
			emailPtr = &email
		}

		newUser := models.SysUser{
			Username:     username,
			PasswordHash: hashedPassword,
			Role:         role,
			Status:       1,
			Phone:        phonePtr,
			Email:        emailPtr,
		}

		if err := tx.Create(&newUser).Error; err != nil {
			return err
		}

		userID = newUser.ID // 记录新生成的 ID

		// 初始化对应的业务空白扩展表
		if role == models.RoleKOL {
			if err := tx.Create(&models.KOLProfile{UserID: userID, Tags: "[]"}).Error; err != nil {
				return err
			}
		} else if role == models.RoleBrand {
			if err := tx.Create(&models.BrandProfile{UserID: userID, CompanyName: "未命名企业"}).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		hlog.CtxErrorf(ctx, "注册数据库事务写入失败: %v", err)
		return 0, response.ErrDatabaseError
	}

	return userID, nil
}

// ResetPasswordService 处理密码重置及全端踢出逻辑
func ResetPasswordService(ctx context.Context, userID uint64, oldPassword, newPassword string) error {
	var user models.SysUser

	// 1. 查库获取当前用户信息
	if err := db.DB.First(&user, userID).Error; err != nil {
		return response.ErrUserNotFound
	}

	// 2. 校验旧密码是否正确
	if !utils.CheckPasswordHash(oldPassword, user.PasswordHash) {
		// 这里临时复用 ErrInvalidParams，并动态修改提示文字，保证返回标准 APIError
		return &response.APIError{HTTPCode: 400, BizCode: 400005, Message: "旧密码错误"}
	}

	// 3. 对新密码进行 bcrypt 加密
	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		hlog.CtxErrorf(ctx, "新密码加密失败: %v", err)
		return response.ErrSystemError
	}

	// 4. 更新数据库中的密码
	if err := db.DB.Model(&user).Update("password_hash", hashedPassword).Error; err != nil {
		hlog.CtxErrorf(ctx, "数据库更新密码失败: %v", err)
		return response.ErrDatabaseError
	}

	// 5. 核心状态引擎：全端踢人逻辑
	userHashKey := fmt.Sprintf("auth:user:%d", userID)
	tokensMap, err := db.RDB.HGetAll(ctx, userHashKey).Result()
	if err == nil && len(tokensMap) > 0 {
		pipe := db.RDB.TxPipeline()
		for _, token := range tokensMap {
			tokenKey := fmt.Sprintf("auth:token:%s", token)
			pipe.Del(ctx, tokenKey)
		}
		pipe.Del(ctx, userHashKey)

		if _, err := pipe.Exec(ctx); err != nil {
			hlog.CtxErrorf(ctx, "Redis 清除用户 Token 失败: %v", err)
		} else {
			hlog.CtxInfof(ctx, "用户 [%d] 密码修改成功，已强制清除 %d 个终端的登录状态", userID, len(tokensMap))
		}
	}

	return nil
}
