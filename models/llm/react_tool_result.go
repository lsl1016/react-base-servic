package model

import (
	"errors"
	"time"

	"react-base-service/components"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReactToolResult 保存 resultRef 指向的完整工具结果。
type ReactToolResult struct {
	ID        uint       `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	ResultRef string     `json:"resultRef" gorm:"column:result_ref;not null"`
	SessionID string     `json:"sessionId" gorm:"column:session_id;not null"`
	RunID     string     `json:"runId" gorm:"column:run_id;not null"`
	ToolUseID string     `json:"toolUseId" gorm:"column:tool_use_id;not null"`
	ToolName  string     `json:"toolName" gorm:"column:tool_name;not null;default:''"`
	Content   string     `json:"content" gorm:"column:content;type:mediumtext;not null"`
	SizeBytes int        `json:"sizeBytes" gorm:"column:size_bytes;not null;default:0"`
	ExpireAt  *time.Time `json:"expireAt" gorm:"column:expire_at"`
	CreatedAt time.Time  `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"column:updated_at"`
}

func (r *ReactToolResult) TableName() string {
	return "tblLlmReactToolResult"
}

func CreateReactToolResult(ctx *gin.Context, result *ReactToolResult) error {
	err := helpers.MysqlClientLLM.Model(&ReactToolResult{}).WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(result).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

func GetReactToolResultByResultRef(ctx *gin.Context, resultRef string) (*ReactToolResult, error) {
	var result ReactToolResult
	err := helpers.MysqlClientLLM.Model(&ReactToolResult{}).WithContext(ctx).
		Where("result_ref = ?", resultRef).First(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &result, nil
}

func DeleteExpiredReactToolResults(ctx *gin.Context, limit int) error {
	if limit <= 0 {
		limit = 1000
	}
	err := helpers.MysqlClientLLM.WithContext(ctx).
		Where("expire_at IS NOT NULL AND expire_at < ?", time.Now()).
		Limit(limit).
		Delete(&ReactToolResult{}).Error
	if err != nil {
		return components.ErrorDbUpdate.Wrap(err)
	}
	return nil
}
