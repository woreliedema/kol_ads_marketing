package db

import (
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"kol_ads_marketing/data_monitor_service/pkg/utils"
)

var DB *gorm.DB

// InitClickHouseClient 无参初始化，配置内聚
func InitClickHouseClient() {
	host := utils.GetEnv("CLICKHOUSE_HOST", "127.0.0.1")
	port := utils.GetEnvInt("CLICKHOUSE_PORT", 9000)
	dbName := utils.GetEnv("CLICKHOUSE_DB_DWS", "dws")
	user := utils.GetEnv("CLICKHOUSE_USER", "admin")
	pass := utils.GetEnv("CLICKHOUSE_PASSWORD", "")

	dsn := fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s", user, pass, host, port, dbName)

	db, err := gorm.Open(clickhouse.Open(dsn), &gorm.Config{})
	if err != nil {
		hlog.Fatalf("❌ ClickHouse 连接失败: %v", err) // 启动时 Fail-Fast
	}

	sqlDB, err := db.DB()
	if err != nil {
		hlog.Fatalf("❌ 获取 ClickHouse 底层连接失败: %v", err)
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB = db
	hlog.Infof("✅ ClickHouse 连接池初始化成功 [Addr: %s:%d]", host, port)
}
