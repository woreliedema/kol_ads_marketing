package service

import (
	"context"
	"errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/models"
	"kol_ads_marketing/user_center/app/rpc"
	"time"
)

// SubmitUGCBindTask 提交绑定任务 (同步解析 -> 入库 -> 异步调度)
func SubmitUGCBindTask(ctx context.Context, userID uint64, platform, spaceURL string) (bool, error) {
	// Step 1: 同步快失败，提取真实 UID
	platformUID, err := rpc.ParseProfileURL(ctx, platform, spaceURL)
	if err != nil {
		// 解析失败直接抛出 400 业务错误，前端弹窗提示
		return false, &response.APIError{HTTPCode: 400, BizCode: 400010, Message: err.Error()}
	}

	//  Step 2: 走内部接口查询热/温数据
	isFresh, freshData, err := rpc.CheckProfileFreshness(ctx, platform, platformUID)

	if err != nil {
		// 探测接口挂了，可以选择熔断报错，也可以选择降级为直接去异步爬取。这里选择保守报错。
		return false, &response.APIError{HTTPCode: 500, BizCode: 500020, Message: "用户画像探测服务异常"}
	}

	ugcAccount := models.UserUGCAccount{
		UserID:           userID,
		Platform:         platform,
		PlatformUID:      platformUID, // 填入极其纯洁的真实 UID
		PlatformSpaceURL: spaceURL,
		UpdateAt:         time.Now(),
	}

	// 业务分流：根据鲜活度决定状态和数据
	if isFresh {
		ugcAccount.AuthStatus = 1 // 直接认定为认证成功！
		// 尝试从 Python 返回的字典里提取昵称和粉丝数存入 MySQL (类型断言注意安全保护)
		if nickname, ok := freshData["nickname"].(string); ok {
			ugcAccount.Nickname = nickname
		}
		if followers, ok := freshData["followers_count"].(float64); ok { // JSON 数字默认解析为 float64
			ugcAccount.FansCount = int64(followers)
		}
	} else {
		ugcAccount.AuthStatus = 0 // 置为采集中状态
	}

	// 容错处理：使用 Upsert，如果该用户已经绑过该平台，覆盖重置状态
	dbErr := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "platform"}},
		DoUpdates: clause.AssignmentColumns([]string{"platform_uid", "platform_space_url", "auth_status", "nickname", "fans_count", "update_at"}),
	}).Create(&ugcAccount).Error

	if dbErr != nil {
		var mysqlErr *mysql.MySQLError
		// 如果发现唯一索引冲突 (1062) 且冲突的不是 (user_id, platform)，
		// 说明这个 platform_uid 已经被别的用户绑定过了！(防盗号防刷)
		if errors.As(dbErr, &mysqlErr) && mysqlErr.Number == 1062 {
			return false, &response.APIError{HTTPCode: 400, BizCode: 400011, Message: "该平台账号已被其他用户绑定！"}
		}

		hlog.CtxErrorf(ctx, "初始化 UGC 绑定记录失败: %v", dbErr)
		return false, response.ErrDatabaseError
	}

	// 异步触发 Python 任务总表调度
	if !isFresh {
		// 如果没有鲜活数据，才会触发极其昂贵的 Kafka 爬虫调度
		rpc.RegisterCrawlerTarget(ctx, platform, platformUID)
	}
	// 返回成功给 Handler，Handler 接着给前端返回 "绑定成功，数据同步中"
	return isFresh, nil
}

// GetUGCBindService 面向前端的同步 UGC 数据服务
func GetUGCBindService(ctx context.Context, userID uint64, platform string) (*models.UserUGCAccount, error) {
	// 1. 先从 MySQL 查出用户当前的绑定记录，获取 platform_uid
	var account models.UserUGCAccount
	err := db.DB.Where("user_id = ? AND platform = ?", userID, platform).First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.APIError{HTTPCode: 404, BizCode: 404010, Message: "未找到该平台的绑定记录"}
		}
		return nil, response.ErrDatabaseError
	}

	// 如果根本还没解析出 UID (极小概率，比如正在解析中)，直接返回当前状态
	if account.PlatformUID == "" {
		return &account, nil
	}

	// 2. 用户中心代前端去调用内部的 Python 探活接口！
	isFresh, freshData, err := rpc.CheckProfileFreshness(ctx, platform, account.PlatformUID)
	if err != nil {
		hlog.CtxErrorf(ctx, "向数据服务同步数据失败: %v", err)
		// 内部服务异常时，降级处理：直接返回 MySQL 里的老数据，不报错！
		return &account, nil
	}

	// 3. 如果 Python 说数据是热乎的，更新 Go 自己的账本 (MySQL)
	if isFresh && freshData != nil {
		needsUpdate := false
		updateMap := make(map[string]interface{})

		// 提取最新数据并比对，只有变化了才去写数据库 (节省 DB 压力)
		if nickname, ok := freshData["nickname"].(string); ok && nickname != account.Nickname {
			updateMap["nickname"] = nickname
			account.Nickname = nickname
			needsUpdate = true
		}
		if fansFloat, ok := freshData["followers_count"].(float64); ok {
			fansInt := int64(fansFloat)
			if fansInt != account.FansCount {
				updateMap["fans_count"] = fansInt
				account.FansCount = fansInt
				needsUpdate = true
			}
		}
		// 状态机：只有当现在的状态是“采集中(0)”时，才允许翻转状态。
		// 如果状态已经是“绑定成功(1)”甚至是被后台管理的“封禁(-1)”，绝对不碰它
		if account.AuthStatus == 0 {
			updateMap["auth_status"] = 1
			account.AuthStatus = 1
			needsUpdate = true
		}

		if needsUpdate {
			// GORM 的机制：
			// 当我们执行 Updates() 传入 Map 时，如果在模型里配置了 autoUpdateTime
			// GORM 会自动帮我们把 update_at 设置为当前时间，且绝对不会碰 bound_at
			db.DB.Model(&account).Updates(updateMap)
		}
	}

	// 4. 把最终的 (可能是更新后的) 数据返回给 Handler
	return &account, nil
}
