package handlers

import (
	"context"
	"errors"
	"fmt"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"strings"

	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/models"
	"kol_ads_marketing/user_center/app/service"

	"github.com/cloudwego/hertz/pkg/app"
)

// 1. 查询账号信息接口

// GetUserInfo 获取当前登录用户的详细信息
// @Summary 获取账号详细信息
// @Description 根据当前 Token 获取用户的基本信息及对应角色的扩展资料（KOL或品牌方）
// @Tags User
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} map[string]interface{} "成功返回用户信息"
// @Router /api/v1/user/info [get]
func GetUserInfo(c context.Context, ctx *app.RequestContext) {
	userIDAny, _ := ctx.Get("user_id")
	userID := userIDAny.(uint64)

	// 直接下推给 Service 层 (内部接口和外部接口共用同一个大脑)
	responseData, err := service.GetUserProfileService(c, userID)
	if err != nil {
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			response.Error(ctx, apiErr)
		} else {
			hlog.CtxErrorf(c, "GetUserInfo 未知异常: %v", err)
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}

	response.Success(ctx, responseData)
}

// 2. 修改 KOL (红人) 扩展信息接口

type UpdateKOLProfileReq struct {
	RealName  string `json:"real_name"`
	Tags      string `json:"tags"`       // JSON字符串，如 "[\"游戏\",\"主播\",\"知识\"]"
	BaseQuote int    `json:"base_quote"` // 基础报价
}

// UpdateKOLProfile 修改 KOL 专属扩展资料
// @Summary 修改 KOL 扩展资料
// @Description 仅限 KOL(role=1) 调用。修改昵称、标签、底价等参数。
// @Tags User
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body UpdateKOLProfileReq true "KOL 资料参数"
// @Success 200 {object} map[string]interface{} "成功提示"
// @Router /api/v1/user/kol/profile [put]
func UpdateKOLProfile(c context.Context, ctx *app.RequestContext) {
	roleAny, _ := ctx.Get("role")
	role := roleAny.(models.RoleType)

	// 极客防御：越权拦截 (放开在 Handler 层拦截，因为属于请求合法性范畴)
	if role != models.RoleKOL {
		response.ErrorWithMsg(ctx, response.ErrUnauthorized, "越权访问：该接口仅限红人(KOL)调用")
		return
	}

	var req UpdateKOLProfileReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}

	userIDAny, _ := ctx.Get("user_id")
	userID := userIDAny.(uint64)

	// 直接下推给 Service 层
	if err := service.UpdateKOLProfileService(c, userID, req.RealName, req.Tags, req.BaseQuote); err != nil {
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			response.Error(ctx, apiErr)
		} else {
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}

	response.Success(ctx, map[string]interface{}{"message": "KOL 资料更新成功"})
}

// 3. 修改 Brand (品牌方) 扩展信息接口

type UpdateBrandProfileReq struct {
	CompanyName string `json:"company_name"`
	Industry    string `json:"industry"`
}

// UpdateBrandProfile 修改品牌方专属扩展资料
// @Summary 修改品牌方扩展资料
// @Description 仅限品牌方(role=2) 调用。修改公司名、所属行业等参数。
// @Tags User
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body UpdateBrandProfileReq true "品牌方资料参数"
// @Success 200 {object} map[string]interface{} "成功提示"
// @Router /api/v1/user/brand/profile [put]
func UpdateBrandProfile(c context.Context, ctx *app.RequestContext) {
	roleAny, _ := ctx.Get("role")
	role := roleAny.(models.RoleType)

	if role != models.RoleBrand {
		response.ErrorWithMsg(ctx, response.ErrUnauthorized, "越权访问：该接口仅限品牌方调用")
		return
	}

	var req UpdateBrandProfileReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}

	userIDAny, _ := ctx.Get("user_id")
	userID := userIDAny.(uint64)

	// 直接下推给 Service 层
	if err := service.UpdateBrandProfileService(c, userID, req.CompanyName, req.Industry); err != nil {
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			response.Error(ctx, apiErr)
		} else {
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}

	response.Success(ctx, map[string]interface{}{"message": "品牌方资料更新成功"})
}

