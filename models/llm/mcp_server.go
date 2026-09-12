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

// McpServer 是 MCP 连接注册表（tblLlmMcpServer）：
// 记录通过管理接口登记的 MCP 服务器（http/http_sdk/repo），
// 启动时按记录拉起客户端并把工具清单同步进 tblLlmTool（tool_type=mcp）。
type McpServer struct {
	ID          uint   `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	ServerID    string `json:"serverId" gorm:"column:server_id;not null"`
	Name        string `json:"name" gorm:"column:name;not null"`
	Kind        string `json:"kind" gorm:"column:kind;not null;default:http"`
	Endpoint    string `json:"endpoint" gorm:"column:endpoint;not null;default:''"`
	Headers     string `json:"headers" gorm:"column:headers"`
	Env         string `json:"env" gorm:"column:env"`
	TimeoutMs   int    `json:"timeoutMs" gorm:"column:timeout_ms;not null;default:30000"`
	Description string `json:"description" gorm:"column:description"`
	Status      int    `json:"status" gorm:"column:status;not null;default:1"`
	// LastCheckStatus: connected=最近一次连接成功；disconnected=最近一次失败；unknown=尚未检测
	LastCheckStatus  string                `json:"lastCheckStatus" gorm:"column:last_check_status;not null;default:unknown"`
	LastCheckMessage string                `json:"lastCheckMessage" gorm:"column:last_check_message;not null;default:''"`
	LastCheckAt      *time.Time            `json:"lastCheckAt" gorm:"column:last_check_at"`
	CallerKey        string                `json:"callerKey" gorm:"column:caller_key;not null"`
	CreatedBy        string                `json:"createdBy" gorm:"column:created_by;not null;default:''"`
	UpdatedBy        string                `json:"updatedBy" gorm:"column:updated_by;not null;default:''"`
	CreatedAt        time.Time             `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt        time.Time             `json:"updatedAt" gorm:"column:updated_at"`
	DeletedAt        soft_delete.DeletedAt `json:"deletedAt" gorm:"column:deleted_at;not null;default:0"`
}

func (s *McpServer) TableName() string {
	return "tblLlmMcpServer"
}

