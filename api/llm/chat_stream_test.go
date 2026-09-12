package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"react-base-service/conf"
)

func TestMiniMaxChatStreamReturnsErrorOnEOFWithoutDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"SELECT 1\"}}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2}}\n\n")
	}))
	defer server.Close()

	client := NewMiniMaxClient("test", "model", conf.EndpointConfig{ApiUrl: server.URL, MaxTokens: 128})
	ch, err := client.ChatStream(context.Background(), []LLMMessage{{Role: "user", Content: "hi"}}, "")
	if err != nil {
		t.Fatalf("ChatStream returned error: %v", err)
	}

	chunks := collectStreamChunks(ch)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Content != "SELECT 1" {
		t.Fatalf("unexpected content chunk: %#v", chunks[0])
	}
	final := chunks[1]
	if final.Error == nil {
		t.Fatalf("expected final chunk error, got %#v", final)
	}
	if final.TerminationReason != StreamTerminationUpstreamEOFWithoutDone {
		t.Fatalf("unexpected termination reason: %#v", final)
	}
	if final.ReceivedDone {
		t.Fatalf("expected received_done=false, got %#v", final)
	}
}

func TestMiniMaxChatStreamAcceptsStopFinishReasonWithoutDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"SELECT 1\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2}}\n\n")
	}))
	defer server.Close()

	client := NewMiniMaxClient("test", "model", conf.EndpointConfig{ApiUrl: server.URL, MaxTokens: 128})
	ch, err := client.ChatStream(context.Background(), []LLMMessage{{Role: "user", Content: "hi"}}, "")
	if err != nil {
		t.Fatalf("ChatStream returned error: %v", err)
	}

	chunks := collectStreamChunks(ch)
	final := chunks[len(chunks)-1]
	if final.Error != nil {
		t.Fatalf("expected successful completion, got %#v", final)
	}
	if !final.Done || final.TerminationReason != StreamTerminationCompleted || final.FinishReason != "stop" {
		t.Fatalf("unexpected final chunk: %#v", final)
	}
}

type gptRequestCapture struct {
	Model               string     `json:"model"`
	MaxCompletionTokens int        `json:"max_completion_tokens"`
	ToolChoice          ToolChoice `json:"tool_choice"`
}

func TestGPTClientAppliesModelMaxCompletionTokensCap(t *testing.T) {
	// 用例自包含：显式注入 model_version_limits 并在结束后恢复，不依赖全局配置加载。
	oldLimits := conf.CustomConf.LLM.ModelVersionLimits
	conf.CustomConf.LLM.ModelVersionLimits = map[string]conf.ModelVersionLimit{
		"deepseek-v3": {MaxCompletionTokens: 16384},
	}
	t.Cleanup(func() {
		conf.CustomConf.LLM.ModelVersionLimits = oldLimits
	})

	t.Run("chat_stream", func(t *testing.T) {
		var captured gptRequestCapture
		server := newGPTCaptureServer(t, &captured)
		defer server.Close()

		client := NewGPTClient("test", "", conf.EndpointConfig{ApiUrl: server.URL, MaxTokens: 32768})
		ch, err := client.ChatStream(context.Background(), []LLMMessage{{Role: "user", Content: "hi"}}, "deepseek-v3")
		if err != nil {
			t.Fatalf("ChatStream returned error: %v", err)
		}
		_ = collectStreamChunks(ch)

		assertRequestMaxCompletionTokensCap(t, captured, "deepseek-v3", 16384)
	})

	t.Run("chat_stream_with_file_payloads", func(t *testing.T) {
		var captured gptRequestCapture
		server := newGPTCaptureServer(t, &captured)
		defer server.Close()

		client := NewGPTClient("test", "", conf.EndpointConfig{ApiUrl: server.URL, MaxTokens: 32768})
		ch, err := client.ChatStreamWithFilePayloads(
			context.Background(),
			[]LLMMessage{{Role: "user", Content: "hi"}},
			"deepseek-v3",
			[]FilePayload{{FileName: "demo.txt", ContentAs: "text", Encoding: "text", Text: "hello"}},
		)
		if err != nil {
			t.Fatalf("ChatStreamWithFilePayloads returned error: %v", err)
		}
		_ = collectStreamChunks(ch)

		assertRequestMaxCompletionTokensCap(t, captured, "deepseek-v3", 16384)
	})

	t.Run("chat_stream_with_tools", func(t *testing.T) {
		var captured gptRequestCapture
		server := newGPTCaptureServer(t, &captured)
		defer server.Close()

		client := NewGPTClient("test", "", conf.EndpointConfig{ApiUrl: server.URL, MaxTokens: 32768})
		ch, err := client.ChatStreamWithTools(
			context.Background(),
			[]ChatMessage{{Role: "user", Content: "hi"}},
			"deepseek-v3",
			[]ToolDefinition{{
				Name:        "demo_tool",
				Description: "demo tool",
				Parameters: map[string]interface{}{
					"type": "object",
				},
			}},
		)
		if err != nil {
			t.Fatalf("ChatStreamWithTools returned error: %v", err)
		}
		_ = collectStreamChunks(ch)

		assertRequestMaxCompletionTokensCap(t, captured, "deepseek-v3", 16384)
	})
}

