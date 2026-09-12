package react

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"react-base-service/components"
	"react-base-service/golib/zlog"
	"react-base-service/helpers"
	model "react-base-service/models/llm"
	"react-base-service/service/mcpclient"

	"github.com/gin-gonic/gin"
)

// MCP 连接管理接口 —— 查看、更新、管理 MCP 连接。
//
// 连接登记在 tblLlmMcpServer（每个 caller 一份视图，name 全局唯一），
// 拉起/停走 service/mcpclient 全局 Manager，工具清单同步进 tblLlmTool（tool_type=mcp），
// ReAct 运行时经既有 get_tool/execute_tool 两段式加载使用，无需改动引擎。
//
// 安全约束：
//   - HTTP 端点经 SSRF 校验（默认拒绝环回/私网；本机演示可在 mcp.allow_private_endpoint 开启）
//   - stdio 仅支持代码内适配器白名单（kind=repo），不接收任意 command/args
//   - 自定义 headers 附加到每个请求，但不允许覆盖协议自身管理的头（Host/Content-Type/Mcp-Session-Id 等）

const (
	mcpCheckConnected    = "connected"
	mcpCheckDisconnected = "disconnected"
	mcpCheckUnknown      = "unknown"
)

type mcpListRequest struct {
	CallerKey string `json:"callerKey"`
}

type mcpScopeRequest struct {
	CallerKey string `json:"callerKey"`
	ServerID  string `json:"serverId"`
}

type mcpCreateRequest struct {
	CallerKey   string            `json:"callerKey"`
	RawConfig   string            `json:"rawConfig"`
	Name        string            `json:"name"`
	Kind        string            `json:"kind"`
	Endpoint    string            `json:"endpoint"`
	Headers     map[string]string `json:"headers"`
	TimeoutMs   int               `json:"timeoutMs"`
	Description string            `json:"description"`
	// BoundCallers 是连接额外绑定的 caller（工具同步到这些 caller 名下）；属主 caller 恒定生效。
	BoundCallers []string `json:"boundCallers"`
	// Status 用指针区分「未传（默认启用）」与「显式停用 0」。
	Status *int `json:"status"`
}

type mcpUpdateRequest struct {
	CallerKey   string            `json:"callerKey"`
	ServerID    string            `json:"serverId"`
	Endpoint    string            `json:"endpoint"`
	Headers     map[string]string `json:"headers"`
	TimeoutMs   *int              `json:"timeoutMs"`
	Description string            `json:"description"`
	// BoundCallers 全量替换绑定 caller；nil 表示不改动。
	BoundCallers []string `json:"boundCallers"`
	Status       *int     `json:"status"`
}

