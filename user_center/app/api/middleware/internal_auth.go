package middleware

import (
	"context"
	"os"

	"kol_ads_marketing/user_center/app/api/response"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// InternalAuth 内部微服务通信专属鉴权中间件
func InternalAuth() app.HandlerFunc {
	// 启动时从环境变量或 .env 读取内部通讯秘钥
	secretKey := os.Getenv("INTERNAL_SECRET_KEY")
	if secretKey == "" {
		hlog.Fatal("致命错误：未配置 INTERNAL_SECRET_KEY，无法启动内部服务防线！")
	}

	return func(c context.Context, ctx *app.RequestContext) {
		// 约定：内部调用必须在 Header 中携带 X-Internal-Secret
		clientKey := string(ctx.GetHeader("X-Internal-Secret"))

		if clientKey != secretKey {
			hlog.CtxWarnf(c, "拦截到非法的内部接口调用请求！来源 IP: %s", ctx.ClientIP())
			// 抛出 403 权限不足
			response.ErrorWithMsg(ctx, response.ErrPermission, "非法内部调用：凭证不匹配")
			ctx.Abort() // 终止传递
			return
		}

		// 秘钥匹配，放行！
		ctx.Next(c)
	}
}
