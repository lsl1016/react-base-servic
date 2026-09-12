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

type SystemPrompt struct {
	ID          uint                  `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	CallerKey   string                `json:"callerKey" gorm:"column:caller_key;not null"`
	RouteValues string                `json:"routeValues" gorm:"column:route_values"`
	Name        string                `json:"name" gorm:"column:name;not null"`
	Content     string                `json:"content" gorm:"column:content;not null"`
	Status      int                   `json:"status" gorm:"column:status;not null;default:1"`
	CreatedBy   string                `json:"createdBy" gorm:"column:created_by;not null;default:''"`
	UpdatedBy   string                `json:"updatedBy" gorm:"column:updated_by;not null;default:''"`
	CreatedAt   time.Time             `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time             `json:"updatedAt" gorm:"column:updated_at"`
	DeletedAt   soft_delete.DeletedAt `json:"deletedAt" gorm:"column:deleted_at;not null;default:0"`
}

func (s *SystemPrompt) TableName() string {
	return "tblLlmSystemPrompt"
}

func CreateSystemPrompt(ctx *gin.Context, sp *SystemPrompt) error {
	err := helpers.MysqlClientLLM.Model(&SystemPrompt{}).WithContext(ctx).Create(sp).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

func GetSystemPromptByID(ctx *gin.Context, id uint) (*SystemPrompt, error) {
	var sp SystemPrompt
	err := helpers.MysqlClientLLM.Model(&SystemPrompt{}).WithContext(ctx).
		Where("id = ?", id).First(&sp).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &sp, nil
}

// ExistSystemPromptByCallerAndRoute 检查 caller+route 组合下是否已有有效记录
func ExistSystemPromptByCallerAndRoute(ctx *gin.Context, callerKey, routeValues string) (bool, error) {
	var count int64
	err := helpers.MysqlClientLLM.Model(&SystemPrompt{}).WithContext(ctx).
		Where("caller_key = ? AND route_values = ?", callerKey, routeValues).
		Count(&count).Error
	if err != nil {
		return false, components.ErrorDbSelect.Wrap(err)
	}
	return count > 0, nil
}

func UpdateSystemPromptByID(ctx *gin.Context, id uint, updates map[string]interface{}) error {
	tx := helpers.MysqlClientLLM.Model(&SystemPrompt{}).WithContext(ctx).
		Where("id = ?", id).
		Updates(updates)
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func SoftDeleteSystemPromptByID(ctx *gin.Context, id uint) error {
	tx := helpers.MysqlClientLLM.WithContext(ctx).Where("id = ?", id).Delete(&SystemPrompt{})
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func ListSystemPromptsByCaller(ctx *gin.Context, callerKey string) ([]SystemPrompt, error) {
	var prompts []SystemPrompt
	err := helpers.MysqlClientLLM.Model(&SystemPrompt{}).WithContext(ctx).
		Where("caller_key = ?", callerKey).
		Order("created_at DESC").
		Find(&prompts).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return prompts, nil
}

// ListSystemPromptsByCallerAndRoutes 按 callerKey + 路由前缀匹配查询系统提示词列表（不过滤 status，管理后台用）
func ListSystemPromptsByCallerAndRoutes(ctx *gin.Context, callerKey string, routePrefixes []string) ([]SystemPrompt, error) {
	var prompts []SystemPrompt
	err := helpers.MysqlClientLLM.Model(&SystemPrompt{}).WithContext(ctx).
		Where("caller_key = ? AND route_values IN ?", callerKey, routePrefixes).
		Order("created_at DESC").
		Find(&prompts).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return prompts, nil
}

// ListSystemPromptsByCallerAndExactRoute 按 callerKey + routeValues 精确匹配查询系统提示词列表
func ListSystemPromptsByCallerAndExactRoute(ctx *gin.Context, callerKey string, routeValues string) ([]SystemPrompt, error) {
	var prompts []SystemPrompt
	err := helpers.MysqlClientLLM.Model(&SystemPrompt{}).WithContext(ctx).
		Where("caller_key = ? AND route_values = ?", callerKey, routeValues).
		Order("created_at DESC").
		Find(&prompts).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return prompts, nil
}

// FindSystemPromptsByCallerAndRoutes 按 callerKey + 路由前缀匹配查询系统提示词；
// 同时并入「默认作用域」（caller_key=default）下命中的提示词，对全部 caller 生效。
func FindSystemPromptsByCallerAndRoutes(ctx *gin.Context, callerKey string, routePrefixes []string) ([]SystemPrompt, error) {
	var prompts []SystemPrompt
	err := helpers.MysqlClientLLM.Model(&SystemPrompt{}).WithContext(ctx).
		Where("caller_key IN ? AND status = 1 AND route_values IN ?", CallerScopeKeys(callerKey), routePrefixes).
		Find(&prompts).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return prompts, nil
}

// ListAllSystemPrompts 列出全部 caller 的系统提示词（管理控制台「全部」视图用）。
func ListAllSystemPrompts(ctx *gin.Context) ([]SystemPrompt, error) {
	var prompts []SystemPrompt
	err := helpers.MysqlClientLLM.Model(&SystemPrompt{}).WithContext(ctx).
		Order("caller_key ASC, created_at DESC").
		Find(&prompts).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return prompts, nil
}