// mcpServerView 是列表/详情返回的连接视图：注册表记录 + 运行时状态 + 工具清单摘要。
type mcpServerView struct {
	ServerID         string `json:"serverId"`
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	Endpoint         string `json:"endpoint"`
	Description      string `json:"description"`
	TimeoutMs        int    `json:"timeoutMs"`
	Status           int    `json:"status"`
	HasHeaders       bool   `json:"hasHeaders"`
	Running          bool   `json:"running"`
	LastCheckStatus  string `json:"lastCheckStatus"`
	LastCheckMessage string `json:"lastCheckMessage"`
	LastCheckAt      string `json:"lastCheckAt"`
	ToolCount        int    `json:"toolCount"`
	// BoundCallers 是该连接工具同步到的全部 caller（属主在首位；其余为绑定 caller）。
	BoundCallers []string `json:"boundCallers"`
	CreatedBy    string   `json:"createdBy"`
	UpdatedBy    string   `json:"updatedBy"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
	// Tools 仅 detail 返回（含 name/description/status）。
	Tools []mcpToolView `json:"tools,omitempty"`
}

type mcpToolView struct {
	ToolID      string `json:"toolId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      int    `json:"status"`
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// mcpConnectionCallers 返回连接工具同步到的全部 caller：属主在首位，其后为绑定 caller（去重、校验存活）。
func mcpConnectionCallers(ctx *gin.Context, server *model.McpServer) []string {
	callers := []string{server.CallerKey}
	bound, err := model.ListMcpServerCallers(ctx, server.ServerID)
	if err != nil {
		zlog.Errorf(ctx, "[MCP] 查询连接绑定 caller 失败: serverId=%s err=%v", server.ServerID, err)
		return callers
	}
	seen := map[string]bool{server.CallerKey: true}
	for _, key := range bound {
		if seen[key] {
			continue
		}
		seen[key] = true
		callers = append(callers, key)
	}
	return callers
}

// normalizeBoundCallers 校验并归一化绑定 caller 列表：非空、去重、剔除属主（属主恒定生效不占绑定行）、必须是存活 caller 或 default 作用域。
func normalizeBoundCallers(ctx *gin.Context, ownerCaller string, raw []string) ([]string, error) {
	normalized := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, key := range raw {
		key = strings.TrimSpace(key)
		if key == "" || key == ownerCaller || seen[key] {
			continue
		}
		if !model.IsReservedCallerKey(key) {
			caller, err := model.GetActiveCallerByKey(ctx, key)
			if err != nil {
				return nil, err
			}
			if caller == nil {
				return nil, components.ParamInvalidf("绑定 caller 不存在或已停用: %s", key)
			}
		}
		seen[key] = true
		normalized = append(normalized, key)
	}
	return normalized, nil
}

func mcpServerToView(server model.McpServer, withTools bool) mcpServerView {
	tools, _ := model.ListMCPServerTools(&gin.Context{}, server.Name)
	view := mcpServerView{
		ServerID:         server.ServerID,
		Name:             server.Name,
		Kind:             server.Kind,
		Endpoint:         server.Endpoint,
		Description:      server.Description,
		TimeoutMs:        server.TimeoutMs,
		Status:           server.Status,
		HasHeaders:       strings.TrimSpace(server.Headers) != "" && strings.TrimSpace(server.Headers) != "{}",
		Running:          mcpclient.GetServer(server.Name) != nil,
		LastCheckStatus:  server.LastCheckStatus,
		LastCheckMessage: server.LastCheckMessage,
		LastCheckAt:      formatTimePtr(server.LastCheckAt),
		ToolCount:        len(tools),
		BoundCallers:     mcpConnectionCallers(&gin.Context{}, &server),
		CreatedBy:        server.CreatedBy,
		UpdatedBy:        server.UpdatedBy,
		CreatedAt:        server.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:        server.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	if withTools {
		view.Tools = make([]mcpToolView, 0, len(tools))
		for _, tool := range tools {
			view.Tools = append(view.Tools, mcpToolView{
				ToolID:      tool.ToolID,
				Name:        tool.Name,
				Description: tool.Description,
				Status:      tool.Status,
			})
		}
	}
	return view
}

// ListMcpServers 列出当前 caller 下的全部 MCP 连接（含工具数与运行状态）。
// @Summary      MCP 连接列表
// @Description  返回指定 caller 下的 MCP 连接（注册表记录 + 运行状态 + 工具数）
// @Tags         React
// @Accept       json
// @Produce      json
// @Router       /react/mcp/list [post]
func ListMcpServers(ctx *gin.Context) {
	var req mcpListRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		components.RenderJsonFail(ctx, components.ParamInvalidf("请求体解析失败: %s", err.Error()))
		return
	}
	callerKey := strings.TrimSpace(req.CallerKey)
	if callerKey == "" {
		components.RenderJsonFail(ctx, components.ParamInvalidf("callerKey 不能为空"))
		return
	}

	servers, err := model.ListMcpServersByCaller(ctx, callerKey)
	if err != nil {
		zlog.Errorf(ctx, "[MCP] 查询连接列表失败: %v", err)
		components.RenderJsonFail(ctx, err)
		return
	}
	views := make([]mcpServerView, 0, len(servers))
	for _, server := range servers {
		views = append(views, mcpServerToView(server, false))
	}
	components.RenderJsonSucc(ctx, views)
}

// GetMcpServerDetail 返回单个 MCP 连接详情（含同步进注册表的工具清单）。
// @Summary      MCP 连接详情
// @Description  按 callerKey + serverId 返回连接完整配置与工具清单
// @Tags         React
// @Accept       json
// @Produce      json
// @Router       /react/mcp/detail [post]
func GetMcpServerDetail(ctx *gin.Context) {
	var req mcpScopeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		components.RenderJsonFail(ctx, components.ParamInvalidf("请求体解析失败: %s", err.Error()))
		return
	}
	server, ok := loadMcpServer(ctx, req.CallerKey, req.ServerID)
	if !ok {
		return
	}
	view := mcpServerToView(*server, true)
	var headers map[string]string
	if strings.TrimSpace(server.Headers) != "" {
		_ = json.Unmarshal([]byte(server.Headers), &headers)
	}
	components.RenderJsonSucc(ctx, gin.H{"server": view, "headers": headers})
}