func CreateMcpServer(ctx *gin.Context, server *McpServer) error {
	err := helpers.MysqlClientLLM.Model(&McpServer{}).WithContext(ctx).Create(server).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

// GetMcpServerByServerID 按 serverId 查询单条记录（含停用，不含已删除）。
func GetMcpServerByServerID(ctx *gin.Context, serverID string) (*McpServer, error) {
	var server McpServer
	err := helpers.MysqlClientLLM.Model(&McpServer{}).WithContext(ctx).
		Where("server_id = ?", serverID).First(&server).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &server, nil
}

// GetMcpServerByName 按服务器名查询（名字全局唯一，用于创建时查重）。
func GetMcpServerByName(ctx *gin.Context, name string) (*McpServer, error) {
	var server McpServer
	err := helpers.MysqlClientLLM.Model(&McpServer{}).WithContext(ctx).
		Where("name = ?", name).First(&server).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &server, nil
}

// GetMcpServerByNameUnscoped 含软删除记录按名查询：软删除行仍占用 uk_name 唯一键，
// 同名重建时需要复活该行而非重复插入。
func GetMcpServerByNameUnscoped(ctx *gin.Context, name string) (*McpServer, error) {
	var server McpServer
	err := helpers.MysqlClientLLM.Unscoped().Model(&McpServer{}).WithContext(ctx).
		Where("name = ?", name).First(&server).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &server, nil
}

// ReviveMcpServerByName 复活软删除的连接记录：按名覆盖全部业务字段并清除删除标记。
func ReviveMcpServerByName(ctx *gin.Context, name string, updates map[string]interface{}) error {
	tx := helpers.MysqlClientLLM.Unscoped().Model(&McpServer{}).WithContext(ctx).
		Where("name = ?", name).
		Updates(updates)
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func UpdateMcpServerByServerID(ctx *gin.Context, serverID string, updates map[string]interface{}) error {
	tx := helpers.MysqlClientLLM.Model(&McpServer{}).WithContext(ctx).
		Where("server_id = ?", serverID).
		Updates(updates)
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func SoftDeleteMcpServerByServerID(ctx *gin.Context, serverID string) error {
	tx := helpers.MysqlClientLLM.WithContext(ctx).Where("server_id = ?", serverID).Delete(&McpServer{})
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

// ListMcpServersByCaller 返回指定 caller 下的全部 MCP 连接（含停用）。
func ListMcpServersByCaller(ctx *gin.Context, callerKey string) ([]McpServer, error) {
	var servers []McpServer
	err := helpers.MysqlClientLLM.Model(&McpServer{}).WithContext(ctx).
		Where("caller_key = ?", callerKey).
		Order("created_at DESC").
		Find(&servers).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return servers, nil
}

// ListEnabledMcpServers 返回全部启用的 MCP 连接（跨 caller，启动拉起用）。
func ListEnabledMcpServers(ctx *gin.Context) ([]McpServer, error) {
	var servers []McpServer
	err := helpers.MysqlClientLLM.Model(&McpServer{}).WithContext(ctx).
		Where("status = 1").
		Find(&servers).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return servers, nil
}

// McpServerCaller 是 MCP 连接与 caller 的绑定（tblLlmMcpServerCaller）：
// 一个连接可绑多个 caller，连接的工具会同步到每个绑定的 caller 名下；
// 连接表自带的 caller_key 是属主 caller（登记方，恒定生效，不占绑定行）。
type McpServerCaller struct {
	ID        uint                  `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	ServerID  string                `json:"serverId" gorm:"column:server_id;not null"`
	CallerKey string                `json:"callerKey" gorm:"column:caller_key;not null"`
	Status    int                   `json:"status" gorm:"column:status;not null;default:1"`
	CreatedBy string                `json:"createdBy" gorm:"column:created_by;not null;default:''"`
	CreatedAt time.Time             `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt time.Time             `json:"updatedAt" gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `json:"deletedAt" gorm:"column:deleted_at;not null;default:0"`
}

func (b *McpServerCaller) TableName() string {
	return "tblLlmMcpServerCaller"
}

// ListMcpServerCallers 返回连接当前生效的绑定 caller（不含属主，属主恒定生效）。
func ListMcpServerCallers(ctx *gin.Context, serverID string) ([]string, error) {
	var rows []McpServerCaller
	err := helpers.MysqlClientLLM.Model(&McpServerCaller{}).WithContext(ctx).
		Where("server_id = ? AND status = 1", serverID).
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	callers := make([]string, 0, len(rows))
	for _, row := range rows {
		callers = append(callers, row.CallerKey)
	}
	return callers, nil
}

// ReplaceMcpServerCallers 全量替换连接的绑定 caller（软删除缺席行、复活/新增传入行）。
func ReplaceMcpServerCallers(ctx *gin.Context, serverID string, callerKeys []string, updatedBy string) error {
	return helpers.MysqlClientLLM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []McpServerCaller
		if err := tx.Model(&McpServerCaller{}).Where("server_id = ?", serverID).Find(&existing).Error; err != nil {
			return components.ErrorDbSelect.Wrap(err)
		}
		active := make(map[string]bool, len(callerKeys))
		for _, key := range callerKeys {
			active[key] = true
		}
		for _, key := range callerKeys {
			if active[key] {
				// 先尝试复活（唯一键 uk_server_caller 可能仍被软删除行占用），未命中再新增。
				result := tx.Unscoped().Model(&McpServerCaller{}).
					Where("server_id = ? AND caller_key = ?", serverID, key).
					Updates(map[string]interface{}{"status": 1, "created_by": updatedBy, "deleted_at": 0})
				if result.Error != nil {
					return components.ErrorDbUpdate.Wrap(result.Error)
				}
				if result.RowsAffected == 0 {
					if err := tx.Create(&McpServerCaller{ServerID: serverID, CallerKey: key, Status: 1, CreatedBy: updatedBy}).Error; err != nil {
						return components.ErrorDbInsert.Wrap(err)
					}
				}
				active[key] = false // 已处理，避免重复
			}
		}
		for _, row := range existing {
			if row.DeletedAt == 0 && !containsString(callerKeys, row.CallerKey) {
				if err := tx.Where("server_id = ? AND caller_key = ?", serverID, row.CallerKey).
					Delete(&McpServerCaller{}).Error; err != nil {
					return components.ErrorDbUpdate.Wrap(err)
				}
			}
		}
		return nil
	})
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// SoftDeleteMcpServerCallersByCallerWithDB 删除 caller 时的级联：清理该 caller 的全部连接绑定。
func SoftDeleteMcpServerCallersByCallerWithDB(tx *gorm.DB, callerKey string) (int64, error) {
	result := tx.Where("caller_key = ?", callerKey).Delete(&McpServerCaller{})
	if result.Error != nil {
		return 0, components.ErrorDbUpdate.Wrap(result.Error)
	}
	return result.RowsAffected, nil
}
