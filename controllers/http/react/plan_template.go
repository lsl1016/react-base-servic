package react

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"react-base-service/components"
	"react-base-service/golib/zlog"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
)

// Plan 模板管理接口 —— TODO 占位实现。
//
// 基座服务不包含 Plan 模式执行器与模板持久化表，这里仅提供进程内存版 CRUD，
// 保证 playground「模板列表」面板可以浏览、新建、编辑、启停且不报错；
// 数据不落库，服务重启后清空。
// 接入方需要真实 Plan 模板能力时，应将 store 替换为落库实现
// （如 tblLlmReactPlanTemplate），并补充模板 JSON Schema 校验与执行侧对接。

type planTemplateRecord struct {
	TemplateID  string          `json:"templateId"`
	CallerKey   string          `json:"callerKey"`
	Description string          `json:"description"`
	Revision    int64           `json:"revision"`
	Status      int             `json:"status"`
	UpdatedBy   string          `json:"updatedBy"`
	UpdatedAt   string          `json:"updatedAt"`
	Template    json.RawMessage `json:"template"`
}

var (
	planTemplateMu    sync.Mutex
	planTemplateStore = map[string]*planTemplateRecord{}
)

func planTemplateKey(callerKey, templateID string) string {
	return callerKey + "/" + templateID
}

// templateDescription 提取模板 JSON 顶层 description 字段，用于列表展示；解析失败不阻断。
func templateDescription(raw json.RawMessage) string {
	var body struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.Description)
}

type planTemplateScopeRequest struct {
	CallerKey  string `json:"callerKey"`
	TemplateID string `json:"templateId"`
}

type planTemplateSaveRequest struct {
	CallerKey   string          `json:"callerKey"`
	TemplateID  string          `json:"templateId"`
	Revision    int64           `json:"revision"`
	Status      int             `json:"status"`
	Template    json.RawMessage `json:"template"`
}

// ListPlanTemplates 列出当前 caller 下的全部 Plan 模板。
// @Summary      Plan 模板列表
// @Description  返回指定 caller 下的 Plan 模板（占位实现，内存数据）
// @Tags         React
// @Accept       json
// @Produce      json
// @Router       /react/plan_template/list [post]
func ListPlanTemplates(ctx *gin.Context) {
	var req planTemplateScopeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		components.RenderJsonFail(ctx, components.ParamInvalidf("请求体解析失败: %s", err.Error()))
		return
	}
	callerKey := strings.TrimSpace(req.CallerKey)
	if callerKey == "" {
		components.RenderJsonFail(ctx, components.ParamInvalidf("callerKey 不能为空"))
		return
	}

	planTemplateMu.Lock()
	items := make([]planTemplateRecord, 0)
	for _, record := range planTemplateStore {
		if record.CallerKey != callerKey {
			continue
		}
		items = append(items, *record)
	}
	planTemplateMu.Unlock()

	sort.Slice(items, func(i, j int) bool { return items[i].TemplateID < items[j].TemplateID })
	components.RenderJsonSucc(ctx, items)
}

// GetPlanTemplateDetail 返回单个 Plan 模板完整内容。
// @Summary      Plan 模板详情
// @Description  按 callerKey + templateId 返回模板完整 JSON（占位实现，内存数据）
// @Tags         React
// @Accept       json
// @Produce      json
// @Router       /react/plan_template/detail [post]
func GetPlanTemplateDetail(ctx *gin.Context) {
	var req planTemplateScopeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		components.RenderJsonFail(ctx, components.ParamInvalidf("请求体解析失败: %s", err.Error()))
		return
	}
	callerKey := strings.TrimSpace(req.CallerKey)
	templateID := strings.TrimSpace(req.TemplateID)
	if callerKey == "" || templateID == "" {
		components.RenderJsonFail(ctx, components.ParamInvalidf("callerKey 与 templateId 不能为空"))
		return
	}

	planTemplateMu.Lock()
	record, ok := planTemplateStore[planTemplateKey(callerKey, templateID)]
	planTemplateMu.Unlock()
	if !ok {
		components.RenderJsonFail(ctx, components.ParamInvalidf("模板不存在: %s", templateID))
		return
	}
	components.RenderJsonSucc(ctx, record)
}

