package react

import (
	"encoding/json"
	"strings"
	"testing"
)

func validAskQuestionInputJSON() json.RawMessage {
	return json.RawMessage(`{
		"questions": [
			{
				"id": "time_range",
				"prompt": "报表统计的时间范围是？",
				"options": [
					{"id": "last_30d", "label": "最近30天（推荐）", "description": "与现有周报口径一致"},
					{"id": "last_90d", "label": "最近90天"}
				]
			},
			{
				"id": "metrics",
				"prompt": "需要包含哪些统计指标？",
				"allowMultiple": true,
				"options": [
					{"id": "pv", "label": "访问量 PV"},
					{"id": "uv", "label": "独立用户 UV"},
					{"id": "avg_duration", "label": "平均停留时长"}
				]
			}
		]
	}`)
}

func TestParseAskQuestionInputAcceptsValidInput(t *testing.T) {
	input, err := parseAskQuestionInput(validAskQuestionInputJSON())
	if err != nil {
		t.Fatalf("parse valid input failed: %v", err)
	}
	if len(input.Questions) != 2 {
		t.Fatalf("unexpected question count: %d", len(input.Questions))
	}
	if input.Questions[0].ID != "time_range" || input.Questions[0].AllowMultiple {
		t.Fatalf("unexpected first question: %+v", input.Questions[0])
	}
	if !input.Questions[1].AllowMultiple || len(input.Questions[1].Options) != 3 {
		t.Fatalf("unexpected second question: %+v", input.Questions[1])
	}
}

func TestParseAskQuestionInputRejectsEmptyQuestions(t *testing.T) {
	_, err := parseAskQuestionInput(json.RawMessage(`{"questions":[]}`))
	if err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("expected empty questions error, got: %v", err)
	}
}

func TestParseAskQuestionInputRejectsTooManyQuestions(t *testing.T) {
	questions := make([]string, 0, askQuestionMaxQuestions+1)
	for i := 0; i < askQuestionMaxQuestions+1; i++ {
		questions = append(questions, `{"id":"q`+string(rune('a'+i))+`","prompt":"问题？","options":[{"id":"a","label":"A"},{"id":"b","label":"B"}]}`)
	}
	raw := json.RawMessage(`{"questions":[` + strings.Join(questions, ",") + `]}`)
	_, err := parseAskQuestionInput(raw)
	if err == nil || !strings.Contains(err.Error(), "must not exceed") {
		t.Fatalf("expected too many questions error, got: %v", err)
	}
}

func TestParseAskQuestionInputRejectsDuplicateQuestionID(t *testing.T) {
	raw := json.RawMessage(`{"questions":[
		{"id":"q1","prompt":"问题一？","options":[{"id":"a","label":"A"},{"id":"b","label":"B"}]},
		{"id":"q1","prompt":"问题二？","options":[{"id":"a","label":"A"},{"id":"b","label":"B"}]}
	]}`)
	_, err := parseAskQuestionInput(raw)
	if err == nil || !strings.Contains(err.Error(), "duplicate question id") {
		t.Fatalf("expected duplicate question id error, got: %v", err)
	}
}

func TestParseAskQuestionInputRejectsTooFewOptions(t *testing.T) {
	raw := json.RawMessage(`{"questions":[{"id":"q1","prompt":"问题？","options":[{"id":"a","label":"A"}]}]}`)
	_, err := parseAskQuestionInput(raw)
	if err == nil || !strings.Contains(err.Error(), "options") {
		t.Fatalf("expected option count error, got: %v", err)
	}
}

func TestParseAskQuestionInputRejectsDuplicateOptionID(t *testing.T) {
	raw := json.RawMessage(`{"questions":[{"id":"q1","prompt":"问题？","options":[{"id":"a","label":"A"},{"id":"a","label":"B"}]}]}`)
	_, err := parseAskQuestionInput(raw)
	if err == nil || !strings.Contains(err.Error(), "duplicate option id") {
		t.Fatalf("expected duplicate option id error, got: %v", err)
	}
}

func TestParseAskQuestionInputRejectsMissingPromptAndLabel(t *testing.T) {
	raw := json.RawMessage(`{"questions":[{"id":"q1","prompt":"  ","options":[{"id":"a","label":"A"},{"id":"b","label":"B"}]}]}`)
	if _, err := parseAskQuestionInput(raw); err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("expected prompt error, got: %v", err)
	}
	raw = json.RawMessage(`{"questions":[{"id":"q1","prompt":"问题？","options":[{"id":"a","label":" "},{"id":"b","label":"B"}]}]}`)
	if _, err := parseAskQuestionInput(raw); err == nil || !strings.Contains(err.Error(), "label is required") {
		t.Fatalf("expected label error, got: %v", err)
	}
}

