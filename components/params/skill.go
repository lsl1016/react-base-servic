package params

import "encoding/json"

// 澄清历史回放展示方式（history 接口用于回放一致性；chat 实时澄清可不返回）
const (
	ClarificationDisplayModeMultiQuestion = "multi_question"
	ClarificationDisplayModeSingleInput   = "single_input"
)

// ClarificationQuestion 澄清问题
type ClarificationQuestion struct {
	QuestionID  string `json:"questionId"`
	Question    string `json:"question"`
	Placeholder string `json:"placeholder,omitempty"`
}

// ClarificationAnswerItem 澄清答案
type ClarificationAnswerItem struct {
	QuestionID string `json:"questionId"`
	Answer     string `json:"answer"`
}

// ClarificationSubmit 澄清提交请求
type ClarificationSubmit struct {
	ClarifyID string                    `json:"clarifyId"`
	MessageID string                    `json:"messageId"`
	Round     int                       `json:"round"`
	Answers   []ClarificationAnswerItem `json:"answers,omitempty"`
	// PlainAnswer 单输入框整体回答；与 answers 二选一，不可同时非空
	PlainAnswer string `json:"plainAnswer,omitempty"`
}

// ClarificationDetail 澄清事件详情（SSE 推送和落库用）
type ClarificationDetail struct {
	ClarifyID          string                    `json:"clarifyId"`
	Round              int                       `json:"round"`
	Message            string                    `json:"message"`
	ClarificationState string                    `json:"clarificationState"`
	Questions          []ClarificationQuestion   `json:"questions"`
	Answers            []ClarificationAnswerItem `json:"answers,omitempty"`
	AnsweredAt         string                    `json:"answeredAt,omitempty"`
	Source             string                    `json:"source,omitempty"`
	// DisplayText / InputPlaceholder：chat 实时澄清一并下发，供单输入前端使用；老前端可忽略
	DisplayText      string `json:"displayText,omitempty"`
	InputPlaceholder string `json:"inputPlaceholder,omitempty"`
	// DisplayMode：建议在 history 已回答澄清中返回；chat 实时可不返回
	DisplayMode string `json:"displayMode,omitempty"`
	// PlainAnswer：history 中单输入回放时返回用户整段回答
	PlainAnswer string `json:"plainAnswer,omitempty"`
}

// SkillChatAttachment 本轮引用的聊天附件（仅 dingtalk-datamap 支持，见 chat 层校验）
type SkillChatAttachment struct {
	FileID      string `json:"fileId"`
	FileName    string `json:"fileName,omitempty"`
	Description string `json:"description,omitempty"`
}

// SkillChatReq 核心对话请求
type SkillChatReq struct {
	CallerKey     string               `json:"callerKey"`
	RouteValues   []string             `json:"routeValues"`
	UserPrompt    string               `json:"userPrompt"`
	ContextInfo   json.RawMessage      `json:"contextInfo" swaggertype:"object"`
	RequestSource string               `json:"requestSource,omitempty"`
	SessionID     string               `json:"sessionId"`
	HistoryTTL    int                  `json:"historyTTL"`
	ModelKey      string               `json:"modelKey"`
	ModelVersion  string               `json:"modelVersion"`
	ModelHash     string               `json:"modelHash"` // 用户自定义模型标识，非空时走用户模型逻辑，忽略 modelKey/modelVersion
	DebugMode     string               `json:"debugMode"` // full 开启调试；空关闭；skill_match/execution_plan 兼容等同 full
	ForceUpdate   bool                 `json:"forceUpdate"`
	Type          string               `json:"type"`
	Clarification *ClarificationSubmit `json:"clarification,omitempty"`
	DingTalkMeta  json.RawMessage      `json:"dingTalkMeta,omitempty" swaggertype:"object"`
	// Attachments 顶层附件引用；仅首轮或需要新文件时传。文件正文会拼进该条 user 消息的 content，追问只发 userPrompt 即可
	Attachments []SkillChatAttachment `json:"attachments,omitempty"`
}

// SkillSessionListReq 会话列表请求
type SkillSessionListReq struct {
	CallerKey   string   `json:"callerKey" binding:"required"`
	RouteValues []string `json:"routeValues"`
	Keyword     string   `json:"keyword"`
}

// SkillSessionMessagesReq 会话消息请求
type SkillSessionMessagesReq struct {
	SessionID   string   `json:"sessionId"`
	CallerKey   string   `json:"callerKey"`
	RouteValues []string `json:"routeValues"`
}

// SkillRenameSessionReq 会话重命名请求
type SkillRenameSessionReq struct {
	SessionID string `json:"sessionId" binding:"required"`
	Title     string `json:"title" binding:"required"`
}

