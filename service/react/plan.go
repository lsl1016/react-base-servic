package react

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	llm "react-base-service/api/llm"
)

// create_plan Meta Tool —— 第一期（计划确认交互闭环）。
//
// 流程：模型对复杂任务先调用 create_plan 提交执行计划（title/overview/steps），
// 工具立即返回 planId 并结束本轮；前端 PlanBlock 渲染计划卡片，用户点击
// 「开始任务」后 SDK 以新 run 发送「开始执行刚才的计划」，模型按计划逐步执行，
// 执行进度用 todo_write 维护。计划状态在 SDK 侧由 reducer 维护
// （tool_use_start 置 pending，新 run 启动时置 accepted）。
//
// TODO（第二期）：Plan Executor——按步骤起 scoped ReAct run、重试 attempt、
// plan_view_update / plan_step_event 事件推送与 PlanRuntimeCard 渲染、计划落库。
// 协议类型已完整保留于 web/sdk/protocol/types.ts。

const (
	planStatusPending  = "pending"
	planStatusAccepted = "accepted"
)

type planStepInput struct {
	ID      string `json:"id"`
	Outline string `json:"outline"`
	Message string `json:"message,omitempty"`
}

type createPlanInput struct {
	Description string          `json:"description"`
	Title       string          `json:"title"`
	Overview    string          `json:"overview"`
	Steps       []planStepInput `json:"steps"`
}

// planRecord 计划记录；第一期为进程内存态，第二期升级为落库。
type planRecord struct {
	PlanID    string          `json:"planId"`
	SessionID string          `json:"sessionId"`
	RunID     string          `json:"runId"`
	Title     string          `json:"title"`
	Overview  string          `json:"overview"`
	Steps     []planStepInput `json:"steps"`
	Status    string          `json:"status"`
	CreatedAt string          `json:"createdAt"`
}

var (
	planMu    sync.Mutex
	planStore = map[string]*planRecord{}
)

func createPlanToolDefinition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name: metaToolCreatePlan,
		Description: "提交一份分步执行计划，等待用户在前端确认后才开始执行。适用于步骤较多（建议不少于3步）、包含不可逆或高风险操作（删除、覆盖、外部副作用），或用户明确要求先给方案再动手的任务。调用本工具后本轮立即结束：返回 planId 与 pending 状态，页面向用户展示计划卡片；用户点击「开始任务」后你会收到「开始执行刚才的计划」的新消息，届时再按计划顺序逐项执行，执行进度用 todo_write 维护。用户未确认前不得执行任何步骤；如果用户在后续消息中要求调整计划，先按反馈修订再重新提交。简单任务（一两步、无副作用）不要使用本工具，直接用 todo_write 拆解并执行即可。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": stringSchema("本次工具调用的简短描述，用于向用户说明为什么调用该内部工具或正在做什么。"),
				"title":       stringSchema("计划标题，一句话概括目标。"),
				"overview":    stringSchema("计划总览：思路、关键决策与预期结果，2-4 句。"),
				"steps": map[string]interface{}{
					"type":        "array",
					"description": "计划步骤列表，按执行顺序排列；每步应可独立执行和验证。",
					"minItems":    1,
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id":      stringSchema("稳定步骤 id，例如 collect、clean、analyze、report。"),
							"outline": stringSchema("步骤概要，一句话描述这一步做什么。"),
							"message": stringSchema("步骤补充说明，可选：输入输出、注意事项等。"),
						},
						"required":             []string{"id", "outline"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"description", "title", "steps"},
			"additionalProperties": false,
		},
	}
}

// executeCreatePlan 校验并登记计划，返回给模型与前端卡片的确认载荷。
func executeCreatePlan(sessionID, runID string, input json.RawMessage) (string, bool, error) {
	var req createPlanInput
	if err := json.Unmarshal(input, &req); err != nil {
		return "", true, fmt.Errorf("create_plan input must be valid JSON: %w", err)
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return "", true, fmt.Errorf("create_plan title is required")
	}
	if len(req.Steps) == 0 {
		return "", true, fmt.Errorf("create_plan steps is required")
	}
	steps := make([]planStepInput, 0, len(req.Steps))
	for index, step := range req.Steps {
		step.ID = strings.TrimSpace(step.ID)
		step.Outline = strings.TrimSpace(step.Outline)
		if step.Outline == "" {
			return "", true, fmt.Errorf("create_plan steps[%d].outline is required", index)
		}
		if step.ID == "" {
			step.ID = fmt.Sprintf("step_%d", index+1)
		}
		steps = append(steps, step)
	}

	planID := "plan_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	record := &planRecord{
		PlanID:    planID,
		SessionID: sessionID,
		RunID:     runID,
		Title:     req.Title,
		Overview:  strings.TrimSpace(req.Overview),
		Steps:     steps,
		Status:    planStatusPending,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	planMu.Lock()
	planStore[planID] = record
	planMu.Unlock()

	result := map[string]interface{}{
		"planId":    planID,
		"status":   planStatusPending,
		"stepCount": len(steps),
		"message":  "计划已提交，等待用户确认。用户点击「开始任务」后会发送新消息指示执行；在此之前不要执行任何步骤，也不要再调用 create_plan。",
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", true, err
	}
	return string(data), false, nil
}
