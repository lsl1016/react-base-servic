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

type Skill struct {
	ID                 uint                  `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	SkillID            string                `json:"skillId" gorm:"column:skill_id;not null"`
	Name               string                `json:"name" gorm:"column:name;not null"`
	Description        string                `json:"description" gorm:"column:description"`
	TriggerCondition   string                `json:"triggerCondition" gorm:"column:trigger_condition"`
	ForbiddenCondition string                `json:"forbiddenCondition" gorm:"column:forbidden_condition"`
	ExecutionSteps     string                `json:"executionSteps" gorm:"column:execution_steps"`
	BusinessContext    string                `json:"businessContext" gorm:"column:business_context"`
	PromptSupplement   string                `json:"promptSupplement" gorm:"column:prompt_supplement"`
	// TriggersJSON 是结构化关键词触发器（JSON 字符串数组，P2-2）：
	// run 装配期对用户消息做确定性子串匹配，命中时把 skill 提示追加到该条用户消息。
	// 与 TriggerCondition（给模型读的纯文本语义提示）并存，职责不同。
	TriggersJSON string `json:"triggersJson" gorm:"column:triggers_json"`
	// Content 是 SKILL.md 正文（文件导入形态）；get_skill 全量返回时随模型注入。
	Content     string                `json:"content" gorm:"column:content"`
	CallerKey   string                `json:"callerKey" gorm:"column:caller_key;not null"`
	RouteValues        string                `json:"routeValues" gorm:"column:route_values"`
	IsDefault          int                   `json:"isDefault" gorm:"column:is_default;not null;default:0"`
	Status             int                   `json:"status" gorm:"column:status;not null;default:1"`
	CreatedBy          string                `json:"createdBy" gorm:"column:created_by;not null;default:''"`
	UpdatedBy          string                `json:"updatedBy" gorm:"column:updated_by;not null;default:''"`
	CreatedAt          time.Time             `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt          time.Time             `json:"updatedAt" gorm:"column:updated_at"`
	DeletedAt          soft_delete.DeletedAt `json:"deletedAt" gorm:"column:deleted_at;not null;default:0"`
}

func (s *Skill) TableName() string {
	return "tblLlmSkill"
}

func CreateSkill(ctx *gin.Context, skill *Skill) error {
	err := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).Create(skill).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

func GetSkillBySkillID(ctx *gin.Context, skillID string) (*Skill, error) {
	var skill Skill
	err := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).
		Where("skill_id = ?", skillID).First(&skill).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &skill, nil
}

// FindActiveSkillByCallerAndName 按 caller+name 查未软删的 Skill（含禁用行），
// 供 SKILL.md 导入的同名覆盖（upsert）判定：导入是权威定义，命中即更新而非重复新建。
func FindActiveSkillByCallerAndName(ctx *gin.Context, callerKey, name string) (*Skill, error) {
	var skill Skill
	err := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).
		Where("caller_key = ? AND name = ?", callerKey, name).First(&skill).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &skill, nil
}

func GetSkillBySkillIDUnscoped(ctx *gin.Context, skillID string) (*Skill, error) {
	var skill Skill
	err := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).
		Unscoped().
		Where("skill_id = ?", skillID).First(&skill).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &skill, nil
}

func UpdateSkillBySkillID(ctx *gin.Context, skillID string, updates map[string]interface{}) error {
	tx := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).
		Where("skill_id = ?", skillID).
		Updates(updates)
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func SoftDeleteSkillBySkillID(ctx *gin.Context, skillID string) error {
	tx := helpers.MysqlClientLLM.WithContext(ctx).Where("skill_id = ?", skillID).Delete(&Skill{})
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func ListSkillsByCaller(ctx *gin.Context, callerKey string) ([]Skill, error) {
	var skills []Skill
	err := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).
		Where("caller_key = ?", callerKey).
		Order("is_default ASC, created_at DESC").
		Find(&skills).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return skills, nil
}

// ListSkillsByCallerAndRoutes 按 callerKey + 路由前缀匹配查询 skill 列表，保留 list 接口的全量状态语义
func ListSkillsByCallerAndRoutes(ctx *gin.Context, callerKey string, routePrefixes []string) ([]Skill, error) {
	var skills []Skill
	err := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).
		Where("caller_key = ? AND route_values IN ?", callerKey, routePrefixes).
		Order("is_default ASC, created_at DESC").
		Find(&skills).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return skills, nil
}

// ListSkillsByRouteWithFallback 按精确路由返回全部 skill + 上级路由仅返回 isDefault=1 的 skill
// exactRoute: 精确匹配的路由值 JSON 字符串，如 '["s_xxx","r_xxx"]'
// parentPrefixes: 上级路由前缀列表，如 ['[]', '["s_xxx"]']
func ListSkillsByRouteWithFallback(ctx *gin.Context, callerKey string, exactRoute string, parentPrefixes []string) ([]Skill, error) {
	var skills []Skill
	db := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).
		Where("caller_key = ?", callerKey)

	if len(parentPrefixes) > 0 {
		db = db.Where("(route_values = ?) OR (route_values IN ? AND is_default = 1)", exactRoute, parentPrefixes)
	} else {
		db = db.Where("route_values = ?", exactRoute)
	}

	err := db.Order("is_default ASC, created_at DESC").Find(&skills).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return skills, nil
}

// FindSkillsByCallerAndRoutes 按 callerKey + 路由前缀匹配查询 skill；
// 同时并入「默认作用域」（caller_key=default）下命中的 skill，对全部 caller 生效。
func FindSkillsByCallerAndRoutes(ctx *gin.Context, callerKey string, routePrefixes []string) ([]Skill, error) {
	var skills []Skill
	err := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).
		Where("caller_key IN ? AND status = 1 AND route_values IN ?", CallerScopeKeys(callerKey), routePrefixes).
		Order("is_default ASC").
		Find(&skills).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return skills, nil
}

// ListAllSkills 列出全部 caller 的 skill（管理控制台「全部」视图用）。
func ListAllSkills(ctx *gin.Context) ([]Skill, error) {
	var skills []Skill
	err := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).
		Order("caller_key ASC, created_at DESC").
		Find(&skills).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return skills, nil
}

// GetDefaultSkillByCaller 获取 caller 级兜底 skill
func GetDefaultSkillByCaller(ctx *gin.Context, callerKey string) (*Skill, error) {
	var skill Skill
	err := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).
		Where("caller_key = ? AND is_default = 1 AND status = 1", callerKey).
		First(&skill).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &skill, nil
}

// GetDefaultSkillByCallerAndRoute 按 callerKey + routeValues 查询兜底 skill（不过滤 status，用于唯一性校验）
func GetDefaultSkillByCallerAndRoute(ctx *gin.Context, callerKey, routeValues string) (*Skill, error) {
	var skill Skill
	err := helpers.MysqlClientLLM.Model(&Skill{}).WithContext(ctx).
		Where("caller_key = ? AND route_values = ? AND is_default = 1", callerKey, routeValues).
		First(&skill).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &skill, nil
}