func TestGPTToolRequestIncludesToolChoiceNone(t *testing.T) {
	ctx := WithToolChoice(context.Background(), ToolChoiceNone)
	body, err := json.Marshal(gptToolRequest{Model: "gpt-test", Stream: true, ToolChoice: toolChoiceFromContext(ctx)})
	if err != nil {
		t.Fatalf("marshal gptToolRequest failed: %v", err)
	}
	if !strings.Contains(string(body), `"tool_choice":"none"`) {
		t.Fatalf("expected tool_choice=none, body=%s", string(body))
	}
}

func TestClaudeToolUsePartKeepsEmptyInputObject(t *testing.T) {
	part := claudeContentPart{
		Type:  "tool_use",
		ID:    "toolu_empty",
		Name:  "demo_tool",
		Input: normalizeClaudeToolInput(nil),
	}
	body, err := json.Marshal(part)
	if err != nil {
		t.Fatalf("marshal tool_use failed: %v", err)
	}

	var toolUse map[string]interface{}
	if err := json.Unmarshal(body, &toolUse); err != nil {
		t.Fatalf("unmarshal tool_use failed: %v", err)
	}
	input, ok := toolUse["input"].(map[string]interface{})
	if !ok || len(input) != 0 {
		t.Fatalf("expected empty input object, got %#v", toolUse["input"])
	}
}

