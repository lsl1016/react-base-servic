package react

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	llm "react-base-service/api/llm"
	"react-base-service/components/params"
	model "react-base-service/models/llm"

	"github.com/gin-gonic/gin"
)

// displayFilesInput 只收 artifactIds：模型全程不接触 uri（防止其在正文里拼链接或改写地址），
// 展示所需的 type/name/uri 由后端按 ID 查库铸造。
type displayFilesInput struct {
	ArtifactIDs []string `json:"artifactIds"`
}

// displayFilesArtifactLookup 按 artifactID 查产物元数据；抽成变量便于单测注入。
var displayFilesArtifactLookup = func(ctx *gin.Context, artifactID string) (*model.ReactArtifact, error) {
	return model.GetReactArtifactByArtifactID(ctx, artifactID)
}

// displayFilesStoreResultRef 落库工具结果引用；抽成变量便于单测注入（避免单测触 DB）。
var displayFilesStoreResultRef = storeResultRef

func displayFilesToolDefinition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        metaToolDisplayFiles,
		Description: "向用户显示文件。只要需要向用户展示或提供图片、HTML、PDF、CSV 等任何文件，必须调用本工具，禁止在正文中输出文件链接或自行构造任何 uri。artifactIds 传入文件生成工具返回描述符里的 artifactId 原值。图片会直接展示并支持放大和前后切换，其他文件会展示预览和下载入口。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": stringSchema("本次展示文件的简短描述，可选。"),
				"artifactIds": map[string]interface{}{
					"type":        "array",
					"description": "需要展示的文件 artifactId 列表，取自文件生成工具返回描述符的 artifactId 字段，必须原样传入。",
					"minItems":    1,
					"items":       stringSchema("文件的 artifactId。"),
				},
			},
			"required":             []string{"artifactIds"},
			"additionalProperties": false,
		},
	}
}

// resolveDisplayFilesArtifacts 把模型传的 artifactIds 解析成前端可渲染的产物条目：
// 逐个查库校验产物存在且归属当前会话（防编造 ID / 跨会话引用），type/name/uri 一律以库中记录铸造，
// 模型无从改写。任一 ID 无效即整体报错，让模型修正后重试。
func (s *reactEngineState) resolveDisplayFilesArtifacts(input json.RawMessage) ([]pythonExecArtifactMetaItem, error) {
	var req displayFilesInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("displayFiles input must be a valid JSON object: %w", err)
	}
	if len(req.ArtifactIDs) == 0 {
		return nil, fmt.Errorf("displayFiles artifactIds must not be empty")
	}
	items := make([]pythonExecArtifactMetaItem, 0, len(req.ArtifactIDs))
	for i, rawID := range req.ArtifactIDs {
		artifactID := strings.TrimSpace(rawID)
		if artifactID == "" {
			return nil, fmt.Errorf("displayFiles artifactIds[%d] is required", i)
		}
		record, err := displayFilesArtifactLookup(s.ctx, artifactID)
		if err != nil {
			return nil, fmt.Errorf("displayFiles artifactIds[%d] lookup failed, please retry", i)
		}
		if record == nil || record.SessionID != s.sessionID {
			return nil, fmt.Errorf("displayFiles artifactIds[%d] %q not found in current session; pass the artifactId exactly as returned by the file-generating tool", i, artifactID)
		}
		items = append(items, pythonExecArtifactMetaItem{
			Type: record.MimeType,
			Name: record.FileName,
			URI:  pythonExecArtifactURLPrefix + record.ArtifactID,
		})
	}
	return items, nil
}

// executeDisplayFilesTool 是 displayFiles 的专属执行路径（仿 ask_question 自发事件的先例）：
// 前端只消费 tool_use_start 里的 input（reducer 对内部工具的 tool_use_end 不接 meta），
// 所以必须在发射 tool_use_start 之前完成解析校验，把 input 替换成后端铸造的
// {"artifacts":[{type,name,uri}]}（前端既有契约形状）——前端看到的永远是验证过的真数据。
// 解析失败时保留模型原始 input（artifactIds 形状渲染不出产物，不会污染 UI），并以错误结果让模型重试。
// 铸造结果同时作为 toolMeta 落库，供历史回放直接复用为 tool_use_start 的 input。
func (s *reactEngineState) executeDisplayFilesTool(call llm.ToolCall, step int) (llm.ToolResultContent, error) {
	description := extractToolDescription(call.Input)
	toolInput := stripToolDescriptionInput(call.Input)

	resolved, resolveErr := s.resolveDisplayFilesArtifacts(toolInput)
	emitInput := toolInput
	var meta json.RawMessage
	if resolveErr == nil {
		if enriched, err := json.Marshal(pythonExecArtifactMeta{Artifacts: resolved}); err == nil {
			emitInput = enriched
			meta = enriched
		}
	}
	_ = s.emitter.EmitStep(step, EventToolUseStart, params.ReactToolUseStartPayload{ToolUseID: call.ID, ToolName: call.Name, ToolInput: emitInput, Description: description, ExecutedBy: executedByInternal, Status: toolExecutionStatusRunning})
	start := time.Now()

	var content string
	isError := false
	if resolveErr != nil {
		content = resolveErr.Error()
		isError = true
	} else {
		body, _ := json.Marshal(map[string]interface{}{
			"displayed": len(resolved),
			"message":   "files displayed to the user",
		})
		content = string(body)
	}
	normalized := normalizeToolResult(call.ID, content, isError, executedByInternal)
	if normalized.ResultRef != "" {
		if err := displayFilesStoreResultRef(s.ctx, s.sessionID, s.runID, call.ID, normalized.ResultRef, content); err != nil {
			return llm.ToolResultContent{}, err
		}
	}
	_ = s.emitter.EmitStep(step, EventToolUseEnd, params.ReactToolUseEndPayload{ToolUseID: call.ID, Content: normalized.Content, ResultRef: normalized.ResultRef, Truncated: normalized.Truncated, OmittedChars: normalized.OmittedChars, IsError: normalized.IsError, ExecutedBy: executedByInternal, Status: normalized.Status, DurationMs: time.Since(start).Milliseconds(), Meta: meta})
	return llm.ToolResultContent{ToolUseID: call.ID, Content: normalized.LLMContent(), IsError: normalized.IsError, Meta: meta}, nil
}
