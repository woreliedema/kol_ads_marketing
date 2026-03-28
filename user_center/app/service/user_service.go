package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kol_ads_marketing/user_center/app/utils"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/models"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

// GetUserProfileService 处理获取用户全量资料的业务逻辑 (支持跨层和内部调用复用)
func GetUserProfileService(ctx context.Context, userID uint64) (map[string]interface{}, error) {
	// 1. 查询基础表获取角色 (顺手加上 phone 和 email)
	var user models.SysUser
	var profile interface{}
	var ugcAccounts []models.UserUGCAccount

	var eg errgroup.Group

	// 1：查询主表基础信息
	eg.Go(func() error {
		err := db.DB.Select("id", "username", "phone", "email", "role", "status", "created_at").First(&user, userID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.ErrUserNotFound
		}
		return err
	})
	// 2：查询角色专属扩展资料表
	eg.Go(func() error {
		// 先去查一下主表确认角色（或者直接用传进来的role，但为了内外部通用，这里再查一次角色是安全的，耗时极短）
		var tempUser models.SysUser
		if err := db.DB.Select("role").First(&tempUser, userID).Error; err != nil {
			return err
		}
		if tempUser.Role == models.RoleKOL {
			var kol models.KOLProfile
			err := db.DB.Where("user_id = ?", userID).First(&kol).Error
			profile = kol
			return err
		} else if tempUser.Role == models.RoleBrand {
			var brand models.BrandProfile
			err := db.DB.Where("user_id = ?", userID).First(&brand).Error
			profile = brand
			return err
		}
		return nil
	})

	// 3：查询绑定的第三方 UGC 账号列表
	eg.Go(func() error {
		return db.DB.Where("user_id = ?", userID).Find(&ugcAccounts).Error
	})

	// 屏障等待
	if err := eg.Wait(); err != nil {
		hlog.CtxErrorf(ctx, "并发聚合用户信息失败: %v", err)
		// 如果是没找到用户，直接返回标准错误
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			return nil, apiErr
		}
		return nil, response.ErrDatabaseError
	}

	// 组装聚合后的超级返回体
	return map[string]interface{}{
		"base_info":    user,
		"profile":      profile,
		"ugc_accounts": ugcAccounts,
	}, nil
}

// UpdateKOLProfileService 更新红人资料
func UpdateKOLProfileService(ctx context.Context, userID uint64, realName string, baseQuote int) error {
	updateData := map[string]interface{}{
		"real_name": realName,
		//"tags":       tags,
		"base_quote": baseQuote,
	}

	if err := db.DB.Model(&models.KOLProfile{}).Where("user_id = ?", userID).Updates(updateData).Error; err != nil {
		hlog.CtxErrorf(ctx, "更新 KOL 资料失败: %v", err)
		return response.ErrDatabaseError
	}
	return nil
}

// UpdateBrandProfileService 更新品牌方资料
func UpdateBrandProfileService(ctx context.Context, userID uint64, companyName string) error {
	updateData := map[string]interface{}{
		"company_name": companyName,
		//"industry":     industry,
	}

	if err := db.DB.Model(&models.BrandProfile{}).Where("user_id = ?", userID).Updates(updateData).Error; err != nil {
		hlog.CtxErrorf(ctx, "更新品牌方资料失败: %v", err)
		return response.ErrDatabaseError
	}
	return nil
}

// CheckAvatarCooldown 检查用户是否处于 7 天修改头像的冷却期内
func CheckAvatarCooldown(ctx context.Context, userID uint64) error {
	redisKey := fmt.Sprintf("avatar_cd:%d", userID)

	// 查看 Redis 中是否存在该 Key
	exists, err := db.RDB.Exists(ctx, redisKey).Result()
	if err != nil {
		hlog.CtxErrorf(ctx, "检查头像冷却期 Redis 失败: %v", err)
		return response.ErrSystemError
	}

	if exists > 0 {
		// 动态获取剩余时间给前端更好的体验
		ttl, _ := db.RDB.TTL(ctx, redisKey).Result()
		days := int(ttl.Hours() / 24)
		hours := int(ttl.Hours()) % 24

		errMsg := fmt.Sprintf("修改太频繁啦！还需等待 %d天%d小时 才能再次修改头像", days, hours)
		return &response.APIError{HTTPCode: 403, BizCode: 403004, Message: errMsg}
	}

	return nil
}

