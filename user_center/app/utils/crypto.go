package utils

import "golang.org/x/crypto/bcrypt"

// HashPassword 使用 bcrypt 对密码进行单向哈希加密
func HashPassword(password string) (string, error) {
	// DefaultCost 是 10，兼顾了安全性和性能
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash 校验用户输入的明文密码与数据库中的哈希值是否匹配
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
