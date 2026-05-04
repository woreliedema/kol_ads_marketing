package core

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinioClient *minio.Client

// BucketName 全局统一的 Bucket 名称
// 设置为 uploads，这样后续拼接出的路径天然就是 /uploads/...，无缝兼容老数据
const BucketName = "uploads"

// InitMinio 初始化对象存储客户端
func InitMinio() {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	useSSL := os.Getenv("MINIO_SECURE") == "True"

	// 1. 初始化 MinIO 客户端
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		hlog.Fatalf("❌ 初始化 MinIO 客户端失败: %v", err)
	}
	MinioClient = client

	// 2. 检查 Bucket 是否存在
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, BucketName)
	if err != nil {
		hlog.Fatalf("❌ 检查 MinIO Bucket 失败: %v", err)
	}

	// 3. 自动创建与策略赋予
	if !exists {
		err = client.MakeBucket(ctx, BucketName, minio.MakeBucketOptions{})
		if err != nil {
			hlog.Fatalf("❌ 创建 MinIO Bucket 失败: %v", err)
		}

		// 极其关键：设置为“公开只读”策略，确保前端不用带签名就能直接渲染图片
		policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::` + BucketName + `/*"]}]}`
		err = client.SetBucketPolicy(ctx, BucketName, policy)
		if err != nil {
			hlog.Fatalf("❌ 设置 MinIO Bucket 策略失败: %v", err)
		}
		hlog.Infof("✅ 成功创建 MinIO Bucket [%s] 并已赋予公开读取权限", BucketName)
	} else {
		hlog.Infof("✅ MinIO Bucket [%s] 挂载正常", BucketName)
	}
}

// RemoveObject 从 MinIO 中彻底物理销毁指定对象
func RemoveObject(ctx context.Context, objectName string) error {
	if MinioClient == nil {
		return fmt.Errorf("MinIO 客户端未初始化")
	}

	// 执行真正的物理删除指令
	err := MinioClient.RemoveObject(ctx, BucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		hlog.CtxErrorf(ctx, "[MinIO] 物理销毁对象 [%s] 失败: %v", objectName, err)
		return err
	}

	hlog.CtxInfof(ctx, "[MinIO] 对象 [%s] 已被彻底物理销毁", objectName)
	return nil
}