// SkillDeleteSessionReq 删除会话请求
type SkillDeleteSessionReq struct {
	SessionID string `json:"sessionId" binding:"required"`
}

// SkillMessageFeedbackReq 回答反馈请求
type SkillMessageFeedbackReq struct {
	MessageID       string  `json:"messageId" binding:"required"`
	Feedback        *int    `json:"feedback,omitempty"`
	ProblemFeedback *string `json:"problemFeedback,omitempty"`
}

// SkillMessageFeedbackResp 回答反馈响应
type SkillMessageFeedbackResp struct {
	SessionID                string `json:"sessionId"`
	MessageID                string `json:"messageId"`
	Sequence                 int    `json:"sequence"`
	Feedback                 int    `json:"feedback"`
	ProblemFeedback          string `json:"problemFeedback"`
	FeedbackUpdatedAt        string `json:"feedbackUpdatedAt,omitempty"`
	ProblemFeedbackUpdatedAt string `json:"problemFeedbackUpdatedAt,omitempty"`
}

// RegisterCallerReq Caller 注册请求
type RegisterCallerReq struct {
	CallerKey   string `json:"callerKey" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Platform    string `json:"platform" binding:"required"`
}

// UpdateCallerReq Caller 更新请求
type UpdateCallerReq struct {
	CallerKey   string `json:"callerKey" binding:"required"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Platform    string `json:"platform"`
	Status      *int   `json:"status"`
}

// ListCallersReq Caller 列表请求（预留扩展）
type ListCallersReq struct{}

// CallerResp Caller 响应
type CallerResp struct {
	CallerKey   string `json:"callerKey"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Platform    string `json:"platform"`
	Status      int    `json:"status"`
	CreatedAt   string `json:"createdAt"`
}

// CreateSkillReq Skill 创建请求
type CreateSkillReq struct {
	Name               string   `json:"name" binding:"required"`
	Description        string   `json:"description" binding:"required"`
	TriggerCondition   string   `json:"triggerCondition" binding:"required"`
	ForbiddenCondition string   `json:"forbiddenCondition"`
	ExecutionSteps     string   `json:"executionSteps" binding:"required"`
	BusinessContext    string   `json:"businessContext" binding:"required"`
	PromptSupplement   string   `json:"promptSupplement"`
	CallerKey          string   `json:"callerKey" binding:"required"`
	RouteValues        []string `json:"routeValues" binding:"required"`
	Status             *int     `json:"status" binding:"required"`
	IsDefault          *int     `json:"isDefault"`
}

// UpdateSkillReq Skill 更新请求
type UpdateSkillReq struct {
	SkillID            string   `json:"skillId" binding:"required"`
	Name               string   `json:"name" binding:"required"`
	Description        string   `json:"description" binding:"required"`
	TriggerCondition   string   `json:"triggerCondition" binding:"required"`
	ForbiddenCondition *string  `json:"forbiddenCondition"`
	ExecutionSteps     string   `json:"executionSteps" binding:"required"`
	BusinessContext    string   `json:"businessContext" binding:"required"`
	PromptSupplement   *string  `json:"promptSupplement"`
	RouteValues        []string `json:"routeValues" binding:"required"`
	Status             *int     `json:"status" binding:"required"`
}

// DeleteSkillReq Skill 删除请求
type DeleteSkillReq struct {
	SkillID string `json:"skillId" binding:"required"`
}

// ListSkillsReq Skill 列表请求
type ListSkillsReq struct {
	CallerKey   string   `json:"callerKey" binding:"required"`
	RouteValues []string `json:"routeValues" binding:"required"`
}

// SkillDetailReq Skill 详情请求
type SkillDetailReq struct {
	SkillID string `json:"skillId" binding:"required"`
}

// SkillResp Skill 响应
type SkillResp struct {
	SkillID            string   `json:"skillId"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	TriggerCondition   string   `json:"triggerCondition"`
	ForbiddenCondition string   `json:"forbiddenCondition"`
	ExecutionSteps     string   `json:"executionSteps"`
	BusinessContext    string   `json:"businessContext"`
	PromptSupplement   string   `json:"promptSupplement"`
	CallerKey          string   `json:"callerKey"`
	RouteValues        []string `json:"routeValues"`
	IsDefault          int      `json:"isDefault"`
	Status             int      `json:"status"`
	CreatedBy          string   `json:"createdBy"`
	UpdatedBy          string   `json:"updatedBy"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
	DeletedAt          string   `json:"deletedAt"`
}

// RegisterToolReq Tool 注册请求
type RegisterToolReq struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description" binding:"required"`
	ToolType    string          `json:"toolType" binding:"required"`
	CallerKey   string          `json:"callerKey" binding:"required"`
	RouteValues []string        `json:"routeValues" binding:"required"`
	Config      json.RawMessage `json:"config" binding:"required" swaggertype:"object"`
}

