package react

import (
	"encoding/json"
	"strings"
	"testing"

	llm "react-base-service/api/llm"
	"react-base-service/components/params"
	model "react-base-service/models/llm"

	"github.com/gin-gonic/gin"
)

func TestDisplayFilesToolDefinition(t *testing.T) {
	def := displayFilesToolDefinition()
	if def.Name != metaToolDisplayFiles {
		t.Fatalf("unexpected tool name: %q", def.Name)
	}
	if !strings.Contains(def.Description, "必须调用") {
		t.Fatalf("displayFiles description should require file display through the tool: %s", def.Description)
	}
	// 模型侧契约只有 artifactIds，不暴露 uri（uri 由后端铸造，防止模型拼链接/改写地址）。
	if strings.Contains(def.Description, "uri 原值") {
		t.Fatalf("displayFiles description should not ask model for uri: %s", def.Description)
	}
	properties, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected object properties: %#v", def.Parameters)
	}
	artifactIDs, ok := properties["artifactIds"].(map[string]interface{})
	if !ok || artifactIDs["minItems"] != 1 {
		t.Fatalf("expected non-empty artifactIds array: %#v", properties["artifactIds"])
	}
	required, ok := def.Parameters["required"].([]string)
	if !ok || !stringSliceContains(required, "artifactIds") {
		t.Fatalf("artifactIds must be required: %#v", def.Parameters["required"])
	}
}

// stubDisplayFilesLookup 注入产物查库桩，测试结束自动恢复。
func stubDisplayFilesLookup(t *testing.T, records map[string]*model.ReactArtifact) {
	t.Helper()
	orig := displayFilesArtifactLookup
	t.Cleanup(func() { displayFilesArtifactLookup = orig })
	displayFilesArtifactLookup = func(ctx *gin.Context, artifactID string) (*model.ReactArtifact, error) {
		return records[artifactID], nil
	}
}

// stubDisplayFilesStoreResultRef 注入落库桩（单测无 DB），测试结束自动恢复。
func stubDisplayFilesStoreResultRef(t *testing.T) {
	t.Helper()
	orig := displayFilesStoreResultRef
	t.Cleanup(func() { displayFilesStoreResultRef = orig })
	displayFilesStoreResultRef = func(ctx *gin.Context, sessionID, runID, toolUseID, resultRef, content string) error {
		return nil
	}
}

// newDisplayFilesTestState 构造带事件捕获 emitter 的引擎状态。
func newDisplayFilesTestState(events *[]params.ReactEvent) *reactEngineState {
	return &reactEngineState{
		sessionID: "sess",
		runID:     "run",
		req:       &runtimeRequest{},
		emitter: &runEventEmitter{
			runID:     "run",
			sessionID: "sess",
			write: func(event params.ReactEvent) error {
				*events = append(*events, event)
				return nil
			},
		},
	}
}

func TestExecuteDisplayFilesToolMintsInputBeforeStart(t *testing.T) {
	stubDisplayFilesLookup(t, map[string]*model.ReactArtifact{
		"image1": {ArtifactID: "image1", SessionID: "sess", MimeType: "image/png", FileName: "chart.png"},
		"pdf1":   {ArtifactID: "pdf1", SessionID: "sess", MimeType: "application/pdf", FileName: "report.pdf"},
	})
	stubDisplayFilesStoreResultRef(t)
	var events []params.ReactEvent
	s := newDisplayFilesTestState(&events)

	result, err := s.executeDisplayFilesTool(llm.ToolCall{
		ID:    "call_1",
		Name:  metaToolDisplayFiles,
		Input: json.RawMessage(`{"description":"展示图表","artifactIds":["image1","pdf1"]}`),
	}, 1)
	if err != nil || result.IsError {
		t.Fatalf("valid displayFiles call failed: result=%+v err=%v", result, err)
	}
	// result.Content 是 LLMContent 信封，内层 JSON 引号被转义。
	if !strings.Contains(result.Content, `\"displayed\":2`) {
		t.Fatalf("unexpected result content: %s", result.Content)
	}

	// tool_use_start 的 input 必须是后端铸造的 {"artifacts":[{type,name,uri}]}，不是模型的 artifactIds。
	if len(events) != 2 || events[0].Type != EventToolUseStart || events[1].Type != EventToolUseEnd {
		t.Fatalf("expected start+end events, got %+v", events)
	}
	startPayload, ok := events[0].Payload.(params.ReactToolUseStartPayload)
	if !ok {
		t.Fatalf("unexpected start payload type: %#v", events[0].Payload)
	}
	var minted pythonExecArtifactMeta
	if err := json.Unmarshal(startPayload.ToolInput, &minted); err != nil {
		t.Fatalf("start input should be minted artifacts json: %v\n%s", err, string(startPayload.ToolInput))
	}
	if len(minted.Artifacts) != 2 ||
		minted.Artifacts[0].URI != pythonExecArtifactURLPrefix+"image1" ||
		minted.Artifacts[0].Type != "image/png" || minted.Artifacts[0].Name != "chart.png" ||
		minted.Artifacts[1].URI != pythonExecArtifactURLPrefix+"pdf1" {
		t.Fatalf("start input should carry backend-built artifacts: %+v", minted.Artifacts)
	}
	if strings.Contains(string(startPayload.ToolInput), "artifactIds") {
		t.Fatalf("start input must not keep raw artifactIds: %s", string(startPayload.ToolInput))
	}
	if startPayload.Description != "展示图表" {
		t.Fatalf("description should be extracted: %q", startPayload.Description)
	}

	// 铸造结果同时作为 toolMeta 随 end 事件下发并落库（回放时替换 start input）。
	endPayload, ok := events[1].Payload.(params.ReactToolUseEndPayload)
	if !ok || string(endPayload.Meta) != string(startPayload.ToolInput) {
		t.Fatalf("end meta should mirror minted input: %#v", events[1].Payload)
	}
	if string(result.Meta) != string(startPayload.ToolInput) {
		t.Fatalf("result meta should carry minted artifacts for persistence: %s", string(result.Meta))
	}
}

func TestExecuteDisplayFilesToolRejectsInvalidArtifacts(t *testing.T) {
	stubDisplayFilesLookup(t, map[string]*model.ReactArtifact{
		"other-sess": {ArtifactID: "other-sess", SessionID: "someone-else", MimeType: "image/png", FileName: "x.png"},
	})
	stubDisplayFilesStoreResultRef(t)

	cases := []string{
		`{"artifactIds":[]}`,
		`{"artifactIds":[""]}`,
		// 编造/抄错的 ID：查不到必须报错，防止假 uri 透传前端。
		`{"artifactIds":["no-such-id"]}`,
		// 跨会话引用：归属校验必须拦截。
		`{"artifactIds":["other-sess"]}`,
	}
	for _, input := range cases {
		var events []params.ReactEvent
		s := newDisplayFilesTestState(&events)
		result, err := s.executeDisplayFilesTool(llm.ToolCall{ID: "call_1", Name: metaToolDisplayFiles, Input: json.RawMessage(input)}, 1)
		if err != nil || !result.IsError || result.Meta != nil {
			t.Fatalf("expected error result without meta for %s, got %+v err=%v", input, result, err)
		}
		// 失败时 start 事件保留模型原始 input（artifactIds 形状渲染不出产物），不发假数据。
		startPayload := events[0].Payload.(params.ReactToolUseStartPayload)
		if strings.Contains(string(startPayload.ToolInput), `"artifacts"`) {
			t.Fatalf("failed call must not mint artifacts input: %s", string(startPayload.ToolInput))
		}
	}
}
