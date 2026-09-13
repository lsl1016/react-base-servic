package model

import (
	"errors"
	"time"

	"react-base-service/components"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

// Bundle 资源类型（tblLlmBundleResource.resource_type）。
const (
	BundleResourceTypeAgent      = "agent"
	BundleResourceTypeSkill      = "skill"
	BundleResourceTypeMcpServer  = "mcp_server"
)

// Bundle 是 Agent Bundle 安装记录（P3 插件包）：
// 安装时把包内 agents/skills/mcp.json 展开写入各注册表，资源清单记入 BundleResource；
// 卸载按清单回滚（新建资源软删、覆盖资源按快照恢复）。
type Bundle struct {
	ID           uint                  `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	BundleID     string                `json:"bundleId" gorm:"column:bundle_id;not null"`
	Name         string                `json:"name" gorm:"column:name;not null"`
	Version      string                `json:"version" gorm:"column:version;not null;default:'1.0.0'"`
	Source       string                `json:"source" gorm:"column:source;not null"`
	ResolvedRef  string                `json:"resolvedRef" gorm:"column:resolved_ref;not null;default:''"`
	ManifestJSON string                `json:"manifestJson" gorm:"column:manifest_json"`
	InstalledBy  string                `json:"installedBy" gorm:"column:installed_by;not null;default:''"`
	CreatedAt    time.Time             `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt    time.Time             `json:"updatedAt" gorm:"column:updated_at"`
	DeletedAt    soft_delete.DeletedAt `json:"deletedAt" gorm:"column:deleted_at;not null;default:0"`
}

func (b *Bundle) TableName() string {
	return "tblLlmBundle"
}

// BundleResource 是 Bundle 资源清单行；PreviousStateJSON 为空表示该资源由安装新建。
type BundleResource struct {
	ID                uint      `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	BundleID          string    `json:"bundleId" gorm:"column:bundle_id;not null"`
	ResourceType      string    `json:"resourceType" gorm:"column:resource_type;not null"`
	ResourceKey       string    `json:"resourceKey" gorm:"column:resource_key;not null"`
	ResourceID        string    `json:"resourceId" gorm:"column:resource_id;not null"`
	PreviousStateJSON string    `json:"previousStateJson" gorm:"column:previous_state_json"`
	CreatedAt         time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt         time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (r *BundleResource) TableName() string {
	return "tblLlmBundleResource"
}

func CreateBundle(ctx *gin.Context, bundle *Bundle) error {
	err := helpers.MysqlClientLLM.Model(&Bundle{}).WithContext(ctx).Create(bundle).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

func GetBundleByName(ctx *gin.Context, name string) (*Bundle, error) {
	var bundle Bundle
	err := helpers.MysqlClientLLM.Model(&Bundle{}).WithContext(ctx).
		Where("name = ?", name).First(&bundle).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &bundle, nil
}

func GetBundleByNameUnscoped(ctx *gin.Context, name string) (*Bundle, error) {
	var bundle Bundle
	err := helpers.MysqlClientLLM.Unscoped().Model(&Bundle{}).WithContext(ctx).
		Where("name = ?", name).First(&bundle).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &bundle, nil
}

func GetBundleByBundleID(ctx *gin.Context, bundleID string) (*Bundle, error) {
	var bundle Bundle
	err := helpers.MysqlClientLLM.Model(&Bundle{}).WithContext(ctx).
		Where("bundle_id = ?", bundleID).First(&bundle).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &bundle, nil
}

func ListBundles(ctx *gin.Context) ([]Bundle, error) {
	var bundles []Bundle
	err := helpers.MysqlClientLLM.Model(&Bundle{}).WithContext(ctx).
		Order("created_at DESC, id DESC").Find(&bundles).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return bundles, nil
}

func UpdateBundleByBundleIDUnscoped(ctx *gin.Context, bundleID string, updates map[string]interface{}) error {
	tx := helpers.MysqlClientLLM.Unscoped().Model(&Bundle{}).WithContext(ctx).
		Where("bundle_id = ?", bundleID).Updates(updates)
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func SoftDeleteBundleByBundleID(ctx *gin.Context, bundleID string) error {
	tx := helpers.MysqlClientLLM.Model(&Bundle{}).WithContext(ctx).
		Where("bundle_id = ?", bundleID).Delete(&Bundle{})
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func CreateBundleResources(ctx *gin.Context, resources []BundleResource) error {
	if len(resources) == 0 {
		return nil
	}
	err := helpers.MysqlClientLLM.Model(&BundleResource{}).WithContext(ctx).Create(&resources).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

func ListBundleResourcesByBundleID(ctx *gin.Context, bundleID string) ([]BundleResource, error) {
	var resources []BundleResource
	err := helpers.MysqlClientLLM.Model(&BundleResource{}).WithContext(ctx).
		Where("bundle_id = ?", bundleID).Order("id ASC").Find(&resources).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return resources, nil
}

// HardDeleteBundleResourcesByBundleID 物理删除资源清单（仅重装同名字时清旧记录用；
// 卸载走软删 bundle 行 + 清单保留作为历史）。
func HardDeleteBundleResourcesByBundleID(ctx *gin.Context, bundleID string) error {
	tx := helpers.MysqlClientLLM.Model(&BundleResource{}).WithContext(ctx).
		Where("bundle_id = ?", bundleID).Delete(&BundleResource{})
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}