// CreateMcpServer 新增 MCP 连接。支持两种方式（rawConfig 优先）：
//  1. rawConfig：粘贴标准 mcpServers JSON（截图所示「手动配置」），可一次登记多个；
//     仅支持 url 形式（{url, headers}），command/args 形式会被拒绝（stdio 只开放白名单适配器）
//  2. 结构化字段：name + kind(http/repo) + endpoint/headers/timeoutMs
//
// 创建成功后立即尝试拉起并同步工具（失败不影响登记，标记 last_check_*）。
// @Summary      新增 MCP 连接
// @Description  登记 MCP 服务器（rawConfig 粘贴或结构化字段），并尝试连接同步工具
// @Tags         React
// @Accept       json
// @Produce      json
// @Router       /react/mcp/create [post]
func CreateMcpServer(ctx *gin.Context) {
	var req mcpCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		components.RenderJsonFail(ctx, components.ParamInvalidf("请求体解析失败: %s", err.Error()))
		return
	}
	callerKey := strings.TrimSpace(req.CallerKey)
	if callerKey == "" {
		components.RenderJsonFail(ctx, components.ParamInvalidf("callerKey 不能为空"))
		return
	}

	drafts, err := parseMcpCreateDrafts(req)
	if err != nil {
		components.RenderJsonFail(ctx, components.ParamInvalidf("%s", err.Error()))
		return
	}
	if err := bindMcpCreateCallers(ctx, callerKey, drafts, req); err != nil {
		components.RenderJsonFail(ctx, err)
		return
	}

	created := make([]mcpServerView, 0, len(drafts))
	for _, draft := range drafts {
		server, err := createMcpServerRecord(ctx, callerKey, draft)
		if err != nil {
			components.RenderJsonFail(ctx, err)
			return
		}
		created = append(created, *server)
	}
	components.RenderJsonSucc(ctx, created)
}

// mcpServerDraft 是 Create/Update 共用的落库前中间结构。
type mcpServerDraft struct {
	Name        string
	Kind        string
	Endpoint    string
	HeadersJSON string
	EnvJSON     string
	TimeoutMs   int
	Description string
	Status      int
	// BoundCallers 创建时一并登记的绑定 caller（已归一化）。
	BoundCallers []string
}

