package react

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	llm "react-base-service/api/llm"
	"react-base-service/conf"
	model "react-base-service/models/llm"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	// 常驻层/按需层条目字段长度约束（按 rune 计），与 DDL 注释保持一致。
	maxMemoryTitleRunes       = 32
	maxMemoryContentRunes     = 500
	maxMemoryDescriptionRunes = 512
	maxMemoryReasonRunes      = 512

	memoryReadMaxItems    = 10
	memoryListDefaultSize = 20
	memoryListMaxSize     = 50
)

// memoryItemKey 计算记忆正文的语义指纹：规范化（去首尾空白、折叠连续空白、ASCII 小写）后取 SHA-256 前 16 位。
// 同一事实的细微排版差异收敛为同一 itemKey，实现 create 幂等。
func memoryItemKey(content string) string {
	normalized := memoryNormalizeContent(content)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])[:16]
}

func memoryNormalizeContent(content string) string {
	return strings.ToLower(strings.Join(strings.Fields(content), " "))
}

// normalizeMemoryTags 归一化标签串：按逗号切分、去空、去重、保持原顺序。
func normalizeMemoryTags(tags string) string {
	parts := strings.Split(tags, ",")
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	return strings.Join(result, ",")
}

// memoryScopeResolved 描述一次 run 可见的记忆空间与写入目标。
type memoryScopeResolved struct {
	owners      []model.MemoryOwner // 可见空间（caller 在前，caller_user 在后）
	writeOwner  model.MemoryOwner   // memory_write 的落库目标
	allowedKeys map[string]struct{} // 可见空间集合（owner_type|owner_key），read/list/write 权限校验用
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
	return owner.OwnerType + "|" + owner.OwnerKey
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
		Description: "写入/更新/删除长期记忆。只记稳定事实与明确偏好（用户称呼、业务口径、长期约定），不记一次性任务上下文，不记录凭证、证件号、密钥等敏感信息；宁可少写不写错。用户明确要求忘记时执行 delete。每次操作必须给 reason。",
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
				"reason":        stringSchema("必填：为什么写入/修改/删除这条记忆。"),
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
	Description string `json:"retrievalHint"`
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

// executeMemoryWrite 是记忆唯一写入口（模型侧）：create 幂等收敛、update 乐观锁、delete 软删，
// 每次成功操作在事务内同步落一条不可变修订流水。
func (s *reactEngineState) executeMemoryWrite(input json.RawMessage) (string, bool, error) {
	var req memoryWriteInput
	if err := json.Unmarshal(input, &req); err != nil {
		return "", true, fmt.Errorf("memory_write input must be a valid JSON object")
	}
	req.Action = strings.TrimSpace(req.Action)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	req.Description = strings.TrimSpace(req.Description)
	req.Tags = normalizeMemoryTags(req.Tags)
	req.Layer = strings.TrimSpace(req.Layer)

	if req.Reason == "" {
		return "", true, fmt.Errorf("reason 必填：说明为什么本次写入/修改/删除")
	}
	if len([]rune(req.Reason)) > maxMemoryReasonRunes {
		return "", true, fmt.Errorf("reason 超长（≤%d字）", maxMemoryReasonRunes)
	}
	if req.Layer != "" && req.Layer != model.MemoryLayerResident && req.Layer != model.MemoryLayerDetached {
		return "", true, fmt.Errorf("layer 仅支持 resident/detached")
	}

	cfg := conf.GetReactRuntimeConfig().Memory
	scope := resolveMemoryScope(s.req.payload.CallerKey, s.req.userName, cfg.MemoryAllowUserScope())

	var result map[string]interface{}
	err := model.GetLLMDB().WithContext(s.ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		switch req.Action {
		case "create":
			result, txErr = s.memoryWriteCreate(tx, &req, scope, cfg)
		case "update":
			result, txErr = s.memoryWriteUpdate(tx, &req, scope, cfg)
		case "delete":
			result, txErr = s.memoryWriteDelete(tx, &req, scope)
		default:
			txErr = fmt.Errorf("action 仅支持 create/update/delete")
		}
		return txErr
	})
	if err != nil {
		return "", true, err
	}
	data, _ := json.Marshal(result)
	return string(data), false, nil
}