// UploadAvatar 统一头像上传接口 (双角色通用)
// @Summary 上传用户头像 (7天冷却)
// @Description 上传图片文件，自动根据当前 Token 身份更新红人或品牌方的头像。每人每 7 天只能修改一次。
// @Tags User
// @Accept multipart/form-data
// @Produce json
// @Security ApiKeyAuth
// @Param avatar formData file true "头像图片文件 (最大 5MB, 仅支持 jpg/png)"
// @Success 200 {object} map[string]interface{} "成功返回新的头像 URL"
// @Router /api/v1/user/avatar/upload [post]
func UploadAvatar(c context.Context, ctx *app.RequestContext) {
	// 1. 提取基础信息
	userIDAny, _ := ctx.Get("user_id")
	roleAny, _ := ctx.Get("role")
	userID := userIDAny.(uint64)
	role := roleAny.(models.RoleType)

	// 2. 拦截器：调用 Service 检查 7 天冷却锁
	if err := service.CheckAvatarCooldown(c, userID); err != nil {
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			response.Error(ctx, apiErr)
		} else {
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}

	// 3. 读取前端传来的文件
	fileHeader, err := ctx.FormFile("avatar")
	if err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "读取文件失败，请确保字段名为 avatar")
		return
	}

	// 4. 基础安全校验：大小与后缀名
	if fileHeader.Size > 5*1024*1024 { // 5MB
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "图片不能超过 5MB")
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "仅支持 JPG/PNG 格式的图片")
		return
	}

	// 5. 生成极其安全的防冲突文件名 (UUID)
	newFileName := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	// 确保本地有个存放目录 (模拟 OSS bucket)
	uploadDir := "./uploads/avatars/"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		hlog.CtxErrorf(c, "创建上传目录失败: %v", err)
		response.Error(ctx, response.ErrSystemError)
		return
	}

	savePath := filepath.Join(uploadDir, newFileName)

	// 6. 核心动作：把文件保存到本地硬盘 (未来这里换成上传到 OSS)
	if err := ctx.SaveUploadedFile(fileHeader, savePath); err != nil {
		hlog.CtxErrorf(c, "保存文件到本地失败: %v", err)
		response.Error(ctx, response.ErrSystemError)
		return
	}

	// 7. 拼接出一个能让前端直接访问的 URL (假设你的服务跑在 8081)
	// 在生产环境中，这应该是一个 CDN 域名，如 https://cdn.yourdomain.com/avatars/...
	// 只存储绝对 URI 路径
	avatarURL := fmt.Sprintf("/uploads/avatars/%s", newFileName)

	// 🚀 8. 业务入库：调用 Service 更新数据库，并挂上 7 天的锁！
	if err := service.UpdateUserAvatar(c, userID, role, avatarURL); err != nil {
		response.Error(ctx, response.ErrSystemError)
		return
	}

	// 9. 成功返回
	response.Success(ctx, map[string]interface{}{
		"message":    "头像修改成功",
		"avatar_url": avatarURL,
	})
}

