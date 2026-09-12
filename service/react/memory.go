package react

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	llm "react-base-service/api/llm"
	"react-base-service/conf"
	model "react-base-service/models/llm"
	memoryService "react-base-service/service/memory"

	"github.com/gin-gonic/gin"
)

const (
	memoryReadMaxItems    = 10
	memoryListDefaultSize = 20
	memoryListMaxSize     = 50
)

// memoryScopeResolved 描述一次 run 可见的记忆空间与写入目标。
type memoryScopeResolved struct {
	owners      []model.MemoryOwner // 可见空间（caller 在前，caller_user 在后）
	writeOwner  model.MemoryOwner   // memory_write 的落库目标
	allowedKeys map[string]struct{} // 可见空间集合（OwnerScopeKey），read/list/write 权限校验用
}

// resolveMemoryScope 解析当前 run 的记忆作用域：caller 级做公共底座，user 级（启用时）为写入目标。
func resolveMemoryScope(callerKey, userName string, allowUserScope bool) memoryScopeResolved {
	scope := memoryScopeResolved{
		owners:      []model.MemoryOwner{model.BuildCallerMemoryOwner(callerKey)},
		allowedKeys: make(map[string]struct{}, 2),
	}
	scope.writeOwner = scope.owners[0]
	scope.allowedKeys[memoryOwnerKey(model.BuildCallerMemoryOwner(callerKey))] = struct{}{}
	if allowUserScope {
		userOwner := model.BuildCallerUserMemoryOwner(callerKey, userName)
		scope.owners = append(scope.owners, userOwner)
		scope.writeOwner = userOwner
		scope.allowedKeys[memoryOwnerKey(userOwner)] = struct{}{}
	}
	return scope
}

func memoryOwnerKey(owner model.MemoryOwner) string {
	return memoryService.OwnerScopeKey(owner.OwnerType, owner.OwnerKey)
}

