package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"react-base-service/components"
	"react-base-service/components/params"
	"react-base-service/components/route"
	"react-base-service/helpers"
	model "react-base-service/models/llm"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	maxAgentTools  = 64
	maxAgentSkills = 32
)

// CreateAgent 创建子 Agent 定义；caller_key=default 是默认作用域伪 caller，不要求真实 caller 存在。
func CreateAgent(ctx *gin.Context, req *params.CreateAgentReq, createdBy string) (*model.Agent, error) {
	if err := validateAgentKey(req.AgentKey); err != nil {
		return nil, err
	}
	if !model.IsReservedCallerKey(req.CallerKey) {
		caller, err := model.GetActiveCallerByKey(ctx, req.CallerKey)
		if err != nil {
			return nil, err
		}
		if caller == nil {
			return nil, components.ErrorCallerNotFound.Sprintf(req.CallerKey)
		}
	}
	if err := validatePermissionMode(req.PermissionMode); err != nil {
		return nil, err
	}
	if err := validateReferenceList("tools", req.Tools, maxAgentTools); err != nil {
		return nil, err
	}
	if err := validateReferenceList("skills", req.Skills, maxAgentSkills); err != nil {
		return nil, err
	}

	rv := req.RouteValues
	if rv == nil {
		rv = []string{}
	}
	routeValues, _ := json.Marshal(rv)
	toolsJSON, _ := json.Marshal(normalizeReferenceList(req.Tools))
	skillsJSON, _ := json.Marshal(normalizeReferenceList(req.Skills))

	existing, err := model.GetAgentByCallerAndAgentKey(ctx, req.CallerKey, req.AgentKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, components.ErrorAgentDuplicate.Sprintf(req.CallerKey, req.AgentKey)
	}

	status, err := resolveAgentStatus(req.Status)
	if err != nil {
		return nil, err
	}

	agent := &model.Agent{
		AgentID:        "agent_" + strings.ReplaceAll(uuid.New().String(), "-", ""),
		AgentKey:       req.AgentKey,
		Name:           req.Name,
		Description:    req.Description,
		CallerKey:      req.CallerKey,
		RouteValues:    string(routeValues),
		SystemPrompt:   req.SystemPrompt,
		ModelKey:       strings.TrimSpace(req.ModelKey),
		ModelVersion:   strings.TrimSpace(req.ModelVersion),
		ToolsJSON:      string(toolsJSON),
		SkillsJSON:     string(skillsJSON),
		MaxSteps:       req.MaxSteps,
		PermissionMode: normalizePermissionMode(req.PermissionMode),
		Status:         status,
		CreatedBy:      createdBy,
		UpdatedBy:      createdBy,
	}
	if err := model.CreateAgent(ctx, agent); err != nil {
		return nil, err
	}
	return agent, nil
}

func UpdateAgent(ctx *gin.Context, req *params.UpdateAgentReq) (*model.Agent, error) {
	existing, err := model.GetAgentByAgentID(ctx, req.AgentID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, components.ErrorAgentNotFound.Sprintf(req.AgentID)
	}
	if err := validateAgentKey(req.AgentKey); err != nil {
		return nil, err
	}
	if err := validateReferenceList("tools", req.Tools, maxAgentTools); err != nil {
		return nil, err
	}
	if err := validateReferenceList("skills", req.Skills, maxAgentSkills); err != nil {
		return nil, err
	}
	if req.AgentKey != existing.AgentKey {
		duplicate, err := model.GetAgentByCallerAndAgentKey(ctx, existing.CallerKey, req.AgentKey)
		if err != nil {
			return nil, err
		}
		if duplicate != nil {
			return nil, components.ErrorAgentDuplicate.Sprintf(existing.CallerKey, req.AgentKey)
		}
	}

	status, err := resolveAgentStatus(req.Status)
	if err != nil {
		return nil, err
	}
	rv := req.RouteValues
	if rv == nil {
		rv = []string{}
	}
	routeValues, _ := json.Marshal(rv)
	toolsJSON, _ := json.Marshal(normalizeReferenceList(req.Tools))
	skillsJSON, _ := json.Marshal(normalizeReferenceList(req.Skills))

	updates := map[string]interface{}{
		"agent_key":    req.AgentKey,
		"name":         req.Name,
		"description":  req.Description,
		"route_values": string(routeValues),
		"tools_json":   string(toolsJSON),
		"skills_json":  string(skillsJSON),
		"status":       status,
		"updated_by":   helpers.GetUserName(ctx),
	}
	if req.SystemPrompt != nil {
		updates["system_prompt"] = *req.SystemPrompt
	}
	if req.ModelKey != nil {
		updates["model_key"] = strings.TrimSpace(*req.ModelKey)
	}
	if req.ModelVersion != nil {
		updates["model_version"] = strings.TrimSpace(*req.ModelVersion)
	}
	if req.MaxSteps != nil {
		updates["max_steps"] = *req.MaxSteps
	}
	if req.PermissionMode != nil {
		if err := validatePermissionMode(*req.PermissionMode); err != nil {
			return nil, err
		}
		updates["permission_mode"] = normalizePermissionMode(*req.PermissionMode)
	}

	if err := model.UpdateAgentByAgentID(ctx, req.AgentID, updates); err != nil {
		return nil, err
	}
	updated, err := model.GetAgentByAgentID(ctx, req.AgentID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, components.ErrorAgentNotFound.Sprintf(req.AgentID)
	}
	return updated, nil
}