// UpdateToolReq Tool 更新请求
type UpdateToolReq struct {
	ToolID      string          `json:"toolId" binding:"required"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	ToolType    string          `json:"toolType"`
	RouteValues []string        `json:"routeValues"`
	Config      json.RawMessage `json:"config" swaggertype:"object"`
	Status      *int            `json:"status"`
}

// DeleteToolReq Tool 删除请求
type DeleteToolReq struct {
	ToolID string `json:"toolId" binding:"required"`
}

// ListToolsReq Tool 列表请求
type ListToolsReq struct {
	CallerKey   string   `json:"callerKey" binding:"required"`
	RouteValues []string `json:"routeValues" binding:"required"`
}

// ToolDetailReq Tool 获取详情请求
type ToolDetailReq struct {
	ToolID string `json:"toolId" binding:"required"`
}

// ToolResp Tool 响应
type ToolResp struct {
	ToolID      string          `json:"toolId"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	ToolType    string          `json:"toolType"`
	CallerKey   string          `json:"callerKey"`
	RouteValues []string        `json:"routeValues"`
	Config      json.RawMessage `json:"config" swaggertype:"object"`
	Status      int             `json:"status"`
	CreatedAt   string          `json:"createdAt"`
	CreatedBy   string          `json:"createdBy"`
	UpdatedAt   string          `json:"updatedAt"`
	UpdatedBy   string          `json:"updatedBy"`
}

// ---------- API Key ----------

// RegisterApiKeyReq API Key 注册请求
type RegisterApiKeyReq struct {
	CallerKey   string   `json:"callerKey" binding:"required"`
	RouteValues []string `json:"routeValues" binding:"required"`
	Name        string   `json:"name" binding:"required"`
	ApiKey      string   `json:"apiKey" binding:"required"`
}

// UpdateApiKeyReq API Key 更新请求
type UpdateApiKeyReq struct {
	ID          uint     `json:"id" binding:"required"`
	Name        string   `json:"name"`
	RouteValues []string `json:"routeValues"`
	ApiKey      string   `json:"apiKey"`
	Status      *int     `json:"status"`
}

// DeleteApiKeyReq API Key 删除请求
type DeleteApiKeyReq struct {
	ID uint `json:"id" binding:"required"`
}

// ListApiKeysReq API Key 列表请求
type ListApiKeysReq struct {
	CallerKey   string   `json:"callerKey" binding:"required"`
	RouteValues []string `json:"routeValues" binding:"required"`
}

// ApiKeyDetailReq API Key 详情请求
type ApiKeyDetailReq struct {
	ID uint `json:"id" binding:"required"`
}

// ApiKeyResp API Key 响应
type ApiKeyResp struct {
	ID          uint     `json:"id"`
	CallerKey   string   `json:"callerKey"`
	RouteValues []string `json:"routeValues"`
	Name        string   `json:"name"`
	ApiKey      string   `json:"apiKey"`
	Status      int      `json:"status"`
	CreatedBy   string   `json:"createdBy"`
	UpdatedBy   string   `json:"updatedBy"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

// ---------- 系统提示词 ----------

// RegisterSystemPromptReq 系统提示词注册请求
type RegisterSystemPromptReq struct {
	CallerKey   string   `json:"callerKey" binding:"required"`
	RouteValues []string `json:"routeValues" binding:"required"`
	Name        string   `json:"name" binding:"required"`
	Content     string   `json:"content" binding:"required"`
}

// UpdateSystemPromptReq 系统提示词更新请求
type UpdateSystemPromptReq struct {
	ID          uint     `json:"id" binding:"required"`
	Name        string   `json:"name"`
	Content     string   `json:"content"`
	RouteValues []string `json:"routeValues"`
	Status      *int     `json:"status"`
}

// DeleteSystemPromptReq 系统提示词删除请求
type DeleteSystemPromptReq struct {
	ID uint `json:"id" binding:"required"`
}

// ListSystemPromptsReq 系统提示词列表请求
type ListSystemPromptsReq struct {
	CallerKey   string   `json:"callerKey" binding:"required"`
	RouteValues []string `json:"routeValues" binding:"required"`
}

// SystemPromptDetailReq 系统提示词详情请求
type SystemPromptDetailReq struct {
	ID uint `json:"id" binding:"required"`
}

// SystemPromptResp 系统提示词响应
type SystemPromptResp struct {
	ID          uint     `json:"id"`
	CallerKey   string   `json:"callerKey"`
	RouteValues []string `json:"routeValues"`
	Name        string   `json:"name"`
	Content     string   `json:"content"`
	Status      int      `json:"status"`
	CreatedBy   string   `json:"createdBy"`
	UpdatedBy   string   `json:"updatedBy"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}