// CreatePlanTemplate 新建 Plan 模板。
// @Summary      新建 Plan 模板
// @Description  在指定 caller 下创建模板，templateId 同 caller 内唯一（占位实现，内存数据）
// @Tags         React
// @Accept       json
// @Produce      json
// @Router       /react/plan_template/create [post]
func CreatePlanTemplate(ctx *gin.Context) {
	var req planTemplateSaveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		components.RenderJsonFail(ctx, components.ParamInvalidf("请求体解析失败: %s", err.Error()))
		return
	}
	callerKey := strings.TrimSpace(req.CallerKey)
	templateID := strings.TrimSpace(req.TemplateID)
	if callerKey == "" || templateID == "" {
		components.RenderJsonFail(ctx, components.ParamInvalidf("callerKey 与 templateId 不能为空"))
		return
	}
	status := req.Status
	if status != 0 && status != 1 {
		status = 1
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	record := &planTemplateRecord{
		TemplateID:  templateID,
		CallerKey:   callerKey,
		Description: templateDescription(req.Template),
		Revision:    1,
		Status:      status,
		UpdatedBy:   helpers.GetUserName(ctx),
		UpdatedAt:   now,
		Template:    req.Template,
	}

	planTemplateMu.Lock()
	key := planTemplateKey(callerKey, templateID)
	if _, exists := planTemplateStore[key]; exists {
		planTemplateMu.Unlock()
		components.RenderJsonFail(ctx, components.ParamInvalidf("模板已存在: %s", templateID))
		return
	}
	planTemplateStore[key] = record
	planTemplateMu.Unlock()

	zlog.Infof(ctx, "[PlanTemplate] create: callerKey=%s templateId=%s（占位实现，未持久化）", callerKey, templateID)
	components.RenderJsonSucc(ctx, gin.H{"templateId": templateID, "revision": record.Revision})
}

// UpdatePlanTemplate 更新 Plan 模板（内容或启停状态），revision 自增。
// @Summary      更新 Plan 模板
// @Description  按 callerKey + templateId 更新模板 JSON 与状态，revision 递增（占位实现，内存数据）
// @Tags         React
// @Accept       json
// @Produce      json
// @Router       /react/plan_template/update [post]
func UpdatePlanTemplate(ctx *gin.Context) {
	var req planTemplateSaveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		components.RenderJsonFail(ctx, components.ParamInvalidf("请求体解析失败: %s", err.Error()))
		return
	}
	callerKey := strings.TrimSpace(req.CallerKey)
	templateID := strings.TrimSpace(req.TemplateID)
	if callerKey == "" || templateID == "" {
		components.RenderJsonFail(ctx, components.ParamInvalidf("callerKey 与 templateId 不能为空"))
		return
	}

	planTemplateMu.Lock()
	record, ok := planTemplateStore[planTemplateKey(callerKey, templateID)]
	if !ok {
		planTemplateMu.Unlock()
		components.RenderJsonFail(ctx, components.ParamInvalidf("模板不存在: %s", templateID))
		return
	}
	if len(req.Template) > 0 {
		record.Template = req.Template
		record.Description = templateDescription(req.Template)
	}
	if req.Status == 0 || req.Status == 1 {
		record.Status = req.Status
	}
	record.Revision++
	record.UpdatedBy = helpers.GetUserName(ctx)
	record.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	revision := record.Revision
	planTemplateMu.Unlock()

	zlog.Infof(ctx, "[PlanTemplate] update: callerKey=%s templateId=%s revision=%d（占位实现，未持久化）", callerKey, templateID, revision)
	components.RenderJsonSucc(ctx, gin.H{"templateId": templateID, "revision": revision})
}