// memoryWriteCreate 新增记忆；同 itemKey 的存量条目（含软删）自动收敛为更新/复活，保证幂等。
func (s *reactEngineState) memoryWriteCreate(tx *gorm.DB, req *memoryWriteInput, scope memoryScopeResolved, cfg conf.ReactMemoryConfig) (map[string]interface{}, error) {
	if err := validateMemoryPayload(req.Title, req.Content, req.Description); err != nil {
		return nil, err
	}
	layer := req.Layer
	if layer == "" {
		layer = model.MemoryLayerDetached
	}

	itemKey := memoryItemKey(req.Content)
	existing, err := model.FindMemoryItemByOwnerAndKeyWithDB(s.ctx, tx, scope.writeOwner, itemKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// 幂等收敛：同一事实重复写入转为更新（软删条目同时复活），reason 标注收敛语义。
		convReq := &memoryWriteInput{
			ItemID:      uint64(existing.ID),
			Layer:       firstNonEmpty(req.Layer, existing.Layer),
			Title:       req.Title,
			Content:     req.Content,
			Description: req.Description,
			Tags:        req.Tags,
			Reason:      "create 命中同指纹条目，收敛为更新；" + req.Reason,
		}
		result, updateErr := s.memoryApplyUpdate(tx, existing, convReq, scope, cfg, true)
		if updateErr != nil {
			return nil, updateErr
		}
		result["converged"] = true
		return result, nil
	}

	if err := checkMemoryLayerCap(s.ctx, tx, scope.writeOwner, layer, cfg, 1); err != nil {
		return nil, err
	}

	item := &model.MemoryItem{
		OwnerType:   scope.writeOwner.OwnerType,
		OwnerKey:    scope.writeOwner.OwnerKey,
		Layer:       layer,
		Title:       req.Title,
		Content:     req.Content,
		Description: req.Description,
		Tags:        req.Tags,
		Source:      model.MemorySourceModel,
		ItemKey:     itemKey,
		Version:     1,
		State:       model.MemoryStateActive,
		LastReason:  req.Reason,
		CreatedBy:   s.runID,
	}
	if err := model.CreateMemoryItemWithDB(s.ctx, tx, item); err != nil {
		return nil, err
	}
	if err := createMemoryRevisionFromItem(s.ctx, tx, item.ID, "create", nil, item, req.Reason, s.runID); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"action":  "create",
		"itemId":  item.ID,
		"layer":   item.Layer,
		"version": item.Version,
		"reason":  req.Reason,
	}, nil
}

// memoryWriteUpdate 按 itemId 更新记忆。
func (s *reactEngineState) memoryWriteUpdate(tx *gorm.DB, req *memoryWriteInput, scope memoryScopeResolved, cfg conf.ReactMemoryConfig) (map[string]interface{}, error) {
	if req.ItemID == 0 {
		return nil, fmt.Errorf("update 需要 itemId")
	}
	item, err := model.GetActiveMemoryItemByID(s.ctx, uint(req.ItemID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("记忆 #%d 不存在或已删除", req.ItemID)
	}
	if err := validateMemoryScopeForItem(scope, item, uint(req.ItemID)); err != nil {
		return nil, err
	}
	if err := validateMemoryPayload(req.Title, req.Content, req.Description); err != nil {
		return nil, err
	}
	return s.memoryApplyUpdate(tx, item, req, scope, cfg, false)
}

// memoryApplyUpdate 执行条目更新（含 create 收敛复用）：乐观锁 + 层级守门 + 修订流水。
func (s *reactEngineState) memoryApplyUpdate(tx *gorm.DB, item *model.MemoryItem, req *memoryWriteInput, scope memoryScopeResolved, cfg conf.ReactMemoryConfig, ignoreVersion bool) (map[string]interface{}, error) {
	targetLayer := item.Layer
	if req.Layer != "" {
		targetLayer = req.Layer
	}
	tags := item.Tags
	if req.Tags != "" {
		tags = req.Tags
	}
	if targetLayer == model.MemoryLayerResident && item.Layer != model.MemoryLayerResident {
		// detached → resident 需要为常驻层腾出容量（本条不计入存量）。
		if err := checkMemoryLayerCap(s.ctx, tx, scope.writeOwner, targetLayer, cfg, 1); err != nil {
			return nil, err
		}
	}
	before := *item
	updates := map[string]interface{}{
		"layer":       targetLayer,
		"title":       req.Title,
		"content":     req.Content,
		"description": req.Description,
		"tags":        tags,
		"state":       model.MemoryStateActive,
		"last_reason": req.Reason,
		"version":     item.Version + 1,
	}
	expectedVersion := 0
	if !ignoreVersion {
		expectedVersion = req.Version
	}
	updated, err := model.UpdateMemoryItemWithVersionWithDB(s.ctx, tx, item.ID, expectedVersion, updates)
	if err != nil {
		return nil, err
	}
	if !updated {
		if req.Version > 0 {
			return nil, fmt.Errorf("记忆 #%d 版本冲突（当前版本已变化），请用 memory_read 重读后重试", item.ID)
		}
		return nil, fmt.Errorf("记忆 #%d 更新失败：条目不存在或状态异常", item.ID)
	}
	after := before
	after.Layer = targetLayer
	after.Title = req.Title
	after.Content = req.Content
	after.Description = req.Description
	after.Tags = tags
	after.State = model.MemoryStateActive
	after.LastReason = req.Reason
	after.Version = before.Version + 1
	if err := createMemoryRevisionFromItem(s.ctx, tx, item.ID, "update", &before, &after, req.Reason, s.runID); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"action":  "update",
		"itemId":  item.ID,
		"layer":   targetLayer,
		"version": after.Version,
		"reason":  req.Reason,
	}, nil
}

