package react

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	llm "react-base-service/api/llm"
	"react-base-service/components/params"
	model "react-base-service/models/llm"
	toolService "react-base-service/service/tool"
)

type clientToolOutput struct {
	ToolUseID string          `json:"toolUseId"`
	Content   json.RawMessage `json:"content"`
	Meta      json.RawMessage `json:"meta,omitempty"`
	IsError   bool            `json:"isError"`
}

const maxClientToolMetaBytes = 1 << 20

// executeClientToolCalls 批量通知前端执行 client tool，并等待同一条 WebSocket 连接回填所有结果。
func (s *reactEngineState) executeClientToolCalls(calls []reactClientToolCall, step int, description string) (map[int]llm.ToolResultContent, error) {
	if s.readClient == nil {
		return nil, fmt.Errorf("client tool requires websocket reader")
	}
	pendingIDs := make([]string, 0, len(calls))
	callByID := make(map[string]reactClientToolCall, len(calls))
	clientEmitter := s.clientToolEmitter()
	for _, item := range calls {
		pendingIDs = append(pendingIDs, item.call.ID)
		callByID[item.call.ID] = item
		_ = clientEmitter.EmitStep(step, EventClientToolUseStart, params.ReactClientToolUseStartPayload{ToolUseID: item.call.ID, ToolName: item.tool.Name, ToolInput: json.RawMessage(item.call.Input), Description: strings.TrimSpace(description), FrontendHint: clientToolFrontendHint(item.tool), Status: toolExecutionStatusWaiting})
	}

	pendingJSON, _ := json.Marshal(pendingIDs)
	if err := model.UpdateReactRunByRunID(s.ctx, s.runID, map[string]interface{}{"state": model.ReactRunStateWaitingClientMessage, "pending_tool_use_ids": string(pendingJSON)}); err != nil {
		return nil, err
	}

	outputs, _, err := s.waitClientToolOutputs(callByID)
	if err != nil {
		s.persistClientToolInterruptedResults(pendingToolCalls(calls), step, err)
		return nil, err
	}
	if err := model.UpdateReactRunByRunID(s.ctx, s.runID, map[string]interface{}{"state": model.ReactRunStateRunning, "pending_tool_use_ids": "[]"}); err != nil {
		return nil, err
	}

	results := make(map[int]llm.ToolResultContent, len(calls))
	for toolUseID, output := range outputs {
		item := callByID[toolUseID]
		meta, err := normalizeClientToolMeta(output.Meta)
		if err != nil {
			return nil, fmt.Errorf("invalid client tool meta for %s: %w", toolUseID, err)
		}
		content := clientToolOutputContent(output.Content)
		normalized := normalizeToolResult(toolUseID, content, output.IsError, executedByClient)
		if normalized.ResultRef != "" {
			if err := storeResultRef(s.ctx, s.sessionID, s.runID, toolUseID, normalized.ResultRef, content); err != nil {
				return nil, err
			}
		}
		_ = clientEmitter.EmitStep(step, EventClientToolUseEnd, params.ReactClientToolUseEndPayload{
			ToolOutputs: []params.ReactClientToolOutput{{
				ToolUseID: toolUseID,
				Content:   normalized.Content,
				Meta:      meta,
				IsError:   normalized.IsError,
				Status:    normalized.Status,
			}},
		})
		results[item.index] = llm.ToolResultContent{ToolUseID: toolUseID, Content: normalized.LLMContent(), IsError: normalized.IsError, Meta: meta}
	}
	return results, nil
}

