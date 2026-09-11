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

type Caller struct {
	ID          uint                  `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	CallerKey   string                `json:"callerKey" gorm:"column:caller_key;not null"`
	Name        string                `json:"name" gorm:"column:name;not null"`
	Description string                `json:"description" gorm:"column:description"`
	Platform    string                `json:"platform" gorm:"column:platform;not null;default:''"`
	Status      int                   `json:"status" gorm:"column:status;not null;default:1"`
	CreatedBy   string                `json:"createdBy" gorm:"column:created_by;not null;default:''"`
	CreatedAt   time.Time             `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time             `json:"updatedAt" gorm:"column:updated_at"`
	DeletedAt   soft_delete.DeletedAt `json:"deletedAt" gorm:"column:deleted_at;not null;default:0"`
}

func (c *Caller) TableName() string {
	return "tblLlmCaller"
}

func CreateCaller(ctx *gin.Context, caller *Caller) error {
	err := helpers.MysqlClientLLM.Model(&Caller{}).WithContext(ctx).Create(caller).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

// GetCallerByKey 按 callerKey 查询（不过滤 status，用于管理后台查重/更新等场景）
func GetCallerByKey(ctx *gin.Context, callerKey string) (*Caller, error) {
	var caller Caller
	err := helpers.MysqlClientLLM.Model(&Caller{}).WithContext(ctx).
		Where("caller_key = ?", callerKey).First(&caller).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &caller, nil
}

// GetActiveCallerByKey 按 callerKey 查询已启用的 caller（用于 pipeline 运行时校验）
func GetActiveCallerByKey(ctx *gin.Context, callerKey string) (*Caller, error) {
	var caller Caller
	err := helpers.MysqlClientLLM.Model(&Caller{}).WithContext(ctx).
		Where("caller_key = ? AND status = 1", callerKey).First(&caller).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &caller, nil
}

func UpdateCallerByKey(ctx *gin.Context, callerKey string, updates map[string]interface{}) error {
	tx := helpers.MysqlClientLLM.Model(&Caller{}).WithContext(ctx).
		Where("caller_key = ?", callerKey).
		Updates(updates)
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func ListCallers(ctx *gin.Context) ([]Caller, error) {
	var callers []Caller
	err := helpers.MysqlClientLLM.Model(&Caller{}).WithContext(ctx).
		Order("created_at DESC").
		Find(&callers).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return callers, nil
}
