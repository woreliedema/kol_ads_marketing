package db

import (
	"gorm.io/gorm"
	"time"
)

// SyncBrandToWideIndex 品牌方数据同步到宽表
func SyncBrandToWideIndex(db *gorm.DB, lastSyncTime time.Time) error {
	sql := `
        INSERT INTO kol_match.match_brand_wide_index (
            brand_user_id, status, username, company_name, 
            brand_avatar_url, tags, is_verified, source_updated_at, sync_status
        )
        SELECT 
            u.id, u.status, u.username, b.company_name, 
            b.avatar_url as brand_avatar_url, b.tags, b.is_verified,
            GREATEST(IFNULL(u.updated_at, '1970-01-01'), IFNULL(b.updated_at, '1970-01-01')),
            0 -- 核心：只要有更新，sync_status 就重置为 0，等待被推送到 ES
        FROM kol_user_center.sys_users u
        JOIN kol_user_center.brand_profiles b ON u.id = b.user_id
        WHERE u.updated_at > ? OR b.updated_at > ?
        ON DUPLICATE KEY UPDATE 
            status = VALUES(status),
            username = VALUES(username),
            company_name = VALUES(company_name),
            brand_avatar_url = VALUES(brand_avatar_url),
            tags = VALUES(tags),
            is_verified = VALUES(is_verified),
            source_updated_at = VALUES(source_updated_at),
            sync_status = 0; -- 触发更新后，重新标记为待同步
    `
	// 执行原生 SQL
	return db.Exec(sql, lastSyncTime, lastSyncTime).Error
}

// SyncKOLToWideIndex KOL数据同步到宽表
func SyncKOLToWideIndex(db *gorm.DB, lastSyncTime time.Time) error {
	sql := `
        INSERT INTO kol_match.match_kol_wide_index (
            kol_user_id, username, status, kol_avatar_url, tags, base_quote, 
            ugc_platforms, total_followers, ugc_accounts_detail, source_updated_at, sync_status
        )
        SELECT 
            u.id, u.username, u.status, k.avatar_url as kol_avatar_url, k.tags, k.base_quote,
            JSON_ARRAYAGG(a.platform) AS ugc_platfrom,
            COALESCE(SUM(a.fans_count), 0),
            IF(COUNT(a.id) > 0, JSON_ARRAYAGG(JSON_OBJECT('platform', a.platform,'platform_uid', a.platform_uid, 'followers', a.fans_count)), '[]'),
            GREATEST(IFNULL(u.updated_at, '1970-01-01'), IFNULL(k.updated_at, '1970-01-01'), IFNULL(MAX(a.update_at), '1970-01-01')),
            0
        FROM kol_user_center.sys_users u
        JOIN kol_user_center.kol_profiles k ON u.id = k.user_id
        LEFT JOIN kol_user_center.user_ugc_accounts a ON u.id = a.user_id  
        WHERE u.status = 1
            and u.updated_at > ? OR k.updated_at > ? OR u.id IN (SELECT user_id FROM kol_user_center.user_ugc_accounts WHERE update_at > ?)
        GROUP BY u.id, u.username, u.status, kol_avatar_url, k.tags, k.base_quote
        ON DUPLICATE KEY UPDATE 
            username = VALUES(username),
            status = VALUES(status),
            kol_avatar_url = VALUES(kol_avatar_url),
            tags = VALUES(tags),
            base_quote = VALUES(base_quote),
            ugc_platforms = VALUES(ugc_platforms),
            total_followers = VALUES(total_followers),
            ugc_accounts_detail = VALUES(ugc_accounts_detail),
            source_updated_at = VALUES(source_updated_at),
            sync_status = 0;
    `
	return db.Exec(sql, lastSyncTime, lastSyncTime, lastSyncTime).Error
}