func parseMcpCreateDrafts(req mcpCreateRequest) ([]mcpServerDraft, error) {
	if strings.TrimSpace(req.RawConfig) != "" {
		return parseMcpServersJSON(req.RawConfig)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name 不能为空（或改用 rawConfig 粘贴 mcpServers JSON）")
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if kind == "" {
		kind = "http"
	}
	draft := mcpServerDraft{
		Name:        name,
		Kind:        kind,
		Description: strings.TrimSpace(req.Description),
	}
	switch kind {
	case "http":
		draft.Endpoint = strings.TrimSpace(req.Endpoint)
		if draft.Endpoint == "" {
			return nil, fmt.Errorf("kind=http 需要 endpoint")
		}
		headersJSON, err := marshalMcpHeaders(req.Headers)
		if err != nil {
			return nil, err
		}
		draft.HeadersJSON = headersJSON
	case "repo":
		draft.EnvJSON = "{}"
	default:
		return nil, fmt.Errorf("不支持的 kind %q（可选: http / repo）", req.Kind)
	}
	draft.TimeoutMs = normalizeMcpTimeout(req.TimeoutMs)
	draft.Status = 1
	if req.Status != nil && *req.Status == 0 {
		draft.Status = 0
	}
	return []mcpServerDraft{draft}, nil
}

// bindMcpCreateCallers 校验 create 请求的绑定 caller，写入每个 draft（rawConfig 多条共享同一份绑定）。
func bindMcpCreateCallers(ctx *gin.Context, ownerCaller string, drafts []mcpServerDraft, req mcpCreateRequest) error {
	bound, err := normalizeBoundCallers(ctx, ownerCaller, req.BoundCallers)
	if err != nil {
		return err
	}
	for i := range drafts {
		drafts[i].BoundCallers = bound
	}
	return nil
}

// parseMcpServersJSON 解析「手动配置」粘贴的标准 mcpServers JSON：
//
//	{ "mcpServers": { "<name>": { "url": "...", "headers": {...} } } }
//
// 也接受去掉外层 mcpServers 的单个映射。command/args 形式直接报错说明原因。
func parseMcpServersJSON(raw string) ([]mcpServerDraft, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("rawConfig 不是合法 JSON: %w", err)
	}
	serversRaw, ok := payload["mcpServers"]
	if !ok {
		serversRaw, ok = payload["servers"]
	}
	if ok {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(serversRaw, &inner); err != nil {
			return nil, fmt.Errorf("mcpServers 必须是 {名称: 配置} 映射: %w", err)
		}
		payload = inner
	}

	drafts := make([]mcpServerDraft, 0, len(payload))
	for name, item := range payload {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			return nil, fmt.Errorf("mcpServers 存在空名称条目")
		}
		var entry struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
			Command string            `json:"command"`
		}
		if err := json.Unmarshal(item, &entry); err != nil {
			return nil, fmt.Errorf("服务器 %s 配置解析失败: %w", trimmedName, err)
		}
		if strings.TrimSpace(entry.Command) != "" {
			return nil, fmt.Errorf("服务器 %s 是 command/args 形式配置：基座仅开放 url 形式的 HTTP MCP 与内置 stdio 适配器（kind=repo），不支持任意命令拉起", trimmedName)
		}
		if strings.TrimSpace(entry.URL) == "" {
			return nil, fmt.Errorf("服务器 %s 缺少 url 字段", trimmedName)
		}
		headersJSON, err := marshalMcpHeaders(entry.Headers)
		if err != nil {
			return nil, fmt.Errorf("服务器 %s: %w", trimmedName, err)
		}
		drafts = append(drafts, mcpServerDraft{
			Name:        trimmedName,
			Kind:        "http",
			Endpoint:    strings.TrimSpace(entry.URL),
			HeadersJSON: headersJSON,
			TimeoutMs:   0,
			Description: "",
			Status:      1,
		})
	}
	if len(drafts) == 0 {
		return nil, fmt.Errorf("rawConfig 中没有可登记的 MCP 服务器")
	}
	return drafts, nil
}

const maxMcpHeadersJSONBytes = 4 << 10

func marshalMcpHeaders(headers map[string]string) (string, error) {
	if len(headers) == 0 {
		return "", nil
	}
	data, err := json.Marshal(headers)
	if err != nil {
		return "", fmt.Errorf("headers 序列化失败: %w", err)
	}
	if len(data) > maxMcpHeadersJSONBytes {
		return "", fmt.Errorf("headers 超过 %d 字节上限", maxMcpHeadersJSONBytes)
	}
	return string(data), nil
}

func normalizeMcpTimeout(timeoutMs int) int {
	if timeoutMs <= 0 {
		return 30000
	}
	if timeoutMs > 300000 {
		return 300000
	}
	return timeoutMs
}

func newMcpServerID(name string) string {
	return "mcpserver_" + name + "_" + fmt.Sprintf("%x", time.Now().UnixNano())
}

