package model

import (
	"errors"
	"time"

	"react-base-service/components"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/soft_delete"
)

var ErrToolUserPolicyAlreadyExists = errors.New("tool user policy already exists")

// ToolUserPolicy 保存业务工具的用户可见性策略。
// BlackUserList 是预留字段，本期不参与接口和运行时判断。
type ToolUserPolicy struct {
	ID            uint                  `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	ToolID        string                `json:"toolId" gorm:"column:tool_id;not null"`
	WhiteUserList string                `json:"whiteUserList" gorm:"column:white_user_list"`
	BlackUserList string                `json:"blackUserList" gorm:"column:black_user_list"`
	CreatedBy     string                `json:"createdBy" gorm:"column:created_by;not null;default:''"`
	UpdatedBy     string                `json:"updatedBy" gorm:"column:updated_by;not null;default:''"`
	CreatedAt     time.Time             `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt     time.Time             `json:"updatedAt" gorm:"column:updated_at"`
	DeletedAt     soft_delete.DeletedAt `json:"deletedAt" gorm:"column:deleted_at;not null;default:0"`
}

func (p *ToolUserPolicy) TableName() string {
	return "tblLlmToolUserPolicy"
}

func GetToolUserPolicyByToolID(ctx *gin.Context, toolID string) (*ToolUserPolicy, error) {
	var policy ToolUserPolicy
	err := helpers.MysqlClientLLM.Model(&ToolUserPolicy{}).WithContext(ctx).
		Where("tool_id = ?", toolID).
		First(&policy).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &policy, nil
}

// CreateOrRestoreToolUserPolicy 新建策略；如果唯一 tool_id 对应的是软删除记录，则恢复该记录。
func CreateOrRestoreToolUserPolicy(ctx *gin.Context, policy *ToolUserPolicy) error {
	err := helpers.MysqlClientLLM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing ToolUserPolicy
		err := tx.Unscoped().
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tool_id = ?", policy.ToolID).
			First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return components.ErrorDbSelect.Wrap(err)
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(policy).Error; err != nil {
				return components.ErrorDbInsert.Wrap(err)
			}
			return nil
		}
		if existing.DeletedAt == 0 {
			return ErrToolUserPolicyAlreadyExists
		}

		updates := map[string]interface{}{
			"white_user_list": policy.WhiteUserList,
			"black_user_list": "[]",
			"updated_by":      policy.UpdatedBy,
			"deleted_at":      0,
		}
		if err := tx.Unscoped().Model(&ToolUserPolicy{}).
			Where("tool_id = ?", policy.ToolID).
			Updates(updates).Error; err != nil {
			return components.ErrorDbUpdate.Wrap(err)
		}
		return nil
	})
	return err
}

func UpdateToolUserPolicyByToolID(ctx *gin.Context, toolID string, updates map[string]interface{}) (bool, error) {
	var updated bool
	err := helpers.MysqlClientLLM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var policy ToolUserPolicy
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tool_id = ?", toolID).
			First(&policy).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return components.ErrorDbSelect.Wrap(err)
		}
		if err := tx.Model(&policy).Updates(updates).Error; err != nil {
			return components.ErrorDbUpdate.Wrap(err)
		}
		updated = true
		return nil
	})
	return updated, err
}

func SoftDeleteToolUserPolicyByToolID(ctx *gin.Context, toolID, updatedBy string) (bool, error) {
	var deleted bool
	err := helpers.MysqlClientLLM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var policy ToolUserPolicy
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tool_id = ?", toolID).
			First(&policy).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return components.ErrorDbSelect.Wrap(err)
		}
		if err := tx.Model(&policy).Update("updated_by", updatedBy).Error; err != nil {
			return components.ErrorDbUpdate.Wrap(err)
		}
		if err := tx.Delete(&policy).Error; err != nil {
			return components.ErrorDbUpdate.Wrap(err)
		}
		deleted = true
		return nil
	})
	return deleted, err
}

func ListToolUserPoliciesByToolIDs(ctx *gin.Context, toolIDs []string) ([]ToolUserPolicy, error) {
	if len(toolIDs) == 0 {
		return []ToolUserPolicy{}, nil
	}
	var policies []ToolUserPolicy
	err := helpers.MysqlClientLLM.Model(&ToolUserPolicy{}).WithContext(ctx).
		Where("tool_id IN ?", toolIDs).
		Order("created_at DESC").
		Find(&policies).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return policies, nil
}