// mergeMemoryItems 合并多空间条目：同 itemKey 时 caller_user 恒覆盖 caller（按 owner 优先级而非更新时间），
// 结果按更新时间倒序输出。
func mergeMemoryItems(items []model.MemoryItem) []model.MemoryItem {
	slices.SortStableFunc(items, func(a, b model.MemoryItem) int {
		return cmp.Compare(memoryOwnerPriority(a.OwnerType), memoryOwnerPriority(b.OwnerType))
	})
	merged := make([]model.MemoryItem, 0, len(items))
	indexByKey := make(map[string]int, len(items))
	for _, item := range items {
		key := item.ItemKey
		if pos, ok := indexByKey[key]; ok {
			merged[pos] = item
			continue
		}
		indexByKey[key] = len(merged)
		merged = append(merged, item)
	}
	slices.SortStableFunc(merged, func(a, b model.MemoryItem) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return merged
}

// memoryOwnerPriority owner 覆盖优先级：数值大的覆盖小的（caller_user > caller）。
func memoryOwnerPriority(ownerType string) int {
	if ownerType == model.MemoryOwnerTypeCallerUser {
		return 1
	}
	return 0
}

// splitMemoryLayers 按层级拆分条目。
func splitMemoryLayers(items []model.MemoryItem) (resident, detached []model.MemoryItem) {
	for _, item := range items {
		if item.Layer == model.MemoryLayerResident {
			resident = append(resident, item)
		} else {
			detached = append(detached, item)
		}
	}
	return resident, detached
}

// renderMemoryContext 渲染注入 system 前缀的 <memory> 块：常驻层全文 + 按需层目录索引。
// 无条目时返回空串（不注入，token 零增量）。
func renderMemoryContext(items []model.MemoryItem, cfg conf.ReactMemoryConfig) string {
	resident, detached := splitMemoryLayers(items)
	if len(resident) == 0 && len(detached) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<memory>\n")
	sb.WriteString("## 长期记忆（自动维护）\n\n")

	if len(resident) > 0 {
		sb.WriteString("### 常驻\n")
		used := 0
		truncated := 0
		for _, item := range resident {
			line := fmt.Sprintf("- [%s] %s\n", item.Title, strings.TrimSpace(item.Content))
			if used+len([]rune(line)) > cfg.ResidentBudgetChars {
				truncated++
				continue
			}
			sb.WriteString(line)
			used += len([]rune(line))
		}
		if truncated > 0 {
			sb.WriteString(fmt.Sprintf("（另有 %d 条常驻记忆超出字符预算未注入，可用 memory_list 查看）\n", truncated))
		}
		sb.WriteString("\n")
	}

	if len(detached) > 0 {
		sb.WriteString("### 记忆目录（需要时用 memory_read 按 itemId 读取全文）\n")
		listed := detached
		overflow := 0
		if len(listed) > cfg.IndexMaxItems {
			overflow = len(listed) - cfg.IndexMaxItems
			listed = listed[:cfg.IndexMaxItems]
		}
		for _, item := range listed {
			description := strings.TrimSpace(item.Description)
			if description == "" {
				description = strings.TrimSpace(item.Content)
			}
			sb.WriteString(fmt.Sprintf("- #%d [%s] %s\n", item.ID, item.Title, description))
		}
		if overflow > 0 {
			sb.WriteString(fmt.Sprintf("（另有 %d 条记忆未列出，可用 memory_list 检索）\n", overflow))
		}
	}

	sb.WriteString("\n记忆使用纪律：以上内容自动维护、可能过时；与用户当前表述冲突时以用户为准，并用 memory_write 修正。\n")
	sb.WriteString("</memory>")
	return sb.String()
}

// buildMemoryContextForRun 在 run 初始化阶段装配记忆注入块：解析作用域 → 查询 → 合并 → 渲染。
func buildMemoryContextForRun(ctx *gin.Context, callerKey, userName string) (string, error) {
	cfg := conf.GetReactRuntimeConfig().Memory
	scope := resolveMemoryScope(callerKey, userName, cfg.MemoryAllowUserScope())
	items, err := model.FindActiveMemoryItemsByOwners(ctx, scope.owners)
	if err != nil {
		return "", err
	}
	return renderMemoryContext(mergeMemoryItems(items), cfg), nil
}

// memoryToolDefinitions 声明三个记忆工具；仅在 memory.enabled 时注册。
func memoryToolDefinitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		objectTool(metaToolMemoryList,
			"列出当前作用域的长期记忆索引（默认按需层全部）。返回条目的 itemId/title/description/tags/layer，不含正文；需要正文时用 memory_read。",
			map[string]interface{}{
				"layer": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"resident", "detached", "all"},
					"description": "层级过滤，默认 detached。",
				},
				"tag":     stringSchema("按标签精确过滤，可选。"),
				"keyword": stringSchema("标题/描述关键词过滤，可选。"),
				"limit":   numberSchema("返回条数上限，默认 20，最大 50。"),
			}),
		objectTool(metaToolMemoryRead,
			"按 itemId 读取记忆全文（正文、版本、来源与最近修订原因）。一次最多 10 条。",
			map[string]interface{}{
				"itemIds": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "number"},
					"description": "memory_list 或记忆目录返回的 itemId 数组。",
				},
			}),
		memoryWriteToolDefinition(),
	}
}