func TestNormalizeClaudeToolInput(t *testing.T) {
	cases := []struct {
		name string
		in   json.RawMessage
		want string
	}{
		{name: "empty", in: nil, want: `{}`},
		{name: "invalid", in: json.RawMessage(`{invalid`), want: `{}`},
		{name: "valid", in: json.RawMessage(`{"query":"demo"}`), want: `{"query":"demo"}`},
		{name: "valid with whitespace", in: json.RawMessage(`{"skillId": "skill_demo_test_d1e8a07b634724521f80ac81"}`), want: `{"skillId":"skill_demo_test_d1e8a07b634724521f80ac81"}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeClaudeToolInput(tt.in)
			if string(got) != tt.want {
				t.Fatalf("unexpected normalized input: got=%s want=%s", string(got), tt.want)
			}
		})
	}
}

func TestClaudeParseToolStreamNormalizesEmptyToolInput(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_empty","name":"demo_tool"}}`,
			`data: {"type":"content_block_stop","index":0}`,
			`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
			`data: {"type":"message_stop"}`,
		}, "\n\n"))),
	}
	ch := make(chan StreamChunk, 4)
	client := NewClaudeClient("test", "claude-sonnet-4", conf.EndpointConfig{})
	client.parseToolStream(context.Background(), resp, ch)

	chunks := collectStreamChunks(ch)
	final := chunks[len(chunks)-1]
	if len(final.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %#v", final.ToolCalls)
	}
	if string(final.ToolCalls[0].Input) != `{}` {
		t.Fatalf("expected empty input object, got %s", string(final.ToolCalls[0].Input))
	}
}

func TestGPTParseToolStreamKeepsToolCallWhenFinishReasonChangesToStop(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_demo","type":"function","function":{"name":"demo_tool","arguments":"{\"query\""}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"hello\"}"}}]},"finish_reason":"tool_calls"}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}, "\n\n"))),
	}
	ch := make(chan StreamChunk, 4)
	client := NewGPTClient("test", "gpt-test", conf.EndpointConfig{})
	client.parseGPTToolStream(context.Background(), resp, ch)

	chunks := collectStreamChunks(ch)
	final := chunks[len(chunks)-1]
	if final.StopReason != "tool_use" {
		t.Fatalf("expected tool_use stop reason, got %#v", final)
	}
	if len(final.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %#v", final.ToolCalls)
	}
	call := final.ToolCalls[0]
	if call.ID != "call_demo" || call.Name != "demo_tool" || string(call.Input) != `{"query":"hello"}` {
		t.Fatalf("unexpected tool call: %#v", call)
	}
}

func TestGPTToolMessagesMapsThinkingToReasoningContent(t *testing.T) {
	messages := []ChatMessage{{
		Role: "assistant",
		Parts: []ContentPart{
			{Type: "thinking", Thinking: "需要先查询技能"},
			{Type: "tool_use", ID: "list_skills:0", Name: "list_skills", Input: json.RawMessage(`{}`)},
		},
	}}

	got := buildGPTToolMessages(messages)
	if len(got) != 1 {
		t.Fatalf("expected one message, got %#v", got)
	}
	msg := got[0]
	if msg.ReasoningContent != "需要先查询技能" {
		t.Fatalf("unexpected reasoning_content: %#v", msg)
	}
	if content, ok := msg.Content.(string); !ok || content != "" {
		t.Fatalf("expected empty string content, got %#v", msg.Content)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ID != "list_skills:0" {
		t.Fatalf("unexpected tool calls: %#v", msg.ToolCalls)
	}

	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal message failed: %v", err)
	}
	if strings.Contains(string(body), `"content":null`) || !strings.Contains(string(body), `"content":""`) {
		t.Fatalf("assistant tool message must use empty string content: %s", string(body))
	}
}

func TestGPTToolMessagesPreservesTextAndToolResult(t *testing.T) {
	messages := []ChatMessage{
		{
			Role: "assistant",
			Parts: []ContentPart{
				{Type: "text", Text: "正在查询"},
				{Type: "tool_use", ID: "call_demo", Name: "demo_tool", Input: json.RawMessage(`{"query":"demo"}`)},
			},
		},
		{
			Role: "user",
			Parts: []ContentPart{{
				Type:      "tool_result",
				ToolUseID: "call_demo",
				Content:   "async task already resolved",
				IsError:   true,
			}},
		},
	}

	got := buildGPTToolMessages(messages)
	if len(got) != 2 {
		t.Fatalf("expected assistant and tool messages, got %#v", got)
	}
	if content, ok := got[0].Content.(string); !ok || content != "正在查询" {
		t.Fatalf("unexpected assistant content: %#v", got[0].Content)
	}
	if got[1].Role != "tool" || got[1].ToolCallID != "call_demo" || got[1].Content != "async task already resolved" {
		t.Fatalf("unexpected tool result message: %#v", got[1])
	}
}

func newGPTCaptureServer(t *testing.T, captured *gptRequestCapture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body failed: %v", err)
		}
		if err := json.Unmarshal(body, captured); err != nil {
			t.Fatalf("unmarshal request body failed: %v, body=%s", err, string(body))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func assertRequestMaxCompletionTokensCap(t *testing.T, captured gptRequestCapture, wantModel string, wantMaxCompletionTokens int) {
	t.Helper()
	if captured.Model != wantModel {
		t.Fatalf("unexpected model: %#v", captured)
	}
	if captured.MaxCompletionTokens != wantMaxCompletionTokens {
		t.Fatalf("unexpected max_completion_tokens: %#v", captured)
	}
}

func collectStreamChunks(ch <-chan StreamChunk) []StreamChunk {
	var chunks []StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}
	return chunks
}
