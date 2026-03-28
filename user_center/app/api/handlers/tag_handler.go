package handlers

import (
	"context"

	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/models"

	"github.com/cloudwego/hertz/pkg/app"
)

// TagNode 树形节点结构体
type TagNode struct {
	ID       uint64    `json:"id"`
	Name     string    `json:"name"`
	Children []TagNode `json:"children,omitempty"`
}

// GetTagTreeReq 接收前端传参
//type GetTagTreeReq struct {
//	//TargetType int8 `query:"target_type"` // 可选: 1-品牌行业标签, 2-红人内容标签
//	TargetType int8 `query:"target_type" form:"target_type" json:"target_type"`
//}

// GetTagTree 获取当前角色的专属标签树
// @Router /api/v1/tags/tree [get]
func GetTagTree(c context.Context, ctx *app.RequestContext) {
	// 🚀 探针 1：直接抓取最底层的原始 URL，看看到底有没有问号后面的东西！
	//rawURL := string(ctx.Request.URI().FullURI())
	//rawQuery := string(ctx.Request.URI().QueryString())
	//
	//// 🚀 探针 2：尝试用 Hertz 最底层的 API 获取参数
	//targetTypeBytes := ctx.QueryArgs().Peek("target_type")
	//targetTypeStr := string(targetTypeBytes)

	targetTypeStr := string(ctx.Query("target_type"))

	var targetType int8
	if targetTypeStr == "1" {
		targetType = 1
	} else if targetTypeStr == "2" {
		targetType = 2
	} else {
		// 🚀 兜底逻辑：如果前端没传 target_type（为空或传错），则根据当前用户角色下发专属标签
		roleAny, _ := ctx.Get("role")
		if roleAny != nil && roleAny.(models.RoleType) == models.RoleBrand {
			targetType = 1 // 品牌方默认拿品牌行业标签
		} else {
			targetType = 2 // 否则默认拿红人内容标签
		}
	}

	var tags []models.SysTag
	// 查询正常状态下的专属标签，按父级和权重排序
	if err := db.DB.Where("target_type IN (?, 3) AND status = 1", targetType).
		Order("parent_id ASC, sort_order ASC").Find(&tags).Error; err != nil {
		response.Error(ctx, response.ErrDatabaseError)
		return
	}

	// 算法：构建父子树
	nodeMap := make(map[uint64]*TagNode)
	var rootNodes []TagNode

	// 第一遍：把所有节点塞进 Map
	for _, t := range tags {
		nodeMap[t.ID] = &TagNode{ID: t.ID, Name: t.Name}
	}

	// 第二遍：组装父子关系
	for _, t := range tags {
		node := nodeMap[t.ID]
		if t.ParentID == 0 {
			// 是一级大类
			rootNodes = append(rootNodes, *node)
		} else {
			// 是二级分类，挂载到父节点下
			if parent, exists := nodeMap[t.ParentID]; exists {
				parent.Children = append(parent.Children, *node)
			}
		}
	}
	//ctx.JSON(200, map[string]interface{}{
	//	"code": 200,
	//	"debug_info": map[string]interface{}{
	//		"1_raw_url":        rawURL,
	//		"2_raw_query":      rawQuery,
	//		"3_parsed_target":  targetTypeStr,
	//		"4_final_db_query": targetType, // 如果这里是 2，说明查的就是红人
	//	},
	//	"data": map[string]interface{}{
	//		"tree": rootNodes,
	//	},
	//})

	// 因为刚才 append 进去的是值拷贝，为了让 Children 生效，我们直接利用第一遍找到的 root 重新组装
	var finalTree []TagNode
	for _, t := range tags {
		if t.ParentID == 0 {
			finalTree = append(finalTree, *nodeMap[t.ID])
		}
	}

	response.Success(ctx, map[string]interface{}{"tree": finalTree})
}
