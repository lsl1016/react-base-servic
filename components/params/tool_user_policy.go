package params

// CreateToolUserPolicyReq 创建工具用户白名单策略请求。
type CreateToolUserPolicyReq struct {
	ToolID        string   `json:"toolId" binding:"required"`
	WhiteUserList []string `json:"whiteUserList" binding:"required"`
}

// UpdateToolUserPolicyReq 更新工具用户白名单策略请求。
type UpdateToolUserPolicyReq struct {
	ToolID        string   `json:"toolId" binding:"required"`
	WhiteUserList []string `json:"whiteUserList" binding:"required"`
}

// DeleteToolUserPolicyReq 删除工具用户白名单策略请求。
type DeleteToolUserPolicyReq struct {
	ToolID string `json:"toolId" binding:"required"`
}

// ToolUserPolicyDetailReq 查询工具用户白名单策略详情请求。
type ToolUserPolicyDetailReq struct {
	ToolID string `json:"toolId" binding:"required"`
}

// ListToolUserPoliciesReq 批量查询工具用户白名单策略请求。
type ListToolUserPoliciesReq struct {
	ToolIDs []string `json:"toolIds" binding:"required"`
}

// ToolUserPolicyResp 工具用户白名单策略响应。
// black_user_list 是预留字段，本期不通过接口暴露。
type ToolUserPolicyResp struct {
	ToolID        string   `json:"toolId"`
	WhiteUserList []string `json:"whiteUserList"`
	CreatedAt     string   `json:"createdAt"`
	CreatedBy     string   `json:"createdBy"`
	UpdatedAt     string   `json:"updatedAt"`
	UpdatedBy     string   `json:"updatedBy"`
}
