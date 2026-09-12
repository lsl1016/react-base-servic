package params

import "encoding/json"

type ReactWSMessage struct {
	Type      string          `json:"type"`
	RunID     string          `json:"runId,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty" swaggertype:"object"`
}

type ReactAttachmentRef struct {
	FileID      string `json:"fileId"`
	FileName    string `json:"fileName,omitempty"`
	Description string `json:"description,omitempty"`
}

type ReactRunPayload struct {
	CallerKey   string               `json:"callerKey"`
	RouteValues []string             `json:"routeValues"`
	Type        string               `json:"type"`
	UserPrompt  string               `json:"userPrompt"`
	Attachments []ReactAttachmentRef `json:"attachments,omitempty"`
	// ControlContext 是运行控制参数，给后端运行态和工具执行链路使用，不作为普通用户提示词直接入模。
	// 这里适合放 requestSource、工具控制参数、执行开关等运行时控制信息，不应放文件正文。
	ControlContext json.RawMessage `json:"controlContext" swaggertype:"object"`
	// LLMContext 是补充给模型的业务上下文/页面上下文，支持 JSON 对象或纯文本字符串，会随用户消息一起参与入模。
	// 这里适合放业务背景、页面态信息、补充说明等提示词上下文，不应用来承载工具控制参数或文件正文。
	LLMContext   json.RawMessage `json:"llmContext" swaggertype:"object"`
	ModelKey     string          `json:"modelKey"`
	ModelVersion string          `json:"modelVersion"`
	ModelHash    string          `json:"modelHash"`
	MaxSteps     int             `json:"maxSteps"`
}

type ReactModelInfo struct {
	ModelKey     string `json:"modelKey"`
	ModelVersion string `json:"modelVersion"`
	DisplayName  string `json:"displayName"`
}

type ReactModelsResp struct {
	Models       []ReactModelInfo `json:"models"`
	DefaultModel ReactModelInfo   `json:"defaultModel"`
}

type ReactModelFallbackPayload struct {
	FromModelKey       string `json:"fromModelKey"`
	FromModelVersion   string `json:"fromModelVersion"`
	ToModelKey         string `json:"toModelKey"`
	ToModelVersion     string `json:"toModelVersion"`
	Reason             string `json:"reason"`
	ResetCurrentOutput bool   `json:"resetCurrentOutput"`
}

