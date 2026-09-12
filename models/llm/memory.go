package model

import (
	"errors"
	"time"

	"react-base-service/components"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	// MemoryOwnerTypeCaller 记忆归属为 caller 维度：该调用方全部会话可见。
	MemoryOwnerTypeCaller = "caller"
	// MemoryOwnerTypeCallerUser 记忆归属为 caller+user 维度：仅该调用方下该用户的会话可见。
	MemoryOwnerTypeCallerUser = "caller_user"

	// MemoryLayerResident 常驻层：随 system 前缀全文注入。
	MemoryLayerResident = "resident"
	// MemoryLayerDetached 按需层：只注入目录索引，正文经 memory_read 按需读取。
	MemoryLayerDetached = "detached"

	MemoryStateActive  = "active"
	MemoryStateDeleted = "deleted"

	MemorySourceModel      = "model"
	MemorySourceReflection = "reflection"
	MemorySourceAdmin      = "admin"
)

// MemoryOwner 描述一个记忆归属维度；(OwnerType, OwnerKey) 唯一确定一个记忆空间。
type MemoryOwner struct {
	OwnerType string
	OwnerKey  string
}

// BuildCallerMemoryOwner 构造 caller 维度记忆归属。
func BuildCallerMemoryOwner(callerKey string) MemoryOwner {
	return MemoryOwner{OwnerType: MemoryOwnerTypeCaller, OwnerKey: callerKey}
}

// BuildCallerUserMemoryOwner 构造 caller+user 维度记忆归属。
func BuildCallerUserMemoryOwner(callerKey, userName string) MemoryOwner {
	return MemoryOwner{OwnerType: MemoryOwnerTypeCallerUser, OwnerKey: callerKey + "|" + userName}
}