func createMcpServerRecord(ctx *gin.Context, callerKey string, draft mcpServerDraft) (*mcpServerView, error) {
	if existing, err := model.GetMcpServerByName(ctx, draft.Name); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, components.ParamInvalidf("服务器名已存在: %s（MCP 服务器名全局唯一）", draft.Name)
	}

	userName := helpers.GetUserName(ctx)
	server := &model.McpServer{
		ServerID:    newMcpServerID(draft.Name),
		Name:        draft.Name,
		Kind:        draft.Kind,
		Endpoint:    draft.Endpoint,
		Headers:     draft.HeadersJSON,
		Env:         draft.EnvJSON,
		TimeoutMs:   draft.TimeoutMs,
		Description: draft.Description,
		Status:      draft.Status,
		CallerKey:   callerKey,
		CreatedBy:   userName,
		UpdatedBy:   userName,
	}
	// 软删除的同名行仍占用 uk_name 唯一键：复活该行而不是重复插入。
	if softDeleted, err := model.GetMcpServerByNameUnscoped(ctx, draft.Name); err != nil {
		return nil, err
	} else if softDeleted != nil {
		if err := model.ReviveMcpServerByName(ctx, draft.Name, map[string]interface{}{
			"server_id":          server.ServerID,
			"kind":               server.Kind,
			"endpoint":           server.Endpoint,
			"headers":            server.Headers,
			"env":                server.Env,
			"timeout_ms":         server.TimeoutMs,
			"description":        server.Description,
			"status":             server.Status,
			"caller_key":         server.CallerKey,
			"created_by":         server.CreatedBy,
			"updated_by":         server.UpdatedBy,
			"last_check_status":  mcpCheckUnknown,
			"last_check_message": "",
			"deleted_at":         0,
		}); err != nil {
			return nil, err
		}
	} else if err := model.CreateMcpServer(ctx, server); err != nil {
		return nil, err
	}

	// 登记绑定 caller（属主之外的工具同步目标）。
	if len(draft.BoundCallers) > 0 {
		if err := model.ReplaceMcpServerCallers(ctx, server.ServerID, draft.BoundCallers, userName); err != nil {
			return nil, err
		}
	}

	if draft.Status == 1 {
		applyMcpConnect(ctx, server)
	} else {
		_ = mcpclient.RemoveServer(draft.Name)
	}
	zlog.Infof(ctx, "[MCP] 新增连接: callerKey=%s name=%s kind=%s endpoint=%s", callerKey, draft.Name, draft.Kind, draft.Endpoint)

	refreshed, err := model.GetMcpServerByServerID(ctx, server.ServerID)
	if err != nil || refreshed == nil {
		refreshed = server
	}
	view := mcpServerToView(*refreshed, true)
	return &view, nil
}

// applyMcpConnect 拉起连接并把工具同步到属主与全部绑定 caller 名下，结果写回 last_check_* 字段。
func applyMcpConnect(ctx *gin.Context, server *model.McpServer) {
	cfg, err := mcpclient.McpServerConfig(*server)
	if err != nil {
		recordMcpCheckResult(ctx, server.ServerID, mcpCheckDisconnected, err.Error())
		return
	}
	if _, err := mcpclient.EnsureServer(*cfg); err != nil {
		recordMcpCheckResult(ctx, server.ServerID, mcpCheckDisconnected, err.Error())
		return
	}
	// 属主用历史 toolId 格式，绑定 caller 用带后缀的副本；绑定 caller 不存在/停用则静默跳过。
	for i, callerKey := range mcpConnectionCallers(ctx, server) {
		if i > 0 {
			if caller, err := model.GetActiveCallerByKey(ctx, callerKey); err != nil || caller == nil {
				if !model.IsReservedCallerKey(callerKey) {
					continue
				}
			}
		}
		if _, err := mcpclient.SyncServerRegistryScoped(callerKey, server.Name, i == 0); err != nil {
			recordMcpCheckResult(ctx, server.ServerID, mcpCheckDisconnected, err.Error())
			return
		}
	}
	recordMcpCheckResult(ctx, server.ServerID, mcpCheckConnected, "连接成功，工具清单已同步")
}

func recordMcpCheckResult(ctx *gin.Context, serverID, status, message string) {
	if len(message) > 500 {
		message = message[:500]
	}
	updates := map[string]interface{}{
		"last_check_status":  status,
		"last_check_message": message,
		"last_check_at":      time.Now(),
	}
	if err := model.UpdateMcpServerByServerID(ctx, serverID, updates); err != nil {
		zlog.Errorf(ctx, "[MCP] 写回连接检测结果失败: serverId=%s err=%v", serverID, err)
	}
}

