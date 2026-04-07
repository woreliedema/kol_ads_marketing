package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

var (
	privateKey   *rsa.PrivateKey
	publicKeyPEM string
)

// InitRSAKeys 服务启动时调用，生成一对 2048 位的 RSA 秘钥
func InitRSAKeys() {
	var err error
	privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		hlog.Fatalf("初始化 RSA 私钥失败: %v", err)
	}

	// 导出公钥为 PEM 格式，方便前端直接使用 JSEncrypt 等库
	pubASN1, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		hlog.Fatalf("导出 RSA 公钥失败: %v", err)
	}

	publicKeyPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	}))

	hlog.Info("[Security] 内存 RSA 秘钥对生成完毕，防重放防线已开启！")
}

// GetPublicKeyPEM 获取公钥字符串
func GetPublicKeyPEM() string {
	return publicKeyPEM
}

// DecryptAndValidatePassword 解密前端传来的密文，并校验时间戳
func DecryptAndValidatePassword(cipherBase64 string) (string, error) {
	// 1. Base64 解码
	cipherBytes, err := base64.StdEncoding.DecodeString(cipherBase64)
	if err != nil {
		return "", errors.New("密码格式非法")
	}

	// 2. RSA 私钥解密 (采用 PKCS1v15 填充方案，前端最常用的标准)
	plaintextBytes, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, cipherBytes)
	if err != nil {
		return "", errors.New("密码解密失败，可能是公钥已过期，请刷新页面重试")
	}

	// 3. 按照约定切割字符串： "真密码|1679000000000"
	plaintext := string(plaintextBytes)
	parts := strings.Split(plaintext, "|")
	if len(parts) != 2 {
		return "", errors.New("非法请求：安全校验失败")
	}

	password := parts[0]
	timestampStr := parts[1]

	// 4. 校验时间戳防重放攻击 (前端传的通常是毫秒级时间戳)
	clientTimestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return "", errors.New("非法请求：时间戳篡改")
	}

	// 将毫秒转为秒进行比对
	clientSeconds := clientTimestamp / 1000
	currentSeconds := time.Now().Unix()

	// 计算时间差的绝对值（允许前端时钟有一点点快或慢，但不能超过 60 秒）
	diff := currentSeconds - clientSeconds
	if diff < -60 || diff > 60 {
		hlog.Warnf("[Security] 拦截到疑似重放攻击！前端时间戳: %d, 服务器时间戳: %d", clientSeconds, currentSeconds)
		return "", errors.New("请求已过期，请重新提交")
	}

	// 校验通过，返回极其纯洁的真实密码！
	return password, nil
}
