package tool

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"react-base-service/components"
	"react-base-service/components/params"
	model "react-base-service/models/llm"

	"github.com/gin-gonic/gin"
)

const (
	maxToolUserPolicyUsers   = 200
	maxToolUserPolicyToolIDs = 200
	maxToolUserNameLength    = 128
)

func normalizeToolUserList(userNames []string) ([]string, error) {
	unique := make(map[string]struct{}, len(userNames))
	for _, userName := range userNames {
		userName = strings.TrimSpace(userName)
		if userName == "" {
			continue
		}
		if len([]rune(userName)) > maxToolUserNameLength {
			return nil, components.ParamInvalidf("用户名不能超过 %d 个字符", maxToolUserNameLength)
		}
		unique[userName] = struct{}{}
	}
	if len(unique) == 0 {
		return nil, components.ParamInvalidf("whiteUserList 不能为空")
	}
	if len(unique) > maxToolUserPolicyUsers {
		return nil, components.ParamInvalidf("whiteUserList 最多包含 %d 个用户", maxToolUserPolicyUsers)
	}

	normalized := make([]string, 0, len(unique))
	for userName := range unique {
		normalized = append(normalized, userName)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func normalizeToolPolicyIDs(toolIDs []string) ([]string, error) {
	if len(toolIDs) == 0 {
		return nil, components.ParamInvalidf("toolIds 不能为空")
	}

	unique := make(map[string]struct{}, len(toolIDs))
	for _, toolID := range toolIDs {
		toolID = strings.TrimSpace(toolID)
		if toolID != "" {
			unique[toolID] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return nil, components.ParamInvalidf("toolIds 不能为空")
	}
	if len(unique) > maxToolUserPolicyToolIDs {
		return nil, components.ParamInvalidf("toolIds 最多包含 %d 个工具", maxToolUserPolicyToolIDs)
	}

	normalized := make([]string, 0, len(unique))
	for toolID := range unique {
		normalized = append(normalized, toolID)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func validateToolPolicyTarget(ctx *gin.Context, toolID string) (*model.Tool, error) {
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return nil, components.ParamInvalidf("toolId 不能为空")
	}
	t, err := model.GetToolByToolID(ctx, toolID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, components.ErrorToolNotFound.Sprintf(toolID)
	}
	if t.Status != 0 {
		return nil, components.ErrorToolUserPolicyRequiresDisabled.Sprintf(toolID)
	}
	return t, nil
}

func CreateToolUserPolicy(ctx *gin.Context, req *params.CreateToolUserPolicyReq, operator string) (*model.ToolUserPolicy, error) {
	t, err := validateToolPolicyTarget(ctx, req.ToolID)
	if err != nil {
		return nil, err
	}
	whiteUserList, err := normalizeToolUserList(req.WhiteUserList)
	if err != nil {
		return nil, err
	}
	whiteUserListJSON, err := json.Marshal(whiteUserList)
	if err != nil {
		return nil, components.ErrorToolUserPolicyInvalid.Sprintf(t.ToolID)
	}

	operator = strings.TrimSpace(operator)
	policy := &model.ToolUserPolicy{
		ToolID:        t.ToolID,
		WhiteUserList: string(whiteUserListJSON),
		BlackUserList: "[]",
		CreatedBy:     operator,
		UpdatedBy:     operator,
	}
	if err := model.CreateOrRestoreToolUserPolicy(ctx, policy); err != nil {
		if errors.Is(err, model.ErrToolUserPolicyAlreadyExists) {
			return nil, components.ErrorToolUserPolicyDuplicate.Sprintf(t.ToolID)
		}
		return nil, err
	}
	return getToolUserPolicy(ctx, t.ToolID)
}

func UpdateToolUserPolicy(ctx *gin.Context, req *params.UpdateToolUserPolicyReq, operator string) (*model.ToolUserPolicy, error) {
	t, err := validateToolPolicyTarget(ctx, req.ToolID)
	if err != nil {
		return nil, err
	}
	whiteUserList, err := normalizeToolUserList(req.WhiteUserList)
	if err != nil {
		return nil, err
	}
	whiteUserListJSON, err := json.Marshal(whiteUserList)
	if err != nil {
		return nil, components.ErrorToolUserPolicyInvalid.Sprintf(t.ToolID)
	}

	updated, err := model.UpdateToolUserPolicyByToolID(ctx, t.ToolID, map[string]interface{}{
		"white_user_list": string(whiteUserListJSON),
		"updated_by":      strings.TrimSpace(operator),
	})
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, components.ErrorToolUserPolicyNotFound.Sprintf(t.ToolID)
	}
	return getToolUserPolicy(ctx, t.ToolID)
}

func DeleteToolUserPolicy(ctx *gin.Context, toolID, operator string) error {
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return components.ParamInvalidf("toolId 不能为空")
	}
	deleted, err := model.SoftDeleteToolUserPolicyByToolID(ctx, toolID, strings.TrimSpace(operator))
	if err != nil {
		return err
	}
	if !deleted {
		return components.ErrorToolUserPolicyNotFound.Sprintf(toolID)
	}
	return nil
}

func GetToolUserPolicy(ctx *gin.Context, toolID string) (*model.ToolUserPolicy, error) {
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return nil, components.ParamInvalidf("toolId 不能为空")
	}
	return getToolUserPolicy(ctx, toolID)
}

func getToolUserPolicy(ctx *gin.Context, toolID string) (*model.ToolUserPolicy, error) {
	policy, err := model.GetToolUserPolicyByToolID(ctx, toolID)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, components.ErrorToolUserPolicyNotFound.Sprintf(toolID)
	}
	return policy, nil
}

func ListToolUserPolicies(ctx *gin.Context, toolIDs []string) ([]model.ToolUserPolicy, error) {
	normalizedToolIDs, err := normalizeToolPolicyIDs(toolIDs)
	if err != nil {
		return nil, err
	}
	return model.ListToolUserPoliciesByToolIDs(ctx, normalizedToolIDs)
}

func ToToolUserPolicyResp(policy *model.ToolUserPolicy) (params.ToolUserPolicyResp, error) {
	var whiteUserList []string
	if err := json.Unmarshal([]byte(policy.WhiteUserList), &whiteUserList); err != nil {
		return params.ToolUserPolicyResp{}, components.ErrorToolUserPolicyInvalid.Sprintf(policy.ToolID)
	}
	normalized, err := normalizeToolUserList(whiteUserList)
	if err != nil {
		return params.ToolUserPolicyResp{}, components.ErrorToolUserPolicyInvalid.Sprintf(policy.ToolID)
	}
	return params.ToolUserPolicyResp{
		ToolID:        policy.ToolID,
		WhiteUserList: normalized,
		CreatedAt:     policy.CreatedAt.Format("2006-01-02 15:04:05"),
		CreatedBy:     policy.CreatedBy,
		UpdatedAt:     policy.UpdatedAt.Format("2006-01-02 15:04:05"),
		UpdatedBy:     policy.UpdatedBy,
	}, nil
}