func memoryWriteToolDefinition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        metaToolMemoryWrite,
		Description: "写入/更新/删除长期记忆。只记稳定事实与明确偏好（用户称呼、业务口径、长期约定），不记一次性任务上下文，不记录凭证、证件号、密钥等敏感信息（含敏感形态的内容会被直接拒绝）；宁可少写不写错。用户明确要求忘记时执行 delete。每次操作必须给 reason。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": stringSchema("本次工具调用的简短描述，用于向用户说明为什么调用该内部工具或正在做什么。"),
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"create", "update", "delete"},
					"description": "操作类型。",
				},
				"itemId":  numberSchema("条目 ID，update/delete 必填。"),
				"version": numberSchema("update 时读取到的版本号（乐观锁）；不传则直接覆盖。"),
				"layer": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"resident", "detached"},
					"description": "层级。create 默认 detached；update 不传保持原层级。常驻层注入每一轮对话，写入需加倍审慎。",
				},
				"title":         stringSchema("短标题（≤32字），create/update 必填。"),
				"content":       stringSchema("记忆正文，一到三句原子事实（≤500字），create/update 必填。"),
				"retrievalHint": stringSchema("检索提示：什么场景需要想起这条记忆（≤512字），create/update 必填。"),
				"tags":          stringSchema("逗号分隔标签，可选。"),
				"reason":        stringSchema("必填：为什么写入/修改/删除。"),
			},
			"required":             []string{"description", "action", "reason"},
			"additionalProperties": false,
		},
	}
}

type memoryListInput struct {
	Layer   string `json:"layer"`
	Tag     string `json:"tag"`
	Keyword string `json:"keyword"`
	Limit   int    `json:"limit"`
}

type memoryListItemView struct {
	ItemID      uint   `json:"itemId"`
	Layer       string `json:"layer"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Source      string `json:"source"`
	Version     int    `json:"version"`
	UpdatedAt   string `json:"updatedAt"`
}

// executeMemoryList 列出当前作用域记忆索引（不含正文）。
func (s *reactEngineState) executeMemoryList(input json.RawMessage) (string, bool, error) {
	var req memoryListInput
	_ = json.Unmarshal(input, &req)

	limit := req.Limit
	if limit <= 0 {
		limit = memoryListDefaultSize
	}
	if limit > memoryListMaxSize {
		limit = memoryListMaxSize
	}

	cfg := conf.GetReactRuntimeConfig().Memory
	scope := resolveMemoryScope(s.req.payload.CallerKey, s.req.userName, cfg.MemoryAllowUserScope())
	items, err := model.FindActiveMemoryItemsByOwners(s.ctx, scope.owners)
	if err != nil {
		return "", true, err
	}

	layer := strings.TrimSpace(req.Layer)
	if layer == "" {
		layer = model.MemoryLayerDetached
	}
	tag := strings.TrimSpace(req.Tag)
	keyword := strings.ToLower(strings.TrimSpace(req.Keyword))
	views := make([]memoryListItemView, 0, limit)
	for _, item := range mergeMemoryItems(items) {
		if layer != "all" && item.Layer != layer {
			continue
		}
		if tag != "" && !memoryItemHasTag(item.Tags, tag) {
			continue
		}
		if keyword != "" && !memoryItemMatchesKeyword(item, keyword) {
			continue
		}
		views = append(views, memoryListItemView{
			ItemID:      item.ID,
			Layer:       item.Layer,
			Title:       item.Title,
			Description: item.Description,
			Tags:        item.Tags,
			Source:      item.Source,
			Version:     item.Version,
			UpdatedAt:   item.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
		if len(views) >= limit {
			break
		}
	}
	data, _ := json.Marshal(map[string]interface{}{"items": views, "total": len(views)})
	return string(data), false, nil
}

func memoryItemHasTag(tags, tag string) bool {
	for _, part := range strings.Split(tags, ",") {
		if strings.TrimSpace(part) == tag {
			return true
		}
	}
	return false
}

func memoryItemMatchesKeyword(item model.MemoryItem, keyword string) bool {
	haystack := strings.ToLower(item.Title + "\n" + item.Description + "\n" + item.Tags)
	return strings.Contains(haystack, keyword)
}

type memoryReadInput struct {
	ItemIDs []uint64 `json:"itemIds"`
}

type memoryReadItemView struct {
	ItemID     uint   `json:"itemId"`
	Layer      string `json:"layer"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Tags       string `json:"tags"`
	Source     string `json:"source"`
	Version    int    `json:"version"`
	LastReason string `json:"lastReason"`
	UpdatedAt  string `json:"updatedAt"`
}

