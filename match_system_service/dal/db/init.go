package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init() {
	// 1. 从环境变量获取数据库配置
	host := os.Getenv("MYSQL_HOST")
	port := os.Getenv("MYSQL_PORT")
	user := os.Getenv("MYSQL_USER")
	password := os.Getenv("MYSQL_PASSWORD")
	dbName := os.Getenv("MYSQL_MS_DBNAME") // 匹配系统专属库

	// 如果关键环境变量缺失，快速失败
	if host == "" || user == "" || dbName == "" {
		log.Fatal("致命错误: MySQL 环境变量缺失，请检查 .env 文件")
	}

	// 2. 拼接 DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbName)

	// 3. 配置 GORM 日志级别
	logLevel := logger.Info
	if os.Getenv("APP_ENV") == "production" {
		logLevel = logger.Error
	}

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatalf("MySQL 连接失败: %v", err)
	}

	// 4. 获取底层的 sql.DB 以配置连接池（应对高并发匹配查询）
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("获取底层 SQL DB 失败: %v", err)
	}

	sqlDB.SetMaxIdleConns(10)           // 空闲连接池中连接的最大数量
	sqlDB.SetMaxOpenConns(100)          // 数据库连接的最大数量
	sqlDB.SetConnMaxLifetime(time.Hour) // 连接可复用的最大时间

	log.Println("✅ MySQL 连接初始化成功 (已配置连接池)")
}
