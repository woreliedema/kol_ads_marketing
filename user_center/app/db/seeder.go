package db

import (
	_ "embed"
	"encoding/json"
	"strings"

	"kol_ads_marketing/user_center/app/models"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

// 核心魔法：直接将 JSON 文件在编译时打包进二进制文件中
//
//go:embed kol_category.json
var kolCategoryJSON []byte

//go:embed brand_category.json
var brandCategoryJSON []byte

// JSON 反序列化结构体
type KolCategory struct {
	Category      string   `json:"category"`
	SubCategories []string `json:"sub_categories"`
}

type BrandCategory struct {
	BrandCategory      string             `json:"brand_category"`
	BrandSubCategories []BrandSubCategory `json:"brand_sub_categories"`
}

type BrandSubCategory struct {
	Name           string   `json:"name"`
	MappingGroupID string   `json:"mapping_group_id"`
	MappedKolTags  []string `json:"mapped_kol_tags"`
}

// InitSysTagsSeeder 初始化系统标签 (仅在表为空时执行一次)
func InitSysTagsSeeder() {
	var count int64
	// 1. 探针：检查表里是否有数据
	DB.Model(&models.SysTag{}).Count(&count)
	if count > 0 {
		hlog.Info("[DB Seeder] sys_tags 表已有数据，跳过初始播种。")
		return
	}

	hlog.Info("[DB Seeder] 侦测到 sys_tags 为空，开始静态解析并注入标签字典...")

	// 2. 解析内置的 JSON 数据
	var kolData []KolCategory
	var brandData []BrandCategory
	if err := json.Unmarshal(kolCategoryJSON, &kolData); err != nil {
		hlog.Fatalf("解析红人标签 JSON 失败: %v", err)
	}
	if err := json.Unmarshal(brandCategoryJSON, &brandData); err != nil {
		hlog.Fatalf("解析品牌标签 JSON 失败: %v", err)
	}

	// 3. 构建 Mapping 倒排索引
	kolMap := make(map[string][]string)
	for _, brandCat := range brandData {
		for _, subCat := range brandCat.BrandSubCategories {
			for _, kolTagName := range subCat.MappedKolTags {
				kolMap[kolTagName] = append(kolMap[kolTagName], subCat.MappingGroupID)
			}
		}
	}

	// 4. 开启事务，安全写入
	err := DB.Transaction(func(tx *gorm.DB) error {
		// --- 写入红人专属标签 (TargetType = 2) ---
		sortOrderLevel1 := 10
		for _, kolCat := range kolData {
			pTag := models.SysTag{
				Name:       kolCat.Category,
				Level:      1,
				TargetType: 2,
				SortOrder:  sortOrderLevel1,
			}
			if err := tx.Create(&pTag).Error; err != nil {
				return err
			}
			sortOrderLevel1 += 10

			sortOrderLevel2 := 10
			for _, subName := range kolCat.SubCategories {
				mappingStr := ""
				if groupIDs, ok := kolMap[subName]; ok {
					mappingStr = strings.Join(groupIDs, ",")
				}

				cTag := models.SysTag{
					ParentID:       pTag.ID,
					Name:           subName,
					Level:          2,
					TargetType:     2,
					MappingGroupID: mappingStr,
					SortOrder:      sortOrderLevel2,
				}
				if err := tx.Create(&cTag).Error; err != nil {
					return err
				}
				sortOrderLevel2 += 10
			}
		}

		// --- 写入品牌专属标签 (TargetType = 1) ---
		sortOrderLevel1 = 100
		for _, brandCat := range brandData {
			pTag := models.SysTag{
				Name:       brandCat.BrandCategory,
				Level:      1,
				TargetType: 1,
				SortOrder:  sortOrderLevel1,
			}
			if err := tx.Create(&pTag).Error; err != nil {
				return err
			}
			sortOrderLevel1 += 10

			sortOrderLevel2 := 10
			for _, subCat := range brandCat.BrandSubCategories {
				cTag := models.SysTag{
					ParentID:       pTag.ID,
					Name:           subCat.Name,
					Level:          2,
					TargetType:     1,
					MappingGroupID: subCat.MappingGroupID,
					SortOrder:      sortOrderLevel2,
				}
				if err := tx.Create(&cTag).Error; err != nil {
					return err
				}
				sortOrderLevel2 += 10
			}
		}
		return nil
	})

	if err != nil {
		hlog.Fatalf("[DB Seeder] 标签播种事务执行失败: %v", err)
	}
	hlog.Info("[DB Seeder] sys_tags 字典数据注入成功！")
}