func loadMcpServer(ctx *gin.Context, callerKey, serverID string) (*model.McpServer, bool) {
	callerKey = strings.TrimSpace(callerKey)
	serverID = strings.TrimSpace(serverID)
	if callerKey == "" || serverID == "" {
		components.RenderJsonFail(ctx, components.ParamInvalidf("callerKey 与 serverId 不能为空"))
		return nil, false
	}
	server, err := model.GetMcpServerByServerID(ctx, serverID)
	if err != nil {
		zlog.Errorf(ctx, "[MCP] 查询连接失败: %v", err)
		components.RenderJsonFail(ctx, err)
		return nil, false
	}
	if server == nil || server.CallerKey != callerKey {
		components.RenderJsonFail(ctx, components.ParamInvalidf("连接不存在: %s", serverID))
		return nil, false
	}
	return server, true
}

// UpdateMcpServer 更新 MCP 连接（端点/请求头/超时/描述/启停）。
// 更新后重启客户端并重新同步工具；停用时会停掉客户端并软删除其注册工具。
// @Summary      更新 MCP 连接
// @Description  按 callerKey + serverId 更新连接配置或启停状态，并重连同步
// @Tags         React
// @Accept       json
// @Produce      json
// @Router       /react/mcp/update [post]
func UpdateMcpServer(ctx *gin.Context) {
	var req mcpUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		components.RenderJsonFail(ctx, components.ParamInvalidf("请求体解析失败: %s", err.Error()))
		return
	}
	server, ok := loadMcpServer(ctx, req.CallerKey, req.ServerID)
	if !ok {
		return
	}

	updates := map[string]interface{}{"updated_by": helpers.GetUserName(ctx)}
	if req.Endpoint != "" {
		if server.Kind != "http" {
			components.RenderJsonFail(ctx, components.ParamInvalidf("kind=%s 不支持修改 endpoint", server.Kind))
			return
		}
		updates["endpoint"] = strings.TrimSpace(req.Endpoint)
	}
	if req.Headers != nil {
		headersJSON, err := marshalMcpHeaders(req.Headers)
		if err != nil {
			components.RenderJsonFail(ctx, components.ParamInvalidf("%s", err.Error()))
			return
		}
		updates["headers"] = headersJSON
	}
	if req.TimeoutMs != nil {
		updates["timeout_ms"] = normalizeMcpTimeout(*req.TimeoutMs)
	}
	if req.Description != "" {
		updates["description"] = strings.TrimSpace(req.Description)
	}
	if req.Status != nil {
		if *req.Status != 0 && *req.Status != 1 {
			components.RenderJsonFail(ctx, components.ParamInvalidf("status 只允许 0/1"))
			return
		}
		updates["status"] = *req.Status
	}
	// 绑定 caller 全量替换：先算差集，落库后解绑方清理工具副本、连接启用时新增方立即同步。
	var unboundCallers []string
	newBoundCallers := []string{}
	bindingsChanged := false
	if req.BoundCallers != nil {
		normalized, err := normalizeBoundCallers(ctx, server.CallerKey, req.BoundCallers)
		if err != nil {
			components.RenderJsonFail(ctx, err)
			return
		}
		current, err := model.ListMcpServerCallers(ctx, server.ServerID)
		if err != nil {
			components.RenderJsonFail(ctx, err)
			return
		}
		currentSet := make(map[string]bool, len(current))
		for _, key := range current {
			currentSet[key] = true
		}
		newSet := make(map[string]bool, len(normalized))
		for _, key := range normalized {
			newSet[key] = true
			if !currentSet[key] {
				bindingsChanged = true
			}
		}
		for _, key := range current {
			if !newSet[key] {
				unboundCallers = append(unboundCallers, key)
				bindingsChanged = true
			}
		}
		newBoundCallers = normalized
	}

	if err := model.UpdateMcpServerByServerID(ctx, server.ServerID, updates); err != nil {
		components.RenderJsonFail(ctx, err)
		return
	}
	if req.BoundCallers != nil {
		if err := model.ReplaceMcpServerCallers(ctx, server.ServerID, newBoundCallers, helpers.GetUserName(ctx)); err != nil {
			components.RenderJsonFail(ctx, err)
			return
		}
	}
	refreshed, err := model.GetMcpServerByServerID(ctx, server.ServerID)
	if err != nil || refreshed == nil {
		components.RenderJsonFail(ctx, err)
		return
	}

	// 解绑的 caller 名下工具副本立即清理（不论连接启停）。
	for _, callerKey := range unboundCallers {
		if _, err := mcpclient.RemoveRegistryToolsForCaller(ctx, refreshed.Name, callerKey); err != nil {
			zlog.Errorf(ctx, "[MCP] 解绑 caller %s 清理工具失败: %v", callerKey, err)
		}
	}

	if refreshed.Status == 1 {
		applyMcpConnect(ctx, refreshed)
	} else {
		_ = mcpclient.RemoveServer(refreshed.Name)
		if _, err := mcpclient.RemoveRegistryTools(ctx, refreshed.Name); err != nil {
			zlog.Errorf(ctx, "[MCP] 停用连接 %s 清理注册工具失败: %v", refreshed.Name, err)
		}
		recordMcpCheckResult(ctx, refreshed.ServerID, mcpCheckUnknown, "连接已停用")
	}
	zlog.Infof(ctx, "[MCP] 更新连接: callerKey=%s name=%s status=%d bindingsChanged=%t", refreshed.CallerKey, refreshed.Name, refreshed.Status, bindingsChanged)
	components.RenderJsonSucc(ctx, gin.H{"serverId": refreshed.ServerID})
}

