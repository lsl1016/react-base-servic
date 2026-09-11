package params

// WhitelistResp 白名单响应
type WhitelistResp struct {
	Users []string `json:"users"`
}

// ========== 模型管理接口 DTO ==========

// ConnCheckReq 连通性检测请求
type ConnCheckReq struct {
	ModelKey     string `json:"modelKey" binding:"required"`
	ModelVersion string `json:"modelVersion" binding:"required"`
	ApiKey       string `json:"apiKey" binding:"required"`
}

// CreateUserModelReq 创建模型请求
type CreateUserModelReq struct {
	ModelName         string   `json:"modelName" binding:"required"`
	ModelKey          string   `json:"modelKey" binding:"required"`
	ModelVersion      string   `json:"modelVersion" binding:"required"`
	ApiKey            string   `json:"apiKey" binding:"required"`
	BizScenes         []string `json:"bizScenes" binding:"required,min=1"`
	IsPlatformDefault int      `json:"isPlatformDefault"`
}

// UpdateUserModelReq 编辑模型请求
type UpdateUserModelReq struct {
	ID                uint     `json:"id" binding:"required"`
	ModelName         string   `json:"modelName"`
	ModelKey          string   `json:"modelKey"`
	ModelVersion      string   `json:"modelVersion"`
	ApiKey            string   `json:"apiKey"`
	BizScenes         []string `json:"bizScenes" binding:"omitempty,min=1"`
	IsPlatformDefault int      `json:"isPlatformDefault"`
}

// DeleteUserModelReq 删除模型请求
type DeleteUserModelReq struct {
	ID uint `json:"id" binding:"required"`
}

// GetUserModelDetailReq 模型详情请求
type GetUserModelDetailReq struct {
	ID uint `json:"id" binding:"required"`
}

// ListUserModelsReq 模型列表请求
type ListUserModelsReq struct {
	BizScene  string `json:"bizScene"`
	ModelName string `json:"modelName"`
}

// UserModelItem 模型列表响应项（API Key 脱敏）
type UserModelItem struct {
	ID                uint     `json:"id"`
	ModelHash         string   `json:"modelHash"`
	UserName          string   `json:"userName"`
	ModelName         string   `json:"modelName"`
	ModelKey          string   `json:"modelKey"`
	ModelVersion      string   `json:"modelVersion"`
	ApiKey            string   `json:"apiKey"`
	BizScenes         []string `json:"bizScenes"`
	IsPlatformDefault int      `json:"isPlatformDefault"`
	CreatedAt         string   `json:"createdAt"`
	UpdatedAt         string   `json:"updatedAt"`
	BaseCredits       *int     `json:"baseCredits,omitempty"`
	BonusCredits      *int     `json:"bonusCredits,omitempty"`
}

// ========== 积分管理接口 DTO ==========

// AdjustCreditsReq 积分调整请求
type AdjustCreditsReq struct {
	UserName  string `json:"userName" binding:"required"`
	ModelHash string `json:"modelHash" binding:"required"`
	Delta     int    `json:"delta" binding:"required"`
}

// ModelsResp 可用模型列表响应
type ModelsResp struct {
	Models []ModelInfo `json:"models"`
}

// ModelInfo 模型信息
type ModelInfo struct {
	Key            string   `json:"key"`
	Versions       []string `json:"versions"`
	DefaultVersion string   `json:"defaultVersion"`
}
