package skill

import (
	"encoding/json"
	"strings"
	"time"

	"react-base-service/components"
	"react-base-service/components/params"
	"react-base-service/components/route"
	"react-base-service/helpers"
	model "react-base-service/models/llm"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func CreateSkill(ctx *gin.Context, req *params.CreateSkillReq, createdBy string) (*model.Skill, error) {
	// caller_key=default 是默认作用域伪 caller（全 caller 可用），不要求真实 caller 存在。
	if !model.IsReservedCallerKey(req.CallerKey) {
		caller, err := model.GetActiveCallerByKey(ctx, req.CallerKey)
		if err != nil {
			return nil, err
		}
		if caller == nil {
			return nil, components.ErrorCallerNotFound.Sprintf(req.CallerKey)
		}
	}

	isDefault, err := resolveCreateSkillIsDefault(req.IsDefault)
	if err != nil {
		return nil, err
	}
	rv := req.RouteValues
	if rv == nil {
		rv = []string{}
	}
	routeValues, _ := json.Marshal(rv)

	if isDefault == 1 {
		existing, err := model.GetDefaultSkillByCallerAndRoute(ctx, req.CallerKey, string(routeValues))
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, components.ErrorSkillDefaultExists.Sprintf(req.CallerKey, string(routeValues))
		}
	}

	skillID := "skill_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	status, err := resolveSkillStatus(req.Status)
	if err != nil {
		return nil, err
	}

	skill := &model.Skill{
		SkillID:            skillID,
		Name:               req.Name,
		Description:        req.Description,
		TriggerCondition:   req.TriggerCondition,
		ForbiddenCondition: req.ForbiddenCondition,
		ExecutionSteps:     req.ExecutionSteps,
		BusinessContext:    req.BusinessContext,
		PromptSupplement:   req.PromptSupplement,
		CallerKey:          req.CallerKey,
		RouteValues:        string(routeValues),
		IsDefault:          isDefault,
		Status:             status,
		CreatedBy:          createdBy,
		UpdatedBy:          createdBy,
	}

	if err := model.CreateSkill(ctx, skill); err != nil {
		return nil, err
	}
	return skill, nil
}

func resolveCreateSkillIsDefault(isDefault *int) (int, error) {
	if isDefault == nil {
		return 0, nil
	}
	if *isDefault != 0 && *isDefault != 1 {
		return 0, components.ErrorParamInvalid.Sprintf("isDefault 仅支持 0 或 1")
	}
	return *isDefault, nil
}

func resolveSkillStatus(status *int) (int, error) {
	if status == nil {
		return 0, components.ErrorParamInvalid.Sprintf("status不能为空")
	}
	if *status != 0 && *status != 1 {
		return 0, components.ErrorParamInvalid.Sprintf("status 仅支持 0 或 1")
	}
	return *status, nil
}

func UpdateSkill(ctx *gin.Context, req *params.UpdateSkillReq) (*model.Skill, error) {
	existing, err := model.GetSkillBySkillID(ctx, req.SkillID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, components.ErrorSkillNotFound.Sprintf(req.SkillID)
	}

	status, err := resolveSkillStatus(req.Status)
	if err != nil {
		return nil, err
	}

	updateRv := req.RouteValues
	if updateRv == nil {
		updateRv = []string{}
	}
	rv, _ := json.Marshal(updateRv)
	updatedBy := helpers.GetUserName(ctx)
	updates := map[string]interface{}{
		"name":              req.Name,
		"description":       req.Description,
		"trigger_condition": req.TriggerCondition,
		"execution_steps":   req.ExecutionSteps,
		"business_context":  req.BusinessContext,
		"route_values":      string(rv),
		"status":            status,
		"updated_by":        updatedBy,
	}
	if req.ForbiddenCondition != nil {
		updates["forbidden_condition"] = *req.ForbiddenCondition
	}
	if req.PromptSupplement != nil {
		updates["prompt_supplement"] = *req.PromptSupplement
	}

	if err := model.UpdateSkillBySkillID(ctx, req.SkillID, updates); err != nil {
		return nil, err
	}

	updated, err := model.GetSkillBySkillID(ctx, req.SkillID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, components.ErrorSkillNotFound.Sprintf(req.SkillID)
	}
	return updated, nil
}

func DeleteSkill(ctx *gin.Context, skillID string) (*model.Skill, error) {
	existing, err := model.GetSkillBySkillID(ctx, skillID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, components.ErrorSkillNotFound.Sprintf(skillID)
	}
	updatedBy := helpers.GetUserName(ctx)
	if err := model.UpdateSkillBySkillID(ctx, skillID, map[string]interface{}{"updated_by": updatedBy}); err != nil {
		return nil, err
	}
	if err := model.SoftDeleteSkillBySkillID(ctx, skillID); err != nil {
		return nil, err
	}

	deleted, err := model.GetSkillBySkillIDUnscoped(ctx, skillID)
	if err != nil {
		return nil, err
	}
	if deleted == nil {
		return nil, components.ErrorSkillNotFound.Sprintf(skillID)
	}
	return deleted, nil
}

func GetDetail(ctx *gin.Context, skillID string) (*model.Skill, error) {
	s, err := model.GetSkillBySkillID(ctx, skillID)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, components.ErrorSkillNotFound.Sprintf(skillID)
	}
	return s, nil
}

func ListByCallerAndRoute(ctx *gin.Context, callerKey string, routeValues []string) ([]model.Skill, error) {
	// callerKey 为空：管理控制台「全部」视图，跨 caller 列出，忽略路由过滤。
	if strings.TrimSpace(callerKey) == "" {
		return model.ListAllSkills(ctx)
	}
	exact, parents := route.SplitRouteExactAndParents(routeValues)
	return model.ListSkillsByRouteWithFallback(ctx, callerKey, exact, parents)
}

// ToSkillResp 将 model.Skill 转换为 params.SkillResp
func parseSkillRouteValues(routeValuesRaw string) []string {
	var routeValues []string
	_ = json.Unmarshal([]byte(routeValuesRaw), &routeValues)
	return routeValues
}

func ToSkillResp(s *model.Skill) params.SkillResp {
	routeValues := parseSkillRouteValues(s.RouteValues)
	return params.SkillResp{
		SkillID:            s.SkillID,
		Name:               s.Name,
		Description:        s.Description,
		TriggerCondition:   s.TriggerCondition,
		ForbiddenCondition: s.ForbiddenCondition,
		ExecutionSteps:     s.ExecutionSteps,
		BusinessContext:    s.BusinessContext,
		PromptSupplement:   s.PromptSupplement,
		CallerKey:          s.CallerKey,
		RouteValues:        routeValues,
		IsDefault:          s.IsDefault,
		Status:             s.Status,
		CreatedBy:          s.CreatedBy,
		UpdatedBy:          s.UpdatedBy,
		CreatedAt:          formatTime(s.CreatedAt),
		UpdatedAt:          formatTime(s.UpdatedAt),
		DeletedAt:          formatDeletedAt(int64(s.DeletedAt)),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func formatDeletedAt(deletedAt int64) string {
	if deletedAt == 0 {
		return ""
	}
	return time.Unix(deletedAt, 0).Format("2006-01-02 15:04:05")
}
