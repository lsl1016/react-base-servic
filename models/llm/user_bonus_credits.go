package model

import (
	"errors"
	"time"

	"react-base-service/components"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UserBonusCredits 管理员赠送积分表（粒度：user_name × model_hash）
type UserBonusCredits struct {
	ID        uint      `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	UserName  string    `json:"userName" gorm:"column:user_name;not null"`
	ModelHash string    `json:"modelHash" gorm:"column:model_hash;not null"`
	Credits   int       `json:"credits" gorm:"column:credits;not null;default:0"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (c *UserBonusCredits) TableName() string {
	return "tblLlmUserBonusCredits"
}

func GetBonusCredits(ctx *gin.Context, userName, modelHash string) (*UserBonusCredits, error) {
	var record UserBonusCredits
	err := helpers.MysqlClientLLM.Model(&UserBonusCredits{}).WithContext(ctx).
		Where("user_name = ? AND model_hash = ?", userName, modelHash).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &record, nil
}

// UpsertBonusCredits 不存在时创建，存在时按 delta 调整（delta 可为负数）
func UpsertBonusCredits(ctx *gin.Context, userName, modelHash string, delta int) error {
	tx := helpers.MysqlClientLLM.WithContext(ctx).
		Where("user_name = ? AND model_hash = ?", userName, modelHash).
		FirstOrCreate(&UserBonusCredits{UserName: userName, ModelHash: modelHash})
	if tx.Error != nil {
		return components.ErrorDbSelect.Wrap(tx.Error)
	}

	err := helpers.MysqlClientLLM.WithContext(ctx).
		Model(&UserBonusCredits{}).
		Where("user_name = ? AND model_hash = ?", userName, modelHash).
		UpdateColumn("credits", gorm.Expr("credits + ?", delta)).Error
	if err != nil {
		return components.ErrorDbUpdate.Wrap(err)
	}
	return nil
}

// DeductBonusCreditsToZero 将赠送积分清零（用于扣减时优先消耗赠送积分）
func DeductBonusCreditsToZero(ctx *gin.Context, userName, modelHash string) error {
	err := helpers.MysqlClientLLM.WithContext(ctx).
		Model(&UserBonusCredits{}).
		Where("user_name = ? AND model_hash = ?", userName, modelHash).
		UpdateColumn("credits", 0).Error
	if err != nil {
		return components.ErrorDbUpdate.Wrap(err)
	}
	return nil
}

// DeductBonusCredits 扣减赠送积分指定数量
func DeductBonusCredits(ctx *gin.Context, userName, modelHash string, amount int) error {
	err := helpers.MysqlClientLLM.WithContext(ctx).
		Model(&UserBonusCredits{}).
		Where("user_name = ? AND model_hash = ?", userName, modelHash).
		UpdateColumn("credits", gorm.Expr("credits - ?", amount)).Error
	if err != nil {
		return components.ErrorDbUpdate.Wrap(err)
	}
	return nil
}

// GetBonusCreditsForUpdate 加行锁查询（用于事务内扣减）
func GetBonusCreditsForUpdate(tx *gorm.DB, userName, modelHash string) (*UserBonusCredits, error) {
	var record UserBonusCredits
	err := tx.Model(&UserBonusCredits{}).
		Where("user_name = ? AND model_hash = ?", userName, modelHash).
		Set("gorm:query_option", "FOR UPDATE").
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &record, nil
}