// executeClientTool 保留单工具执行路径，当前批量路径会优先处理同一步多个 client tool。
func (s *reactEngineState) executeClientTool(call llm.ToolCall, tool model.Tool, step int, description string) (llm.ToolResultContent, error) {
	clientEmitter := s.clientToolEmitter()
	_ = clientEmitter.EmitStep(step, EventClientToolUseStart, params.ReactClientToolUseStartPayload{ToolUseID: call.ID, ToolName: tool.Name, ToolInput: json.RawMessage(call.Input), Description: strings.TrimSpace(description), FrontendHint: clientToolFrontendHint(tool), Status: toolExecutionStatusWaiting})
	pendingJSON, _ := json.Marshal([]string{call.ID})
	if err := model.UpdateReactRunByRunID(s.ctx, s.runID, map[string]interface{}{"state": model.ReactRunStateWaitingClientMessage, "pending_tool_use_ids": string(pendingJSON)}); err != nil {
		return llm.ToolResultContent{}, err
	}

	output, _, err := s.waitClientToolOutput(call.ID)
	if !IsReactRunCancelled(err) {
		_ = model.UpdateReactRunByRunID(s.ctx, s.runID, map[string]interface{}{"state": model.ReactRunStateRunning, "pending_tool_use_ids": "[]"})
	}
	if err != nil {
		s.persistClientToolInterruptedResults([]llm.ToolCall{call}, step, err)
		return llm.ToolResultContent{}, err
	}
	meta, err := normalizeClientToolMeta(output.Meta)
	if err != nil {
		return llm.ToolResultContent{}, fmt.Errorf("invalid client tool meta for %s: %w", call.ID, err)
	}

	content := string(output.Content)
	if !json.Valid(output.Content) {
		content = strings.Trim(string(output.Content), "\"")
	}
	normalized := normalizeToolResult(call.ID, content, output.IsError, executedByClient)
	if normalized.ResultRef != "" {
		if err := storeResultRef(s.ctx, s.sessionID, s.runID, call.ID, normalized.ResultRef, content); err != nil {
			return llm.ToolResultContent{}, err
		}
	}
	_ = clientEmitter.EmitStep(step, EventClientToolUseEnd, params.ReactClientToolUseEndPayload{
		ToolOutputs: []params.ReactClientToolOutput{{
			ToolUseID: call.ID,
			Content:   normalized.Content,
			Meta:      meta,
			IsError:   normalized.IsError,
			Status:    normalized.Status,
		}},
	})
	return llm.ToolResultContent{ToolUseID: call.ID, Content: normalized.LLMContent(), IsError: normalized.IsError, Meta: meta}, nil
}

func (s *reactEngineState) clientToolEmitter() *runEventEmitter {
	return s.emitter
}

// waitClientToolOutput 等待单个 client tool 回填；当前主要由兼容单工具路径使用。
func (s *reactEngineState) waitClientToolOutput(toolUseID string) (clientToolOutput, int64, error) {
	if s.readClient == nil {
		return clientToolOutput{}, 0, fmt.Errorf("client tool requires websocket reader")
	}
	start := time.Now()
	for {
		msg, err := s.readClient()
		if err != nil {
			if !IsReactClientDisconnected(err) {
				_ = model.UpdateReactRunByRunID(s.ctx, s.runID, map[string]interface{}{"state": model.ReactRunStateExpired})
			}
			return clientToolOutput{}, 0, err
		}
		switch msg.Type {
		case EventCancel:
			return clientToolOutput{}, 0, ErrReactRunCancelled
		case EventClientToolUseEnd:
			var payload struct {
				ToolOutputs []clientToolOutput `json:"toolOutputs"`
			}
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return clientToolOutput{}, 0, err
			}
			for _, output := range payload.ToolOutputs {
				if output.ToolUseID == toolUseID {
					return output, time.Since(start).Milliseconds(), nil
				}
			}
			return clientToolOutput{}, 0, fmt.Errorf("client tool output missing: %s", toolUseID)
		default:
			return clientToolOutput{}, 0, fmt.Errorf("unexpected message while waiting client tool: %s", msg.Type)
		}
	}
}

// waitClientToolOutputs 在 run 进入 waiting_client_message 后阻塞读取前端回填，直到所有 pending tool 都有结果。
func (s *reactEngineState) waitClientToolOutputs(callByID map[string]reactClientToolCall) (map[string]clientToolOutput, int64, error) {
	if s.readClient == nil {
		return nil, 0, fmt.Errorf("client tool requires websocket reader")
	}
	start := time.Now()
	for {
		msg, err := s.readClient()
		if err != nil {
			if !IsReactClientDisconnected(err) {
				_ = model.UpdateReactRunByRunID(s.ctx, s.runID, map[string]interface{}{"state": model.ReactRunStateExpired})
			}
			return nil, 0, err
		}
		switch msg.Type {
		case EventCancel:
			return nil, 0, ErrReactRunCancelled
		case EventClientToolUseEnd:
			var payload struct {
				ToolOutputs []clientToolOutput `json:"toolOutputs"`
			}
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return nil, 0, err
			}

			// 只接受当前 pending 集合中的 toolUseID，额外结果会被忽略，缺失结果会整体失败。
			outputs := make(map[string]clientToolOutput, len(payload.ToolOutputs))
			for _, output := range payload.ToolOutputs {
				if _, ok := callByID[output.ToolUseID]; ok {
					outputs[output.ToolUseID] = output
				}
			}
			for toolUseID := range callByID {
				if _, ok := outputs[toolUseID]; !ok {
					return nil, 0, fmt.Errorf("client tool output missing: %s", toolUseID)
				}
			}
			return outputs, time.Since(start).Milliseconds(), nil
		default:
			return nil, 0, fmt.Errorf("unexpected message while waiting client tool: %s", msg.Type)
		}
	}
}

