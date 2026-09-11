package react

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	llm "react-base-service/api/llm"
	"react-base-service/components/params"
	model "react-base-service/models/llm"
)

func TestNormalizeClientToolMeta(t *testing.T) {
	meta, err := normalizeClientToolMeta(json.RawMessage(`{"version":1,"diff":{"original":"a","modified":"b"}}`))
	if err != nil {
		t.Fatalf("normalize valid meta: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(meta, &decoded); err != nil || decoded["version"] != float64(1) {
		t.Fatalf("unexpected normalized meta: %s, err=%v", string(meta), err)
	}

	if _, err := normalizeClientToolMeta(json.RawMessage(`[]`)); err == nil {
		t.Fatal("expected array meta to be rejected")
	}
	if _, err := normalizeClientToolMeta(json.RawMessage(`{"value":"` + strings.Repeat("x", maxClientToolMetaBytes) + `"}`)); err == nil {
		t.Fatal("expected oversized meta to be rejected")
	}
}

func TestClientToolMetaPersistsOutsideModelMessageAndReplays(t *testing.T) {
	const toolUseID = "call_1"
	meta := json.RawMessage(`{"version":1,"original":"select 1","modified":"select 2"}`)
	normalized := NormalizedToolResult{
		ToolUseID:  toolUseID,
		Content:    "SQL 修改成功",
		IsError:    false,
		ExecutedBy: executedByClient,
		Status:     toolExecutionStatusSuccess,
	}
	contentJSON, err := marshalStoredToolResultContent([]llm.ToolResultContent{{
		ToolUseID: toolUseID,
		Content:   normalized.LLMContent(),
		Meta:      meta,
	}})
	if err != nil {
		t.Fatalf("marshal stored tool result: %v", err)
	}

	modelMessage, ok := parseStoredModelMessage(string(contentJSON))
	if !ok || len(modelMessage.Parts) != 1 {
		t.Fatalf("unexpected stored model message: %#v", modelMessage)
	}
	if strings.Contains(modelMessage.Parts[0].Content, "select 1") || strings.Contains(modelMessage.Parts[0].Content, "select 2") {
		t.Fatalf("tool meta leaked into model content: %s", modelMessage.Parts[0].Content)
	}

	storedMeta := parseHistoryToolMeta(string(contentJSON))
	if string(storedMeta[toolUseID]) != string(meta) {
		t.Fatalf("unexpected stored tool meta: %s", string(storedMeta[toolUseID]))
	}

	builder := newHistoryEventBuilder("session_1", nil, nil)
	builder.appendToolResultEvents(model.ReactMessage{
		RunID:       "run_1",
		SessionID:   "session_1",
		StepIndex:   0,
		ContentJSON: string(contentJSON),
		CreatedAt:   time.Now(),
	})
	if len(builder.events) != 1 || builder.events[0].Type != EventClientToolUseEnd {
		t.Fatalf("unexpected replay events: %#v", builder.events)
	}
	payload, ok := builder.events[0].Payload.(params.ReactClientToolUseEndPayload)
	if !ok || len(payload.ToolOutputs) != 1 {
		t.Fatalf("unexpected replay payload: %#v", builder.events[0].Payload)
	}
	if string(payload.ToolOutputs[0].Meta) != string(meta) {
		t.Fatalf("unexpected replay meta: %s", string(payload.ToolOutputs[0].Meta))
	}
	if payload.ToolOutputs[0].Status != toolExecutionStatusSuccess {
		t.Fatalf("unexpected replay status: %s", payload.ToolOutputs[0].Status)
	}
}

func TestClientToolCancelledStatusSurvivesHistoryReplay(t *testing.T) {
	normalized := normalizeToolResult("call_cancelled", "用户取消了本次运行，客户端工具未返回结果。", true, executedByClient)
	normalized.Status = toolExecutionStatusCancelled
	contentJSON, err := marshalStoredToolResultContent([]llm.ToolResultContent{{
		ToolUseID: normalized.ToolUseID,
		Content:   normalized.LLMContent(),
		IsError:   true,
	}})
	if err != nil {
		t.Fatalf("marshal cancelled result: %v", err)
	}

	builder := newHistoryEventBuilder("session_1", nil, nil)
	builder.appendToolResultEvents(model.ReactMessage{
		RunID: "run_1", SessionID: "session_1", StepIndex: 0,
		ContentJSON: string(contentJSON), CreatedAt: time.Now(),
	})
	if len(builder.events) != 1 {
		t.Fatalf("unexpected replay events: %#v", builder.events)
	}
	payload := builder.events[0].Payload.(params.ReactClientToolUseEndPayload)
	if payload.ToolOutputs[0].Status != toolExecutionStatusCancelled || !payload.ToolOutputs[0].IsError {
		t.Fatalf("unexpected cancelled replay output: %#v", payload.ToolOutputs[0])
	}
}
