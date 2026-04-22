package utils

import (
	"os"
	"strconv"
)

// GetEnv 获取字符串类型的环境变量，带默认值
func GetEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

// GetEnvInt 获取整型环境变量，带默认值
func GetEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if v, err := strconv.Atoi(val); err == nil {
			return v
		}
	}
	return fallback
}
