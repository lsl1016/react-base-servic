package model

import (
	"errors"

	"react-base-service/components"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CallerConfigSnapshot 是 Caller 配置复制所需的完整资源快照。
type CallerConfigSnapshot struct {
	Skills           []Skill
	SystemPrompts    []SystemPrompt
	Tools            []Tool
	ToolUserPolicies []ToolUserPolicy
	ApiKeys          []ApiKey
}

// GetCallerByKeyUnscoped 查询 Caller，并包含软删除记录。
func GetCallerByKeyUnscoped(ctx *gin.Context, callerKey string) (*Caller, error) {
	var caller Caller
	err := GetLLMDB().WithContext(ctx).Unscoped().
		Where("caller_key = ?", callerKey).
		First(&caller).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &caller, nil
}

// GetCallerByKeyWithDB 在指定数据库会话中查询未删除 Caller。
func GetCallerByKeyWithDB(tx *gorm.DB, callerKey string) (*Caller, error) {
	var caller Caller
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("caller_key = ?", callerKey).First(&caller).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &caller, nil
}

// GetCallerByKeyUnscopedWithDB 在指定数据库会话中查询 Caller，并包含软删除记录。
func GetCallerByKeyUnscopedWithDB(tx *gorm.DB, callerKey string) (*Caller, error) {
	var caller Caller
	err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("caller_key = ?", callerKey).First(&caller).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &caller, nil
}

// LoadCallerConfigSnapshotWithDB 加载 Caller 下所有未删除配置资源。
func LoadCallerConfigSnapshotWithDB(tx *gorm.DB, callerKey string) (*CallerConfigSnapshot, error) {
	snapshot := &CallerConfigSnapshot{}
	queries := []struct {
		dest  interface{}
		where string
		args  []interface{}
	}{
		{dest: &snapshot.Skills, where: "caller_key = ?", args: []interface{}{callerKey}},
		{dest: &snapshot.SystemPrompts, where: "caller_key = ?", args: []interface{}{callerKey}},
		{dest: &snapshot.Tools, where: "caller_key = ?", args: []interface{}{callerKey}},
		{dest: &snapshot.ApiKeys, where: "caller_key = ?", args: []interface{}{callerKey}},
	}
	for _, query := range queries {
		if err := tx.Where(query.where, query.args...).Find(query.dest).Error; err != nil {
			return nil, components.ErrorDbSelect.Wrap(err)
		}
	}

	toolIDs := make([]string, 0, len(snapshot.Tools))
	for i := range snapshot.Tools {
		toolIDs = append(toolIDs, snapshot.Tools[i].ToolID)
	}
	if len(toolIDs) > 0 {
		if err := tx.Where("tool_id IN ?", toolIDs).Find(&snapshot.ToolUserPolicies).Error; err != nil {
			return nil, components.ErrorDbSelect.Wrap(err)
		}
	}
	return snapshot, nil
}

// CreateCallerConfigWithDB 在指定事务中写入目标 Caller 和全部配置资源。
func CreateCallerConfigWithDB(tx *gorm.DB, caller *Caller, snapshot *CallerConfigSnapshot) error {
	if err := tx.Create(caller).Error; err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	if len(snapshot.Skills) > 0 {
		if err := tx.Create(&snapshot.Skills).Error; err != nil {
			return components.ErrorDbInsert.Wrap(err)
		}
	}
	if len(snapshot.SystemPrompts) > 0 {
		if err := tx.Create(&snapshot.SystemPrompts).Error; err != nil {
			return components.ErrorDbInsert.Wrap(err)
		}
	}
	if len(snapshot.Tools) > 0 {
		if err := tx.Create(&snapshot.Tools).Error; err != nil {
			return components.ErrorDbInsert.Wrap(err)
		}
	}
	if len(snapshot.ToolUserPolicies) > 0 {
		if err := tx.Create(&snapshot.ToolUserPolicies).Error; err != nil {
			return components.ErrorDbInsert.Wrap(err)
		}
	}
	if len(snapshot.ApiKeys) > 0 {
		if err := tx.Create(&snapshot.ApiKeys).Error; err != nil {
			return components.ErrorDbInsert.Wrap(err)
		}
	}
	return nil
}

// SoftDeleteCallerConfigsWithDB 软删除单个 Caller 及其聚合配置，并返回各资源受影响行数。
func SoftDeleteCallerConfigsWithDB(tx *gorm.DB, callerKey string) (map[string]int64, error) {
	stats := map[string]int64{}
	var toolIDs []string
	if err := tx.Model(&Tool{}).
		Where("caller_key = ?", callerKey).
		Pluck("tool_id", &toolIDs).Error; err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}

	deletes := []struct {
		key   string
		model interface{}
		where string
		args  []interface{}
	}{
		{key: "toolUserPolicies", model: &ToolUserPolicy{}, where: "tool_id IN ?", args: []interface{}{toolIDs}},
		{key: "skills", model: &Skill{}, where: "caller_key = ?", args: []interface{}{callerKey}},
		{key: "systemPrompts", model: &SystemPrompt{}, where: "caller_key = ?", args: []interface{}{callerKey}},
		{key: "tools", model: &Tool{}, where: "caller_key = ?", args: []interface{}{callerKey}},
		{key: "apiKeys", model: &ApiKey{}, where: "caller_key = ?", args: []interface{}{callerKey}},
		{key: "mcpServerCallers", model: &McpServerCaller{}, where: "caller_key = ?", args: []interface{}{callerKey}},
		{key: "callers", model: &Caller{}, where: "caller_key = ?", args: []interface{}{callerKey}},
	}
	for _, item := range deletes {
		if item.key == "toolUserPolicies" && len(toolIDs) == 0 {
			stats[item.key] = 0
			continue
		}
		result := tx.Where(item.where, item.args...).Delete(item.model)
		if result.Error != nil {
			return nil, components.ErrorDbUpdate.Wrap(result.Error)
		}
		stats[item.key] = result.RowsAffected
	}
	return stats, nil
}