func DeleteAgent(ctx *gin.Context, agentID string) (*model.Agent, error) {
	existing, err := model.GetAgentByAgentID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, components.ErrorAgentNotFound.Sprintf(agentID)
	}
	if err := model.UpdateAgentByAgentID(ctx, agentID, map[string]interface{}{"updated_by": helpers.GetUserName(ctx)}); err != nil {
		return nil, err
	}
	if err := model.SoftDeleteAgentByAgentID(ctx, agentID); err != nil {
		return nil, err
	}
	deleted, err := model.GetAgentByAgentIDUnscoped(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if deleted == nil {
		return nil, components.ErrorAgentNotFound.Sprintf(agentID)
	}
	return deleted, nil
}

func GetDetail(ctx *gin.Context, agentID string) (*model.Agent, error) {
	agent, err := model.GetAgentByAgentID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, components.ErrorAgentNotFound.Sprintf(agentID)
	}
	return agent, nil
}

// ListByCallerAndRoute 管理列表：callerKey 为空跨 caller 全量；否则按路由前缀列出（含全部状态）。
func ListByCallerAndRoute(ctx *gin.Context, callerKey string, routeValues []string) ([]model.Agent, error) {
	if strings.TrimSpace(callerKey) == "" {
		return model.ListAllAgents(ctx)
	}
	exact, parents := route.SplitRouteExactAndParents(routeValues)
	prefixes := append([]string{exact}, parents...)
	return model.ListAgentsByCallerAndRoutes(ctx, callerKey, prefixes)
}

// ---------- Markdown 定义导入（OH 文件型 Agent 定义对应物） ----------

// agentMarkdownFrontmatter 是 SKILL.md 风格 Agent 定义文件的 frontmatter 字段。
type agentMarkdownFrontmatter struct {
	AgentKey     string   `yaml:"agent_key"`
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	CallerKey    string   `yaml:"caller_key"`
	RouteValues  []string `yaml:"route_values"`
	SystemPrompt string   `yaml:"system_prompt"`
	ModelKey     string   `yaml:"model_key"`
	ModelVersion string   `yaml:"model_version"`
	Tools        []string `yaml:"tools"`
	Skills       []string `yaml:"skills"`
	MaxSteps     int      `yaml:"max_steps"`
}

// ImportFromMarkdown 解析「frontmatter + 正文」格式的 Agent 定义并入库：
// frontmatter 字段对应 CreateAgentReq（yaml 风格命名），正文（第二个 --- 之后）即 system_prompt；
// frontmatter 中的 caller_key / route_values 缺省时取请求体字段。
func ImportFromMarkdown(ctx *gin.Context, req *params.ImportAgentReq, createdBy string) (*model.Agent, error) {
	frontmatter, body := splitAgentMarkdown(req.Markdown)
	if strings.TrimSpace(frontmatter) == "" {
		return nil, components.ErrorAgentImportInvalid.Sprintf("缺少 frontmatter（文件须以 --- 开头）")
	}
	var meta agentMarkdownFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return nil, components.ErrorAgentImportInvalid.Sprintf(fmt.Sprintf("frontmatter 解析失败: %v", err))
	}
	if strings.TrimSpace(body) == "" {
		return nil, components.ErrorAgentImportInvalid.Sprintf("正文为空（第二个 --- 之后应为子 Agent 系统提示词）")
	}

	callerKey := firstNonEmpty(meta.CallerKey, req.CallerKey)
	if callerKey == "" {
		return nil, components.ErrorAgentImportInvalid.Sprintf("caller_key 不能为空（frontmatter 或请求体至少提供一处）")
	}
	routeValues := meta.RouteValues
	if routeValues == nil {
		routeValues = req.RouteValues
	}
	status := req.Status
	if status == nil {
		enabled := 1
		status = &enabled
	}

	createReq := &params.CreateAgentReq{
		AgentKey:     meta.AgentKey,
		Name:         meta.Name,
		Description:  meta.Description,
		CallerKey:    callerKey,
		RouteValues:  routeValues,
		SystemPrompt: strings.TrimSpace(body),
		ModelKey:     meta.ModelKey,
		ModelVersion: meta.ModelVersion,
		Tools:        meta.Tools,
		Skills:       meta.Skills,
		MaxSteps:     meta.MaxSteps,
		Status:       status,
	}
	if createReq.AgentKey == "" && createReq.Name != "" {
		createReq.AgentKey = sanitizeAgentKeyFromName(meta.Name)
	}
	return CreateAgent(ctx, createReq, createdBy)
}

