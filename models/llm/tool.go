package model

import (
	"encoding/json"
	"errors"
	"time"

	"react-base-service/components"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

type Tool struct {
	ID          uint                  `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	ToolID      string                `json:"toolId" gorm:"column:tool_id;not null"`
	Name        string                `json:"name" gorm:"column:name;not null"`
	Description string                `json:"description" gorm:"column:description"`
	ToolType    string                `json:"toolType" gorm:"column:tool_type;not null"`
	CallerKey   string                `json:"callerKey" gorm:"column:caller_key;not null"`
	RouteValues string                `json:"routeValues" gorm:"column:route_values"`
	Config      string                `json:"config" gorm:"column:config"`
	Status      int                   `json:"status" gorm:"column:status;not null;default:1"`
	CreatedBy   string                `json:"createdBy" gorm:"column:created_by;not null;default:''"`
	UpdatedBy   string                `json:"updatedBy" gorm:"column:updated_by;not null;default:''"`
	CreatedAt   time.Time             `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time             `json:"updatedAt" gorm:"column:updated_at"`
	DeletedAt   soft_delete.DeletedAt `json:"deletedAt" gorm:"column:deleted_at;not null;default:0"`
}

func (t *Tool) TableName() string {
	return "tblLlmTool"
}

func CreateTool(ctx *gin.Context, tool *Tool) error {
	err := helpers.MysqlClientLLM.Model(&Tool{}).WithContext(ctx).Create(tool).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

// ExistToolByCallerAndName 检查同一 caller 下是否存在同名 tool
func ExistToolByCallerAndName(ctx *gin.Context, callerKey, name string) (bool, error) {
	var count int64
	err := helpers.MysqlClientLLM.Model(&Tool{}).WithContext(ctx).
		Where("caller_key = ? AND name = ?", callerKey, name).
		Count(&count).Error
	if err != nil {
		return false, components.ErrorDbSelect.Wrap(err)
	}
	return count > 0, nil
}

func GetToolByToolID(ctx *gin.Context, toolID string) (*Tool, error) {
	var tool Tool
	err := helpers.MysqlClientLLM.Model(&Tool{}).WithContext(ctx).
		Where("tool_id = ?", toolID).First(&tool).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &tool, nil
}

// GetToolByToolIDUnscoped 含软删除记录查询（MCP 连接重新启用时恢复同步用：
// 软删除行仍占用 uk_tool_id 唯一键，只能更新恢复，不能重复插入）。
func GetToolByToolIDUnscoped(ctx *gin.Context, toolID string) (*Tool, error) {
	var tool Tool
	err := helpers.MysqlClientLLM.Unscoped().Model(&Tool{}).WithContext(ctx).
		Where("tool_id = ?", toolID).First(&tool).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &tool, nil
}

func UpdateToolByToolID(ctx *gin.Context, toolID string, updates map[string]interface{}) error {
	tx := helpers.MysqlClientLLM.Model(&Tool{}).WithContext(ctx).
		Where("tool_id = ?", toolID).
		Updates(updates)
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

// UpdateToolByToolIDUnscoped 含软删除行的更新（恢复软删除的 MCP 注册工具用）。
func UpdateToolByToolIDUnscoped(ctx *gin.Context, toolID string, updates map[string]interface{}) error {
	tx := helpers.MysqlClientLLM.Unscoped().Model(&Tool{}).WithContext(ctx).
		Where("tool_id = ?", toolID).
		Updates(updates)
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func SoftDeleteToolByToolID(ctx *gin.Context, toolID string) error {
	tx := helpers.MysqlClientLLM.WithContext(ctx).Where("tool_id = ?", toolID).Delete(&Tool{})
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func ListToolsByCaller(ctx *gin.Context, callerKey string) ([]Tool, error) {
	var tools []Tool
	err := helpers.MysqlClientLLM.Model(&Tool{}).WithContext(ctx).
		Where("caller_key = ?", callerKey).
		Order("created_at DESC").
		Find(&tools).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return tools, nil
}

// ListToolsByCallerAndExactRoute 按 callerKey + routeValues 精确匹配查询 tool 列表
func ListToolsByCallerAndExactRoute(ctx *gin.Context, callerKey string, routeValues string) ([]Tool, error) {
	var tools []Tool
	err := helpers.MysqlClientLLM.Model(&Tool{}).WithContext(ctx).
		Where("caller_key = ? AND route_values = ?", callerKey, routeValues).
		Order("created_at DESC").
		Find(&tools).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return tools, nil
}

// ListToolsByType 按工具类型查询全部工具（含停用；MCP 注册表清理/统计用）。
func ListToolsByType(ctx *gin.Context, toolType string) ([]Tool, error) {
	var tools []Tool
	err := helpers.MysqlClientLLM.Model(&Tool{}).WithContext(ctx).
		Where("tool_type = ?", toolType).
		Find(&tools).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return tools, nil
}

// ListMCPServerTools 返回指定 MCP 服务器同步进注册表的工具（按 config.mcpServer 匹配）。
func ListMCPServerTools(ctx *gin.Context, serverName string) ([]Tool, error) {
	tools, err := ListToolsByType(ctx, "mcp")
	if err != nil {
		return nil, err
	}
	matched := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		var cfg struct {
			MCPServer string `json:"mcpServer"`
		}
		if err := json.Unmarshal([]byte(tool.Config), &cfg); err != nil || cfg.MCPServer != serverName {
			continue
		}
		matched = append(matched, tool)
	}
	return matched, nil
}

// FindToolsByCallerAndRoutes 按 callerKey + 路由前缀匹配查询 tool；
// 同时并入「默认作用域」（caller_key=default）下命中的工具，对全部 caller 生效。
func FindToolsByCallerAndRoutes(ctx *gin.Context, callerKey string, routePrefixes []string) ([]Tool, error) {
	var tools []Tool
	err := helpers.MysqlClientLLM.Model(&Tool{}).WithContext(ctx).
		Where("caller_key IN ? AND status = 1 AND route_values IN ?", CallerScopeKeys(callerKey), routePrefixes).
		Find(&tools).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return tools, nil
}

// ListAllTools 列出全部 caller 的工具（管理控制台「全部」视图用）。
func ListAllTools(ctx *gin.Context) ([]Tool, error) {
	var tools []Tool
	err := helpers.MysqlClientLLM.Model(&Tool{}).WithContext(ctx).
		Order("caller_key ASC, created_at DESC").
		Find(&tools).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return tools, nil
}