// UploadBusinessLicense 品牌方上传营业执照接口
// @Summary 上传营业执照 (仅限品牌方)
// @Description 品牌方角色专属。上传营业执照图片，大小不超过 10MB。
// @Tags User
// @Accept multipart/form-data
// @Produce json
// @Security ApiKeyAuth
// @Param license formData file true "营业执照图片 (最大 10MB, 仅支持 jpg/png)"
// @Success 200 {object} map[string]interface{} "成功返回新的资质 URL"
// @Router /api/v1/user/brand/license/upload [post]
func UploadBusinessLicense(c context.Context, ctx *app.RequestContext) {
	// 1. 提取角色并进行极客防御：越权拦截！
	roleAny, _ := ctx.Get("role")
	role := roleAny.(models.RoleType)
	if role != models.RoleBrand {
		response.ErrorWithMsg(ctx, response.ErrUnauthorized, "越权访问：该资质上传接口仅限品牌方使用")
		return
	}

	userIDAny, _ := ctx.Get("user_id")
	userID := userIDAny.(uint64)

	// 2. 读取前端传来的资质文件 (字段名要求为 license)
	fileHeader, err := ctx.FormFile("license")
	if err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "读取文件失败，请确保字段名为 license")
		return
	}

	// 3. 资质文件体积放宽 (通常营业执照扫描件较大，这里放宽到 10MB)
	if fileHeader.Size > 10*1024*1024 {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "图片不能超过 10MB")
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, "目前仅支持 JPG/PNG 格式的资质图片")
		return
	}

	// 4. 生成防冲突文件名
	newFileName := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	// 核心架构：机密资质必须与普通头像物理隔离
	uploadDir := "./uploads/licenses/"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		hlog.CtxErrorf(c, "创建资质上传目录失败: %v", err)
		response.Error(ctx, response.ErrSystemError)
		return
	}

	savePath := filepath.Join(uploadDir, newFileName)

	// 5. 保存文件到本地
	if err := ctx.SaveUploadedFile(fileHeader, savePath); err != nil {
		hlog.CtxErrorf(c, "保存资质文件失败: %v", err)
		response.Error(ctx, response.ErrSystemError)
		return
	}

	// 6. 生成绝对 URI (依靠我们之前配好的 h.Static("/uploads", "./"))
	licenseURL := fmt.Sprintf("/uploads/licenses/%s", newFileName)

	// 7. 下推给 Service 入库
	if err := service.UpdateBrandLicenseService(c, userID, licenseURL); err != nil {
		response.Error(ctx, response.ErrSystemError)
		return
	}

	// 8. 成功返回
	response.Success(ctx, map[string]interface{}{
		"message":     "营业执照上传成功",
		"license_url": licenseURL,
	})
}

// DeleteLicenseReq 删除营业执照时二次确认弹窗接收前端传来的密码
type DeleteLicenseReq struct {
	// 加上 vd 标签，防范前端传空字符串
	Password string `json:"password" vd:"required;msg:'密码不能为空'"`
}

// DeleteBusinessLicense 验证密码并销毁营业执照
// @Summary 销毁营业执照 (仅限品牌方)
// @Description 验证登录密码后，物理删除服务器上的资质文件并清空记录。
// @Tags User
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body DeleteLicenseReq true "密码校验参数"
// @Success 200 {object} map[string]interface{} "成功提示"
// @Router /api/v1/user/brand/license/delete [post]
func DeleteBusinessLicense(c context.Context, ctx *app.RequestContext) {
	// 1. 越权防御
	roleAny, _ := ctx.Get("role")
	role := roleAny.(models.RoleType)
	if role != models.RoleBrand {
		response.ErrorWithMsg(ctx, response.ErrUnauthorized, "越权访问：该功能仅限品牌方使用")
		return
	}

	// 2. 参数绑定与校验
	var req DeleteLicenseReq
	if err := ctx.BindAndValidate(&req); err != nil {
		response.ErrorWithMsg(ctx, response.ErrInvalidParams, err.Error())
		return
	}

	userIDAny, _ := ctx.Get("user_id")
	userID := userIDAny.(uint64)

	// 3. 核心下推：将 userID 和 前端传来的密码 扔给 Service 处理
	if err := service.DeleteBrandLicenseService(c, userID, req.Password); err != nil {
		var apiErr *response.APIError
		if errors.As(err, &apiErr) {
			response.Error(ctx, apiErr)
		} else {
			response.Error(ctx, response.ErrSystemError)
		}
		return
	}

	// 4. 成功返回
	response.Success(ctx, map[string]interface{}{
		"message": "敏感资质已安全物理销毁",
	})
}