// splitAgentMarkdown 拆分 frontmatter 与正文：文件以 --- 行开头，到下一个 --- 行结束。
func splitAgentMarkdown(markdown string) (frontmatter, body string) {
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	trimmed := strings.TrimLeft(normalized, "\n")
	if !strings.HasPrefix(trimmed, "---\n") {
		return "", strings.TrimSpace(trimmed)
	}
	rest := trimmed[len("---\n"):]
	if idx := strings.Index(rest, "\n---"); idx >= 0 {
		frontmatter = rest[:idx]
		after := rest[idx+len("\n---"):]
		after = strings.TrimLeft(after, "\n")
		return frontmatter, after
	}
	return "", ""
}

func sanitizeAgentKeyFromName(name string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(name) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			builder.WriteRune(r)
		}
	}
	return strings.ToLower(builder.String())
}

func validateAgentKey(agentKey string) error {
	agentKey = strings.TrimSpace(agentKey)
	if agentKey == "" {
		return components.ErrorParamInvalid.Sprintf("agentKey 不能为空")
	}
	for _, r := range agentKey {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return components.ErrorAgentKeyInvalid.Sprintf(agentKey)
	}
	return nil
}

func validatePermissionMode(mode string) error {
	switch strings.TrimSpace(mode) {
	case "", model.AgentPermissionModeInherit, model.AgentPermissionModeAuto, model.AgentPermissionModeConfirm, model.AgentPermissionModeConfirmRisky:
		return nil
	default:
		return components.ErrorParamInvalid.Sprintf("permissionMode 仅支持 inherit/auto/confirm/confirm_risky")
	}
}

func normalizePermissionMode(mode string) string {
	if mode = strings.TrimSpace(mode); mode == "" {
		return model.AgentPermissionModeInherit
	}
	return mode
}

// validateReferenceList 校验 tools/skills 白名单：非空、去重、无空白项、数量受限。
func validateReferenceList(field string, values []string, limit int) error {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return components.ErrorParamInvalid.Sprintf("%s 白名单不允许空项", field)
		}
		if seen[value] {
			return components.ErrorParamInvalid.Sprintf("%s 白名单存在重复项: %s", field, value)
		}
		seen[value] = true
	}
	if len(values) > limit {
		return components.ErrorParamInvalid.Sprintf("%s 白名单最多 %d 项", field, limit)
	}
	return nil
}

func normalizeReferenceList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			normalized = append(normalized, value)
		}
	}
	return normalized
}

func resolveAgentStatus(status *int) (int, error) {
	if status == nil {
		return 0, components.ErrorParamInvalid.Sprintf("status不能为空")
	}
	if *status != 0 && *status != 1 {
		return 0, components.ErrorParamInvalid.Sprintf("status 仅支持 0 或 1")
	}
	return *status, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseAgentRouteValues(routeValuesRaw string) []string {
	var routeValues []string
	_ = json.Unmarshal([]byte(routeValuesRaw), &routeValues)
	if routeValues == nil {
		routeValues = []string{}
	}
	return routeValues
}

func parseAgentReferenceJSON(raw string) []string {
	var values []string
	_ = json.Unmarshal([]byte(raw), &values)
	if values == nil {
		values = []string{}
	}
	return values
}

func ToAgentResp(a *model.Agent) params.AgentResp {
	return params.AgentResp{
		AgentID:        a.AgentID,
		AgentKey:       a.AgentKey,
		Name:           a.Name,
		Description:    a.Description,
		CallerKey:      a.CallerKey,
		RouteValues:    parseAgentRouteValues(a.RouteValues),
		SystemPrompt:   a.SystemPrompt,
		ModelKey:       a.ModelKey,
		ModelVersion:   a.ModelVersion,
		Tools:          parseAgentReferenceJSON(a.ToolsJSON),
		Skills:         parseAgentReferenceJSON(a.SkillsJSON),
		MaxSteps:       a.MaxSteps,
		PermissionMode: a.PermissionMode,
		Status:         a.Status,
		CreatedBy:      a.CreatedBy,
		UpdatedBy:      a.UpdatedBy,
		CreatedAt:      formatAgentTime(a.CreatedAt),
		UpdatedAt:      formatAgentTime(a.UpdatedAt),
	}
}

func formatAgentTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