// MemoryItem 长期记忆条目：一条原子事实，逻辑唯一键 (owner_type, owner_key, item_key)。
type MemoryItem struct {
	ID          uint      `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	OwnerType   string    `json:"ownerType" gorm:"column:owner_type;not null"`
	OwnerKey    string    `json:"ownerKey" gorm:"column:owner_key;not null"`
	Layer       string    `json:"layer" gorm:"column:layer;not null;default:'detached'"`
	Title       string    `json:"title" gorm:"column:title;not null;default:''"`
	Content     string    `json:"content" gorm:"column:content;not null"`
	Description string    `json:"description" gorm:"column:description;not null;default:''"`
	Tags        string    `json:"tags" gorm:"column:tags;not null;default:''"`
	Source      string    `json:"source" gorm:"column:source;not null;default:'model'"`
	ItemKey     string    `json:"itemKey" gorm:"column:item_key;not null"`
	Version     int       `json:"version" gorm:"column:version;not null;default:1"`
	State       string    `json:"state" gorm:"column:state;not null;default:'active'"`
	LastReason  string    `json:"lastReason" gorm:"column:last_reason;not null;default:''"`
	CreatedBy   string    `json:"createdBy" gorm:"column:created_by;not null;default:''"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (m *MemoryItem) TableName() string {
	return "tblLlmMemoryItem"
}

// MemoryRevision 记忆修订流水：不可变，只插不改；回滚 = 用旧快照反向提交新修订。
type MemoryRevision struct {
	ID         uint      `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	ItemID     uint      `json:"itemId" gorm:"column:item_id;not null"`
	Action     string    `json:"action" gorm:"column:action;not null"`
	BeforeJSON string    `json:"beforeJSON" gorm:"column:before_json"`
	AfterJSON  string    `json:"afterJSON" gorm:"column:after_json"`
	Reason     string    `json:"reason" gorm:"column:reason;not null"`
	Source     string    `json:"source" gorm:"column:source;not null"`
	CreatedBy  string    `json:"createdBy" gorm:"column:created_by;not null"`
	CreatedAt  time.Time `json:"createdAt" gorm:"column:created_at"`
}

func (r *MemoryRevision) TableName() string {
	return "tblLlmMemoryRevision"
}

// FindActiveMemoryItemsByOwners 查询一组记忆空间下的全部 active 条目，按更新时间倒序。
func FindActiveMemoryItemsByOwners(ctx *gin.Context, owners []MemoryOwner) ([]MemoryItem, error) {
	if len(owners) == 0 {
		return []MemoryItem{}, nil
	}
	db := helpers.MysqlClientLLM.Model(&MemoryItem{}).WithContext(ctx)
	query := db.Where("state = ?", MemoryStateActive)
	for _, owner := range owners {
		query = query.Or("owner_type = ? AND owner_key = ?", owner.OwnerType, owner.OwnerKey)
	}
	var items []MemoryItem
	err := query.Order("updated_at DESC").Find(&items).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return items, nil
}

// GetActiveMemoryItemByID 按 ID 查 active 条目；不存在返回 nil。
func GetActiveMemoryItemByID(ctx *gin.Context, id uint) (*MemoryItem, error) {
	var item MemoryItem
	err := helpers.MysqlClientLLM.Model(&MemoryItem{}).WithContext(ctx).
		Where("id = ? AND state = ?", id, MemoryStateActive).First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &item, nil
}

// FindMemoryItemByOwnerAndKeyWithDB 在指定事务内按逻辑唯一键查条目（含已软删，供幂等收敛/复活）。
func FindMemoryItemByOwnerAndKeyWithDB(ctx *gin.Context, tx *gorm.DB, owner MemoryOwner, itemKey string) (*MemoryItem, error) {
	var item MemoryItem
	err := tx.Model(&MemoryItem{}).WithContext(ctx).
		Where("owner_type = ? AND owner_key = ? AND item_key = ?", owner.OwnerType, owner.OwnerKey, itemKey).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &item, nil
}

// CountActiveMemoryItemsWithDB 统计指定记忆空间某层的 active 条目数，用于常驻/按需上限守门。
func CountActiveMemoryItemsWithDB(ctx *gin.Context, tx *gorm.DB, owner MemoryOwner, layer string) (int64, error) {
	var count int64
	query := tx.Model(&MemoryItem{}).WithContext(ctx).
		Where("owner_type = ? AND owner_key = ? AND state = ?", owner.OwnerType, owner.OwnerKey, MemoryStateActive)
	if layer != "" {
		query = query.Where("layer = ?", layer)
	}
	err := query.Count(&count).Error
	if err != nil {
		return 0, components.ErrorDbSelect.Wrap(err)
	}
	return count, nil
}

// CreateMemoryItemWithDB 在指定事务内创建记忆条目。
func CreateMemoryItemWithDB(ctx *gin.Context, tx *gorm.DB, item *MemoryItem) error {
	err := tx.Model(&MemoryItem{}).WithContext(ctx).Create(item).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

// UpdateMemoryItemWithVersionWithDB 带乐观锁更新条目；expectedVersion>0 时必须与当前版本一致，否则返回 false。
// 更新恒定递增 version，由 updates 显式传入。
func UpdateMemoryItemWithVersionWithDB(ctx *gin.Context, tx *gorm.DB, id uint, expectedVersion int, updates map[string]interface{}) (bool, error) {
	query := tx.Model(&MemoryItem{}).WithContext(ctx).Where("id = ?", id)
	if expectedVersion > 0 {
		query = query.Where("version = ?", expectedVersion)
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return false, components.ErrorDbUpdate.Wrap(result.Error)
	}
	return result.RowsAffected > 0, nil
}

// CreateMemoryRevisionWithDB 在指定事务内追加修订流水。
func CreateMemoryRevisionWithDB(ctx *gin.Context, tx *gorm.DB, revision *MemoryRevision) error {
	err := tx.Model(&MemoryRevision{}).WithContext(ctx).Create(revision).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

// FindMemoryRevisionsByItemID 查询条目的修订历史（正序），供管理面与排障使用。
func FindMemoryRevisionsByItemID(ctx *gin.Context, itemID uint) ([]MemoryRevision, error) {
	var revisions []MemoryRevision
	err := helpers.MysqlClientLLM.Model(&MemoryRevision{}).WithContext(ctx).
		Where("item_id = ?", itemID).Order("id ASC").Find(&revisions).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return revisions, nil
}