type ReactEvent struct {
	Type      string `json:"type"`
	Seq       int    `json:"seq"`
	RunID     string `json:"runId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	StepIndex *int   `json:"stepIndex,omitempty"`
	// AgentPath 标记多 Agent 事件归属（如 main/ops-agent）；外层 run 省略该字段，
	// 旧客户端无感，新客户端按缺省 "main" 渲染。
	AgentPath string `json:"agentPath,omitempty"`
	Payload   any    `json:"payload,omitempty"`
}

type ReactThoughtStartPayload struct{}

type ReactThoughtDeltaPayload struct {
	ContentDelta string `json:"contentDelta,omitempty"`
}

type ReactThoughtEndPayload struct {
	Content           string `json:"content,omitempty"`
	InputTokens       int    `json:"inputTokens,omitempty"`
	OutputTokens      int    `json:"outputTokens,omitempty"`
	CacheReadTokens   int    `json:"cacheReadTokens,omitempty"`
	CacheCreateTokens int    `json:"cacheCreateTokens,omitempty"`
	ContextUsedTokens int    `json:"contextUsedTokens,omitempty"`
	MaxContextTokens  int    `json:"maxContextTokens,omitempty"`
}

type ReactContentStartPayload struct{}

type ReactContentDeltaPayload struct {
	ContentDelta string `json:"contentDelta"`
}

type ReactContentEndPayload struct {
	Content           string `json:"content,omitempty"`
	InputTokens       int    `json:"inputTokens,omitempty"`
	OutputTokens      int    `json:"outputTokens,omitempty"`
	CacheReadTokens   int    `json:"cacheReadTokens,omitempty"`
	CacheCreateTokens int    `json:"cacheCreateTokens,omitempty"`
	ContextUsedTokens int    `json:"contextUsedTokens,omitempty"`
	MaxContextTokens  int    `json:"maxContextTokens,omitempty"`
}

type ReactToolUseStartPayload struct {
	ToolUseID   string          `json:"toolUseId"`
	ToolName    string          `json:"toolName"`
	ToolInput   json.RawMessage `json:"toolInput,omitempty" swaggertype:"object"`
	Description string          `json:"description,omitempty"`
	ExecutedBy  string          `json:"executedBy"`
	Status      string          `json:"status"`
}

type ReactClientToolUseStartPayload struct {
	ToolUseID    string          `json:"toolUseId"`
	ToolName     string          `json:"toolName"`
	ToolInput    json.RawMessage `json:"toolInput,omitempty" swaggertype:"object"`
	Description  string          `json:"description,omitempty"`
	FrontendHint string          `json:"frontendHint,omitempty"`
	Status       string          `json:"status"`
}

type ReactToolUseEndPayload struct {
	ToolUseID    string          `json:"toolUseId"`
	Content      string          `json:"content,omitempty"`
	ResultRef    string          `json:"resultRef,omitempty"`
	Truncated    bool            `json:"truncated"`
	OmittedChars int             `json:"omittedChars,omitempty"`
	IsError      bool            `json:"isError"`
	ExecutedBy   string          `json:"executedBy"`
	Status       string          `json:"status"`
	DurationMs   int64           `json:"durationMs"`
	Meta         json.RawMessage `json:"meta,omitempty" swaggertype:"object"`
}

type ReactClientToolOutput struct {
	ToolUseID string          `json:"toolUseId"`
	Content   string          `json:"content"`
	Meta      json.RawMessage `json:"meta,omitempty" swaggertype:"object"`
	IsError   bool            `json:"isError"`
	Status    string          `json:"status,omitempty"`
}

type ReactClientToolUseEndPayload struct {
	ToolOutputs []ReactClientToolOutput `json:"toolOutputs"`
}

type ReactCompactStartPayload struct {
	MessageCount  int `json:"messageCount"`
	EstimatedSize int `json:"estimatedSize"`
}

type ReactCompactEndPayload struct {
	BeforeMessageCount int    `json:"beforeMessageCount"`
	AfterMessageCount  int    `json:"afterMessageCount"`
	Summary            string `json:"summary"`
}

type ReactTodoUpdatePayload struct {
	TodoState json.RawMessage `json:"todoState" swaggertype:"object"`
}

type ReactHeartbeatPayload struct {
	Timestamp int64 `json:"ts"`
}

type ReactDonePayload struct {
	InputTokens       int    `json:"inputTokens"`
	OutputTokens      int    `json:"outputTokens"`
	CacheReadTokens   int    `json:"cacheReadTokens"`
	CacheCreateTokens int    `json:"cacheCreateTokens"`
	ContextUsedTokens int    `json:"contextUsedTokens"`
	MaxContextTokens  int    `json:"maxContextTokens"`
	TerminationReason string `json:"terminationReason,omitempty"`
}

type ReactErrorPayload struct {
	ErrNo             int    `json:"errNo"`
	ErrMsg            string `json:"errMsg"`
	ContextUsedTokens int    `json:"contextUsedTokens"`
	MaxContextTokens  int    `json:"maxContextTokens"`
}

type ReactCancelledPayload struct {
	OK                bool   `json:"ok"`
	Reason            string `json:"reason,omitempty"`
	ContextUsedTokens int    `json:"contextUsedTokens"`
	MaxContextTokens  int    `json:"maxContextTokens"`
}

type ReactSessionListReq struct {
	CallerKey   string   `json:"callerKey" binding:"required"`
	RouteValues []string `json:"routeValues"`
	Type        string   `json:"type"`
	Keyword     string   `json:"keyword"`
	Page        int      `json:"page"`
	PageSize    int      `json:"pageSize"`
}

type ReactSessionItem struct {
	SessionID   string   `json:"sessionId"`
	CallerKey   string   `json:"callerKey"`
	RouteValues []string `json:"routeValues"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	LastRunID   string   `json:"lastRunId"`
	LastMessage string   `json:"lastMessage"`
	State       string   `json:"state"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

type ReactSessionListResp struct {
	Sessions []ReactSessionItem `json:"sessions"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

type ReactSessionEventsReq struct {
	SessionID string `json:"sessionId" binding:"required"`
}

// ReactAsyncTaskListReq 查询当前会话第三方已终态但本地尚未处理的异步任务，使用稳定游标分页。
type ReactAsyncTaskListReq struct {
	SessionID string `json:"sessionId" binding:"required"`
	Cursor    string `json:"cursor"`
	PageSize  int    `json:"pageSize"`
}

// ReactAsyncTaskItem 是前端可见的异步任务状态，不暴露提交快照和 Provider 任务信息。
type ReactAsyncTaskItem struct {
	ToolUseID       string `json:"toolUseId"`
	ToolName        string `json:"toolName"`
	State           string `json:"state"`
	ExecutionStatus string `json:"executionStatus"`
	ProviderStatus  string `json:"providerStatus,omitempty"`
	Progress        *int   `json:"progress,omitempty"`
	ErrorMessage    string `json:"errorMessage,omitempty"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
	LastObservedAt  string `json:"lastObservedAt,omitempty"`
	CompletedAt     string `json:"completedAt,omitempty"`
}

// ReactAsyncTaskListResp 返回当前页和下一页游标；前端应取完全部页面后再替换会话任务快照。
type ReactAsyncTaskListResp struct {
	Tasks              []ReactAsyncTaskItem `json:"tasks"`
	NextCursor         string               `json:"nextCursor,omitempty"`
	HasMore            bool                 `json:"hasMore"`
	HasProcessingTasks bool                 `json:"hasProcessingTasks"`
}

type ReactHistoryEvent struct {
	Type      string `json:"type"`
	Seq       int    `json:"seq"`
	RunID     string `json:"runId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	StepIndex *int   `json:"stepIndex,omitempty"`
	AgentPath string `json:"agentPath,omitempty"`
	Payload   any    `json:"payload,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

type ReactSessionEventsResp struct {
	SessionID string              `json:"sessionId"`
	Title     string              `json:"title"`
	Events    []ReactHistoryEvent `json:"events"`
}

// ReactRunFeedbackReq 轮次反馈请求：feedback 与 problemFeedback 至少传一个
type ReactRunFeedbackReq struct {
	RunID           string  `json:"runId" binding:"required"`
	Feedback        *int    `json:"feedback,omitempty"`
	ProblemFeedback *string `json:"problemFeedback,omitempty"`
}

// ReactRunFeedbackResp 轮次反馈响应
type ReactRunFeedbackResp struct {
	RunID                    string `json:"runId"`
	SessionID                string `json:"sessionId"`
	Feedback                 int    `json:"feedback"`
	ProblemFeedback          string `json:"problemFeedback"`
	FeedbackUpdatedAt        string `json:"feedbackUpdatedAt,omitempty"`
	ProblemFeedbackUpdatedAt string `json:"problemFeedbackUpdatedAt,omitempty"`
}

// ReactSessionFeedbackReq 会话维度查询轮次反馈请求（历史回显）
type ReactSessionFeedbackReq struct {
	SessionID string `json:"sessionId" binding:"required"`
}

// ReactSessionFeedbackResp 会话维度轮次反馈列表
type ReactSessionFeedbackResp struct {
	Items []ReactRunFeedbackResp `json:"items"`
}
