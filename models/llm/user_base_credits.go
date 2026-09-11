package model

import (
	"errors"
	"time"

	"react-base-service/components"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UserBaseCredits 月度基础积分表（粒度：user_name × model_hash）
type UserBaseCredits struct {
	ID         uint      `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	UserName   string    `json:"userName" gorm:"column:user_name;not null"`
	ModelHash  string    `json:"modelHash" gorm:"column:model_hash;not null"`
	Credits    int       `json:"credits" gorm:"column:credits;not null;default:1000"`
	ResetMonth string    `json:"resetMonth" gorm:"column:reset_month;not null;default:''"`
	CreatedAt  time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt  time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (c *UserBaseCredits) TableName() string {
	return "tblLlmUserBaseCredits"
}

func GetBaseCredits(ctx *gin.Context, userName, modelHash string) (*UserBaseCredits, error) {
	var record UserBaseCredits
	err := helpers.MysqlClientLLM.Model(&UserBaseCredits{}).WithContext(ctx).
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

func CreateBaseCredits(ctx *gin.Context, record *UserBaseCredits) error {
	err := helpers.MysqlClientLLM.Model(&UserBaseCredits{}).WithContext(ctx).Create(record).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

func UpdateBaseCredits(ctx *gin.Context, userName, modelHash string, updates map[string]interface{}) error {
	tx := helpers.MysqlClientLLM.Model(&UserBaseCredits{}).WithContext(ctx).
		Where("user_name = ? AND model_hash = ?", userName, modelHash).
		Updates(updates)
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

// DeductBaseCredits 扣减基础积分（积分可扣为负数）
func DeductBaseCredits(ctx *gin.Context, userName, modelHash string, amount int) error {
	tx := helpers.MysqlClientLLM.WithContext(ctx).
		Model(&UserBaseCredits{}).
		Where("user_name = ? AND model_hash = ?", userName, modelHash).
		UpdateColumn("credits", gorm.Expr("credits - ?", amount))
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}