// UpdateUserAvatar 更新数据库中的头像 URL，并加上 7 天冷却锁
func UpdateUserAvatar(ctx context.Context, userID uint64, role models.RoleType, avatarURL string) error {
	// 1. 根据角色更新对应的扩展表
	var err error
	if role == models.RoleKOL {
		err = db.DB.Model(&models.KOLProfile{}).Where("user_id = ?", userID).Update("avatar_url", avatarURL).Error
	} else if role == models.RoleBrand {
		err = db.DB.Model(&models.BrandProfile{}).Where("user_id = ?", userID).Update("avatar_url", avatarURL).Error
	} else {
		return response.ErrInvalidParams
	}

	if err != nil {
		hlog.CtxErrorf(ctx, "更新头像数据库失败: %v", err)
		return response.ErrDatabaseError
	}

	// 2. 数据库更新成功后，写入 Redis 7 天冷却锁
	redisKey := fmt.Sprintf("avatar_cd:%d", userID)
	// 7 * 24 小时 = 168 小时
	if err := db.RDB.Set(ctx, redisKey, avatarURL, 7*24*time.Hour).Err(); err != nil {
		hlog.CtxErrorf(ctx, "写入头像冷却 Redis 锁失败: %v", err)
		// 即使 Redis 写入失败，也不阻断前端返回，因为头像确实已经改成功了
	}

	return nil
}

// UpdateBrandLicenseService 更新品牌方的营业执照URL
func UpdateBrandLicenseService(ctx context.Context, userID uint64, licenseURL string) error {
	// 直接指定更新 license_url 字段
	if err := db.DB.Model(&models.BrandProfile{}).Where("user_id = ?", userID).Update("license_url", licenseURL).Error; err != nil {
		hlog.CtxErrorf(ctx, "更新品牌方营业执照失败: %v", err)
		return response.ErrDatabaseError
	}
	return nil
}

// DeleteBrandLicenseService 验证密码、物理删除文件并清空数据库记录
func DeleteBrandLicenseService(ctx context.Context, userID uint64, password string) error {
	// 1. 查出用户基础表，校验密码
	var user models.SysUser
	if err := db.DB.First(&user, userID).Error; err != nil {
		return response.ErrUserNotFound
	}

	// 复用我们之前封装好的密码校验工具
	if !utils.CheckPasswordHash(password, user.PasswordHash) {
		// 动态抛出业务错误
		return &response.APIError{HTTPCode: 401, BizCode: 401002, Message: "安全校验失败：登录密码错误"}
	}

	// 2. 查出当前品牌方资料，获取现有的 license_url
	var profile models.BrandProfile
	if err := db.DB.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		hlog.CtxErrorf(ctx, "查询品牌方资料失败: %v", err)
		return response.ErrDatabaseError
	}

	// 3. 核心动作：物理删除本地文件
	if profile.LicenseURL != "" {
		// 我们之前存的 URL 是 "/uploads/licenses/xxxx.jpg"
		// 为了找到本地物理路径，需要去掉开头的 "/"，并在前面加上 "./"
		relativePath := strings.TrimPrefix(profile.LicenseURL, "/")
		physicalPath := filepath.Join(".", relativePath)

		// 调用 os.Remove 物理删除硬盘上的文件
		if err := os.Remove(physicalPath); err != nil {
			// 如果错误是“文件本来就不存在”，可以安全忽略；否则打印日志
			if !os.IsNotExist(err) {
				hlog.CtxErrorf(ctx, "物理删除营业执照文件失败 [%s]: %v", physicalPath, err)
				// 此时可以选择 return 报错阻断流程，也可以继续执行把数据库清空。这里选择容错继续。
			}
		} else {
			hlog.CtxInfof(ctx, "成功物理粉碎营业执照文件: %s", physicalPath)
		}
	}

	// 4. 将 license_url 更新为空字符串
	if err := db.DB.Model(&models.BrandProfile{}).Where("user_id = ?", userID).Update("license_url", "").Error; err != nil {
		hlog.CtxErrorf(ctx, "销毁品牌方营业执照数据库记录失败: %v", err)
		return response.ErrDatabaseError
	}

	return nil
}

// UpdateUserTagsService 统一的标签更新服务
func UpdateUserTagsService(ctx context.Context, userID uint64, role models.RoleType, tags []string) error {
	// 防御性编程：如果用户选择跳过，设为默认值
	if len(tags) == 0 {
		tags = []string{"未分类"} // 给个默认数组
	}

	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return err
	}

	// 3. 根据角色定向落库
	if role == models.RoleKOL {
		return db.DB.Model(&models.KOLProfile{}).
			Where("user_id = ?", userID).
			Update("tags", string(tagsJSON)).Error

	} else if role == models.RoleBrand {
		// 品牌方也直接更新 tags 字段，写入 JSON 数据！
		return db.DB.Model(&models.BrandProfile{}).
			Where("user_id = ?", userID).
			Update("tags", string(tagsJSON)).Error
	}

	return nil
}
