package db

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 1. 配置边界 (Configuration Boundary)
// 将 DB 配置抽象为结构体。
// 无论本地硬编码，还是从 Nacos/YAML 动态加载，都只需要实例化这个结构体即可。

type MySQLConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	DBName       string
	Charset      string
	MaxIdleConns int           // 最大空闲连接数
	MaxOpenConns int           // 最大打开连接数
	MaxLifetime  time.Duration // 连接最大存活时间
	Debug        bool          // 是否开启 SQL 打印
}

// BuildDSN 动态构建 DSN 字符串，隐藏拼接细节
func (cfg *MySQLConfig) BuildDSN() string {
	if cfg.Charset == "" {
		cfg.Charset = "utf8mb4"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.Charset,
	)
}

// 全局 DB 实例 (可导出供 Service 层调用，或者后续封装 Repository 层)

var DB *gorm.DB

// 2. 连接边界 (Connection Boundary)
// 纯粹负责实例化 GORM，不包含任何业务表结构迁移的逻辑

// InitMySQL 初始化数据库连接，返回 error 交由上层(main.go 的 lifespan)决定是否 panic
func InitMySQL(cfg *MySQLConfig) error {
	dsn := cfg.BuildDSN()

	// 动态配置 GORM 日志级别
	gormLogLevel := logger.Silent
	if cfg.Debug {
		gormLogLevel = logger.Info
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	}

	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return fmt.Errorf("gorm.Open 失败: %w", err)
	}

	// 获取底层的 sql.DB 对象进行连接池调优
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取底层 sql.DB 失败: %w", err)
	}

	// 注入连接池配置
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.MaxLifetime)
	}

	// 赋值给全局变量
	DB = db
	log.Println("✅ MySQL 底层连接池初始化成功！")
	return nil
}