// executeMemoryRead 按 ID 读取记忆全文；条目归属不在当前作用域内时整单拒绝，防跨用户读取。
func (s *reactEngineState) executeMemoryRead(input json.RawMessage) (string, bool, error) {
	var req memoryReadInput
	_ = json.Unmarshal(input, &req)
	if len(req.ItemIDs) == 0 {
		return "", true, fmt.Errorf("itemIds 不能为空")
	}
	if len(req.ItemIDs) > memoryReadMaxItems {
		return "", true, fmt.Errorf("一次最多读取 %d 条记忆", memoryReadMaxItems)
	}

	cfg := conf.GetReactRuntimeConfig().Memory
	scope := resolveMemoryScope(s.req.payload.CallerKey, s.req.userName, cfg.MemoryAllowUserScope())
	views := make([]memoryReadItemView, 0, len(req.ItemIDs))
	for _, rawID := range req.ItemIDs {
		if rawID == 0 {
			continue
		}
		item, err := model.GetActiveMemoryItemByID(s.ctx, uint(rawID))
		if err != nil {
			return "", true, err
		}
		if item == nil {
			return "", true, fmt.Errorf("记忆 #%d 不存在或已删除", rawID)
		}
		if _, ok := scope.allowedKeys[memoryOwnerKey(model.MemoryOwner{OwnerType: item.OwnerType, OwnerKey: item.OwnerKey})]; !ok {
			return "", true, fmt.Errorf("记忆 #%d 不在当前作用域内，无权读取", rawID)
		}
		views = append(views, memoryReadItemView{
			ItemID:     item.ID,
			Layer:      item.Layer,
			Title:      item.Title,
			Content:    item.Content,
			Tags:       item.Tags,
			Source:     item.Source,
			Version:    item.Version,
			LastReason: item.LastReason,
			UpdatedAt:  item.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	if len(views) == 0 {
		return "", true, fmt.Errorf("itemIds 不能为空")
	}
	data, _ := json.Marshal(map[string]interface{}{"items": views})
	return string(data), false, nil
}

type memoryWriteInput struct {
	Action      string `json:"action"`
	ItemID      uint64 `json:"itemId"`
	Version     int    `json:"version"`
	Layer       string `json:"layer"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Description string `json:"retrievalHint"`
	Tags        string `json:"tags"`
	Reason      string `json:"reason"`
}

// executeMemoryWrite 是引擎侧记忆写入口：解析入参、解析作用域后交给统一写核心
// （service/memory.ApplyMutation，与管理面共用校验/幂等/修订流水/敏感拦截/指标）。
func (s *reactEngineState) executeMemoryWrite(input json.RawMessage) (string, bool, error) {
	var req memoryWriteInput
	if err := json.Unmarshal(input, &req); err != nil {
		return "", true, fmt.Errorf("memory_write input must be a valid JSON object")
	}

	cfg := conf.GetReactRuntimeConfig().Memory
	scope := resolveMemoryScope(s.req.payload.CallerKey, s.req.userName, cfg.MemoryAllowUserScope())
	result, err := memoryService.ApplyMutation(s.ctx, memoryService.MutationInput{
		Action:           req.Action,
		ItemID:           uint(req.ItemID),
		Version:          req.Version,
		Layer:            req.Layer,
		Title:            req.Title,
		Content:          req.Content,
		Description:      req.Description,
		Tags:             req.Tags,
		Reason:           req.Reason,
		Owner:            scope.writeOwner,
		Source:           model.MemorySourceModel,
		CreatedBy:        s.runID,
		AllowedOwnerKeys: scope.allowedKeys,
	})
	if err != nil {
		return "", true, err
	}
	data, _ := json.Marshal(result)
	return string(data), false, nil
}