// DeleteMcpServer 删除 MCP 连接：停掉客户端、软删除注册表工具、软删除连接记录。
// @Summary      删除 MCP 连接
// @Description  按 callerKey + serverId 删除连接及其同步的注册工具
// @Tags         React
// @Accept       json
// @Produce      json
// @Router       /react/mcp/delete [post]
func DeleteMcpServer(ctx *gin.Context) {
	var req mcpScopeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		components.RenderJsonFail(ctx, components.ParamInvalidf("请求体解析失败: %s", err.Error()))
		return
	}
	server, ok := loadMcpServer(ctx, req.CallerKey, req.ServerID)
	if !ok {
		return
	}

	_ = mcpclient.RemoveServer(server.Name)
	removedTools, err := mcpclient.RemoveRegistryTools(ctx, server.Name)
	if err != nil {
		zlog.Errorf(ctx, "[MCP] 删除连接 %s 清理注册工具失败: %v", server.Name, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	if err := model.SoftDeleteMcpServerByServerID(ctx, server.ServerID); err != nil {
		components.RenderJsonFail(ctx, err)
		return
	}
	zlog.Infof(ctx, "[MCP] 删除连接: callerKey=%s name=%s 清理工具=%d", server.CallerKey, server.Name, removedTools)
	components.RenderJsonSucc(ctx, gin.H{"serverId": server.ServerID, "removedTools": removedTools})
}

// ConnectMcpServer 连接测试：拉起客户端、同步工具清单并写回检测结果。
// @Summary      MCP 连接测试
// @Description  按 callerKey + serverId 立即连接该 MCP 服务器并同步工具清单
// @Tags         React
// @Accept       json
// @Produce      json
// @Router       /react/mcp/connect [post]
func ConnectMcpServer(ctx *gin.Context) {
	var req mcpScopeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		components.RenderJsonFail(ctx, components.ParamInvalidf("请求体解析失败: %s", err.Error()))
		return
	}
	server, ok := loadMcpServer(ctx, req.CallerKey, req.ServerID)
	if !ok {
		return
	}
	if server.Status != 1 {
		components.RenderJsonFail(ctx, components.ParamInvalidf("连接已停用，请先启用再测试"))
		return
	}

	applyMcpConnect(ctx, server)
	refreshed, err := model.GetMcpServerByServerID(ctx, server.ServerID)
	if err != nil || refreshed == nil {
		components.RenderJsonFail(ctx, err)
		return
	}
	if refreshed.LastCheckStatus != mcpCheckConnected {
		components.RenderJsonFail(ctx, components.ParamInvalidf("连接失败: %s", refreshed.LastCheckMessage))
		return
	}
	view := mcpServerToView(*refreshed, true)
	components.RenderJsonSucc(ctx, gin.H{"serverId": refreshed.ServerID, "toolCount": view.ToolCount, "message": refreshed.LastCheckMessage})
}