func TestRenderAskQuestionResultResolvesLabelsByOptionID(t *testing.T) {
	input, err := parseAskQuestionInput(validAskQuestionInputJSON())
	if err != nil {
		t.Fatalf("parse valid input failed: %v", err)
	}
	answer := askQuestionAnswer{
		Answers: []askQuestionAnswerItem{
			{QuestionID: "time_range", SelectedOptionIDs: []string{"last_30d"}},
			{QuestionID: "metrics", SelectedOptionIDs: []string{"pv", "uv"}, FreeText: "另外加一个新用户注册数"},
		},
	}
	var result askQuestionResult
	if err := json.Unmarshal([]byte(renderAskQuestionResult(input, answer)), &result); err != nil {
		t.Fatalf("result should be valid JSON: %v", err)
	}
	if result.Skipped || result.Note != "" {
		t.Fatalf("unexpected skipped/note: %+v", result)
	}
	if len(result.Answers) != 2 {
		t.Fatalf("unexpected answers count: %+v", result.Answers)
	}
	if result.Answers[0].QuestionID != "time_range" || len(result.Answers[0].Selected) != 1 || result.Answers[0].Selected[0].Label != "最近30天（推荐）" {
		t.Fatalf("label should be resolved from backend questions: %+v", result.Answers[0])
	}
	if len(result.Answers[1].Selected) != 2 || result.Answers[1].FreeText != "另外加一个新用户注册数" {
		t.Fatalf("multi-select answer should keep order and freeText: %+v", result.Answers[1])
	}
}

func TestRenderAskQuestionResultHandlesSkip(t *testing.T) {
	input, _ := parseAskQuestionInput(validAskQuestionInputJSON())
	var result askQuestionResult
	if err := json.Unmarshal([]byte(renderAskQuestionResult(input, askQuestionAnswer{Skipped: true})), &result); err != nil {
		t.Fatalf("result should be valid JSON: %v", err)
	}
	if !result.Skipped || result.Cancelled || !strings.Contains(result.Note, "跳过") {
		t.Fatalf("unexpected skip result: %+v", result)
	}
	// 跳过时保留原始问题列表（selected 为空），历史回放才能看到当时问了什么。
	if len(result.Answers) != 2 {
		t.Fatalf("skip result should keep question list: %+v", result.Answers)
	}
	if result.Answers[0].Prompt != "报表统计的时间范围是？" || len(result.Answers[0].Selected) != 0 {
		t.Fatalf("skip answer should keep prompt with empty selection: %+v", result.Answers[0])
	}
}

func TestRenderAskQuestionCancelledResult(t *testing.T) {
	input, _ := parseAskQuestionInput(validAskQuestionInputJSON())
	var result askQuestionResult
	if err := json.Unmarshal([]byte(renderAskQuestionCancelledResult(input, ErrReactRunCancelled)), &result); err != nil {
		t.Fatalf("result should be valid JSON: %v", err)
	}
	if !result.Skipped || !result.Cancelled || !strings.Contains(result.Note, "取消") {
		t.Fatalf("unexpected cancelled result: %+v", result)
	}
	if len(result.Answers) != 2 || result.Answers[1].Prompt != "需要包含哪些统计指标？" {
		t.Fatalf("cancelled result should keep question list: %+v", result.Answers)
	}

	if err := json.Unmarshal([]byte(renderAskQuestionCancelledResult(input, ErrReactClientDisconnected)), &result); err != nil {
		t.Fatalf("result should be valid JSON: %v", err)
	}
	if !result.Cancelled || !strings.Contains(result.Note, "断开") {
		t.Fatalf("disconnected result should mention断开: %+v", result)
	}
}

func TestRenderAskQuestionResultNotesUnansweredQuestions(t *testing.T) {
	input, _ := parseAskQuestionInput(validAskQuestionInputJSON())
	answer := askQuestionAnswer{
		Answers: []askQuestionAnswerItem{{QuestionID: "time_range", SelectedOptionIDs: []string{"last_90d"}}},
	}
	var result askQuestionResult
	if err := json.Unmarshal([]byte(renderAskQuestionResult(input, answer)), &result); err != nil {
		t.Fatalf("result should be valid JSON: %v", err)
	}
	if len(result.Answers) != 2 {
		t.Fatalf("all questions should appear in result: %+v", result.Answers)
	}
	if !strings.Contains(result.Note, "metrics") {
		t.Fatalf("note should mention unanswered question: %q", result.Note)
	}
}

func TestResolveAskQuestionOptionsFallsBackToIDAndDedupes(t *testing.T) {
	question := askQuestionItem{
		ID:     "q1",
		Prompt: "问题？",
		Options: []askQuestionOption{
			{ID: "a", Label: "选项A"},
			{ID: "b", Label: "选项B"},
		},
	}
	selected := resolveAskQuestionOptions(question, []string{"a", "a", " ", "unknown"})
	if len(selected) != 2 {
		t.Fatalf("expected dedupe and skip blank ids: %+v", selected)
	}
	if selected[0].Label != "选项A" {
		t.Fatalf("known option should resolve label: %+v", selected[0])
	}
	if selected[1].ID != "unknown" || selected[1].Label != "unknown" {
		t.Fatalf("unknown option should fall back to id: %+v", selected[1])
	}
}

func TestInternalMetaToolIncludesAskQuestion(t *testing.T) {
	if !isInternalMetaTool(metaToolAskQuestion) {
		t.Fatalf("ask_question should be internal meta tool")
	}
	found := false
	for _, def := range internalMetaToolDefinitions() {
		if def.Name == metaToolAskQuestion {
			found = true
		}
	}
	if !found {
		t.Fatalf("ask_question should be in internal tool definitions")
	}
}
