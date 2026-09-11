package model

import (
	"react-base-service/helpers"

	"gorm.io/gorm"
)

// GetLLMDB 返回 LLM 数据库连接，供需要手动管理事务的 service 层使用
func GetLLMDB() *gorm.DB {
	return helpers.MysqlClientLLM
}