// memoryWriteDelete 软删记忆。
func (s *reactEngineState) memoryWriteDelete(tx *gorm.DB, req *memoryWriteInput, scope memoryScopeResolved) (map[string]interface{}, error) {
	if req.ItemID == 0 {
		return nil, fmt.Errorf("delete 需要 itemId")
	}
	item, err := model.GetActiveMemoryItemByID(s.ctx, uint(req.ItemID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("记忆 #%d 不存在或已删除", req.ItemID)
	}
	if err := validateMemoryScopeForItem(scope, item, uint(req.ItemID)); err != nil {
		return nil, err
	}

	before := *item
	updated, err := model.UpdateMemoryItemWithVersionWithDB(s.ctx, tx, item.ID, req.Version, map[string]interface{}{
		"state":       model.MemoryStateDeleted,
		"last_reason": req.Reason,
		"version":     item.Version + 1,
	})
	if err != nil {
		return nil, err
	}
	if !updated {
		if req.Version > 0 {
			return nil, fmt.Errorf("记忆 #%d 版本冲突（当前版本已变化），请用 memory_read 重读后重试", item.ID)
		}
		return nil, fmt.Errorf("记忆 #%d 删除失败：条目不存在或状态异常", item.ID)
	}
	if err := createMemoryRevisionFromItem(s.ctx, tx, item.ID, "delete", &before, nil, req.Reason, s.runID); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"action":  "delete",
		"itemId":  item.ID,
		"version": before.Version + 1,
		"reason":  req.Reason,
	}, nil
}

// checkMemoryLayerCap 常驻/按需层上限守门：extra 是本次操作将要新增的条目数（纯新增传 1），
// 结果总数超过上限才拒绝（上限值本身可达成）。
func checkMemoryLayerCap(ctx *gin.Context, tx *gorm.DB, owner model.MemoryOwner, layer string, cfg conf.ReactMemoryConfig, extra int64) error {
	if layer == model.MemoryLayerResident {
		count, err := model.CountActiveMemoryItemsWithDB(ctx, tx, owner, model.MemoryLayerResident)
		if err != nil {
			return err
		}
		if count+extra > int64(cfg.ResidentMaxItems) {
			return fmt.Errorf("常驻层已满（上限 %d 条）：请先将一条常驻记忆改为 detached（update layer=detached）再写入", cfg.ResidentMaxItems)
		}
		return nil
	}
	count, err := model.CountActiveMemoryItemsWithDB(ctx, tx, owner, "")
	if err != nil {
		return err
	}
	if count+extra > int64(cfg.DetachedMaxItems) {
		return fmt.Errorf("该记忆空间条目数已达软上限（%d 条）：请先整理，删除或合并过时记忆后再写入", cfg.DetachedMaxItems)
	}
	return nil
}

func validateMemoryScopeForItem(scope memoryScopeResolved, item *model.MemoryItem, itemID uint) error {
	if _, ok := scope.allowedKeys[memoryOwnerKey(model.MemoryOwner{OwnerType: item.OwnerType, OwnerKey: item.OwnerKey})]; !ok {
		return fmt.Errorf("记忆 #%d 不在当前作用域内，无权操作", itemID)
	}
	return nil
}

func validateMemoryPayload(title, content, description string) error {
	if title == "" {
		return fmt.Errorf("title 必填")
	}
	if len([]rune(title)) > maxMemoryTitleRunes {
		return fmt.Errorf("title 超长（≤%d字）", maxMemoryTitleRunes)
	}
	if content == "" {
		return fmt.Errorf("content 必填")
	}
	if len([]rune(content)) > maxMemoryContentRunes {
		return fmt.Errorf("content 超长（≤%d字）", maxMemoryContentRunes)
	}
	if description == "" {
		return fmt.Errorf("description 必填")
	}
	if len([]rune(description)) > maxMemoryDescriptionRunes {
		return fmt.Errorf("description 超长（≤%d字）", maxMemoryDescriptionRunes)
	}
	return nil
}

// createMemoryRevisionFromItem 落一条不可变修订流水。
func createMemoryRevisionFromItem(ctx *gin.Context, tx *gorm.DB, itemID uint, action string, before, after *model.MemoryItem, reason, createdBy string) error {
	revision := &model.MemoryRevision{
		ItemID:    itemID,
		Action:    action,
		Reason:    reason,
		Source:    model.MemorySourceModel,
		CreatedBy: createdBy,
	}
	var err error
	if before != nil {
		if revision.BeforeJSON, err = memoryItemSnapshotJSON(before); err != nil {
			return err
		}
	}
	if after != nil {
		if revision.AfterJSON, err = memoryItemSnapshotJSON(after); err != nil {
			return err
		}
	}
	return model.CreateMemoryRevisionWithDB(ctx, tx, revision)
}

func memoryItemSnapshotJSON(item *model.MemoryItem) (string, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