// pendingToolCalls 从批量 client tool 调用中取出原始 ToolCall 列表，供取消补记录复用。
func pendingToolCalls(calls []reactClientToolCall) []llm.ToolCall {
	toolCalls := make([]llm.ToolCall, 0, len(calls))
	for _, item := range calls {
		toolCalls = append(toolCalls, item.call)
	}
	return toolCalls
}

// persistClientToolInterruptedResults 在等待回填被取消或断线时，为所有 pending client tool 补终态 tool_result 落库，
// 让历史消息能看出工具未返回结果；用户取消记为 cancelled，断连仍记为 error。
// 取消时连接仍在，额外补发 client_tool_use_end 让前端工具卡片收敛；落库失败不阻断原有流程。
func (s *reactEngineState) persistClientToolInterruptedResults(calls []llm.ToolCall, step int, waitErr error) {
	status, content, interrupted := classifyToolInterruption(s.runCtx, waitErr)
	if len(calls) == 0 || !interrupted {
		return
	}
	if status == toolExecutionStatusError {
		content = "连接断开，服务端未收到 Client Tool 结果；客户端操作可能尚未执行，也可能已经执行但响应丢失。恢复后若工具存在副作用，不得直接重复调用，应先检查当前前端状态或向用户确认。"
	} else {
		content = "用户取消了本次运行，客户端工具未返回结果。"
	}
	results := make([]llm.ToolResultContent, 0, len(calls))
	toolOutputs := make([]params.ReactClientToolOutput, 0, len(calls))
	for _, call := range calls {
		normalized := normalizeToolResult(call.ID, content, true, executedByClient)
		normalized.Status = status
		results = append(results, llm.ToolResultContent{ToolUseID: call.ID, Content: normalized.LLMContent(), IsError: true})
		toolOutputs = append(toolOutputs, params.ReactClientToolOutput{ToolUseID: call.ID, Content: normalized.Content, IsError: true, Status: status})
	}
	if _, err := persistToolResultMessage(s.ctx, s.req, s.runID, s.sessionID, results, step); err != nil {
		return
	}
	if status == toolExecutionStatusCancelled {
		_ = s.clientToolEmitter().EmitStep(step, EventClientToolUseEnd, params.ReactClientToolUseEndPayload{ToolOutputs: toolOutputs})
	}
}

// clientToolOutputContent 将前端回填内容统一转成字符串，兼容 JSON 和普通字符串两类 payload。
func clientToolOutputContent(content json.RawMessage) string {
	if !json.Valid(content) {
		return strings.Trim(string(content), "\"")
	}
	return string(content)
}

// normalizeClientToolMeta 只接受有限大小的 JSON object；该数据仅用于 UI 回放，不进入模型上下文。
func normalizeClientToolMeta(meta json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(meta)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if len(trimmed) > maxClientToolMetaBytes {
		return nil, fmt.Errorf("meta exceeds %d bytes", maxClientToolMetaBytes)
	}

	var value map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, fmt.Errorf("meta must be a JSON object: %w", err)
	}
	if value == nil {
		return nil, fmt.Errorf("meta must be a JSON object")
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(normalized), nil
}

// clientToolFrontendHint 从工具配置中读取前端展示提示，未配置时退回工具名。
func clientToolFrontendHint(tool model.Tool) string {
	cfg, err := toolService.ParseToolConfig(tool.Config)
	if err == nil && strings.TrimSpace(cfg.FrontendHint) != "" {
		return strings.TrimSpace(cfg.FrontendHint)
	}
	return tool.Name
}
