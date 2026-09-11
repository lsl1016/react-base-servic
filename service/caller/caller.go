package caller

import (
	"encoding/json"
	"strings"

	"react-base-service/components"
	model "react-base-service/models/llm"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RegisterCaller(ctx *gin.Context, callerKey, name, description, platform string, createdBy string) (*model.Caller, error) {
	existing, err := model.GetCallerByKeyUnscoped(ctx, callerKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, components.ErrorCallerDuplicate.Sprintf(callerKey)
	}

	caller := &model.Caller{
		CallerKey:   callerKey,
		Name:        name,
		Description: description,
		Platform:    platform,
		Status:      1,
		CreatedBy:   createdBy,
	}
	if err := model.CreateCaller(ctx, caller); err != nil {
		return nil, err
	}

	// 自动创建 caller 级兜底 skill
	//if err := createDefaultSkill(ctx, callerKey, createdBy); err != nil {
	//	return nil, components.ErrorSkillCreateFailed.Sprintf("创建兜底skill失败: " + err.Error())
	//}

	return caller, nil
}

func createDefaultSkill(ctx *gin.Context, callerKey, createdBy string) error {
	skillID := "skill_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	routeValues, _ := json.Marshal([]string{})

	skill := &model.Skill{
		SkillID:          skillID,
		Name:             callerKey + "-默认对话",
		Description:      "通用对话技能，当用户问题不匹配任何具体技能时使用",
		TriggerCondition: "用户的问题不属于任何具体技能的范围",
		ExecutionSteps:   "直接基于上下文回答用户问题",
		CallerKey:        callerKey,
		RouteValues:      string(routeValues),
		IsDefault:        1,
		Status:           1,
		CreatedBy:        createdBy,
	}
	return model.CreateSkill(ctx, skill)
}

func GetByKey(ctx *gin.Context, callerKey string) (*model.Caller, error) {
	caller, err := model.GetActiveCallerByKey(ctx, callerKey)
	if err != nil {
		return nil, err
	}
	if caller == nil {
		return nil, components.ErrorCallerNotFound.Sprintf(callerKey)
	}
	return caller, nil
}

func UpdateCaller(ctx *gin.Context, callerKey string, updates map[string]interface{}) error {
	existing, err := model.GetCallerByKey(ctx, callerKey)
	if err != nil {
		return err
	}
	if existing == nil {
		return components.ErrorCallerNotFound.Sprintf(callerKey)
	}
	return model.UpdateCallerByKey(ctx, callerKey, updates)
}

func ListAll(ctx *gin.Context) ([]model.Caller, error) {
	return model.ListCallers(ctx)
}
