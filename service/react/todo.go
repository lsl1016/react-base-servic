package react

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	reactTodoStateVersion = 1

	reactTodoStatusPending    = "pending"
	reactTodoStatusInProgress = "in_progress"
	reactTodoStatusCompleted  = "completed"
	reactTodoStatusCancelled  = "cancelled"
)

var reactTodoStatusSet = map[string]bool{
	reactTodoStatusPending:    true,
	reactTodoStatusInProgress: true,
	reactTodoStatusCompleted:  true,
	reactTodoStatusCancelled:  true,
}

type reactTodoItem struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type reactTodoState struct {
	Version   int             `json:"version"`
	Items     []reactTodoItem `json:"items"`
	UpdatedAt int64           `json:"updatedAt"`
}

type reactTodoWriteItem struct {
	ID         string
	HasContent bool
	Content    string
	HasStatus  bool
	Status     string
}

type reactTodoWriteInput struct {
	Merge bool
	Todos []reactTodoWriteItem
}

type reactTodoWriteResult struct {
	Todos      []reactTodoItem `json:"todos"`
	TotalCount int             `json:"totalCount"`
	WasMerge   bool            `json:"wasMerge"`
}

func nowUnixMilli() int64 {
	return time.Now().UnixMilli()
}

func applyReactTodoWrite(currentJSON string, patchJSON json.RawMessage, now int64) (reactTodoState, reactTodoWriteResult, error) {
	current, err := decodeReactTodoState(currentJSON, now)
	if err != nil {
		return reactTodoState{}, reactTodoWriteResult{}, err
	}
	input, err := decodeReactTodoWriteInput(patchJSON)
	if err != nil {
		return reactTodoState{}, reactTodoWriteResult{}, err
	}

	var nextItems []reactTodoItem
	if input.Merge {
		nextItems, err = mergeReactTodoItems(current.Items, input.Todos, now)
	} else {
		nextItems, err = replaceReactTodoItems(input.Todos, now)
	}
	if err != nil {
		return reactTodoState{}, reactTodoWriteResult{}, err
	}

	nextState := reactTodoState{
		Version:   reactTodoStateVersion,
		Items:     nextItems,
		UpdatedAt: now,
	}
	return nextState, reactTodoWriteResult{
		Todos:      cloneReactTodoItems(nextState.Items),
		TotalCount: len(nextState.Items),
		WasMerge:   input.Merge,
	}, nil
}

func decodeReactTodoState(currentJSON string, now int64) (reactTodoState, error) {
	currentJSON = strings.TrimSpace(currentJSON)
	if currentJSON == "" || currentJSON == "null" {
		return reactTodoState{Version: reactTodoStateVersion, Items: []reactTodoItem{}, UpdatedAt: now}, nil
	}
	var raw interface{}
	if err := json.Unmarshal([]byte(currentJSON), &raw); err != nil {
		return reactTodoState{}, err
	}
	return normalizeReactTodoState(raw, now)
}

func normalizeReactTodoState(raw interface{}, now int64) (reactTodoState, error) {
	stateMap, ok := raw.(map[string]interface{})
	if !ok {
		return reactTodoState{}, fmt.Errorf("todo state must be object")
	}
	items, err := normalizeReactTodoItems(stateMap["items"], now)
	if err != nil {
		return reactTodoState{}, err
	}
	updatedAt := normalizeReactTodoTimestamp(stateMap["updatedAt"], now)
	return reactTodoState{Version: reactTodoStateVersion, Items: items, UpdatedAt: updatedAt}, nil
}

func normalizeReactTodoItems(raw interface{}, now int64) ([]reactTodoItem, error) {
	if raw == nil {
		return []reactTodoItem{}, nil
	}
	rawItems, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("todo items must be array")
	}
	items := make([]reactTodoItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		itemMap, ok := rawItem.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("todo item must be object")
		}
		id := strings.TrimSpace(readReactTodoString(itemMap["id"]))
		content := strings.TrimSpace(readReactTodoString(itemMap["content"]))
		if id == "" || content == "" {
			continue
		}
		status := strings.TrimSpace(readReactTodoString(itemMap["status"]))
		if !isReactTodoStatus(status) {
			status = reactTodoStatusPending
		}
		createdAt := normalizeReactTodoTimestamp(itemMap["createdAt"], now)
		items = append(items, reactTodoItem{
			ID:        id,
			Content:   content,
			Status:    status,
			CreatedAt: createdAt,
			UpdatedAt: normalizeReactTodoTimestamp(itemMap["updatedAt"], createdAt),
		})
	}
	return items, nil
}

func decodeReactTodoWriteInput(raw json.RawMessage) (reactTodoWriteInput, error) {
	if len(raw) == 0 || !json.Valid(raw) {
		return reactTodoWriteInput{}, fmt.Errorf("todo_write input must be a valid JSON object")
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		return reactTodoWriteInput{}, err
	}
	if data == nil {
		return reactTodoWriteInput{}, fmt.Errorf("todo_write input must be object")
	}

	var merge bool
	if rawMerge, ok := data["merge"]; ok {
		if err := json.Unmarshal(rawMerge, &merge); err != nil {
			return reactTodoWriteInput{}, fmt.Errorf("todo_write.merge must be boolean")
		}
	} else if _, hasLegacyItems := data["items"]; hasLegacyItems {
		merge = true
	} else {
		return reactTodoWriteInput{}, fmt.Errorf("todo_write.merge is required")
	}

	rawTodos, ok := data["todos"]
	if !ok {
		rawTodos = data["items"]
	}
	items, err := decodeReactTodoWriteItems(rawTodos)
	if err != nil {
		return reactTodoWriteInput{}, err
	}
	return reactTodoWriteInput{Merge: merge, Todos: items}, nil
}

func decodeReactTodoWriteItems(raw json.RawMessage) ([]reactTodoWriteItem, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("todo_write.todos is required")
	}
	var values []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("todo_write.todos must be array")
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("todo_write.todos must not be empty")
	}
	items := make([]reactTodoWriteItem, 0, len(values))
	for _, value := range values {
		item, err := decodeReactTodoWriteItem(value)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func decodeReactTodoWriteItem(value map[string]json.RawMessage) (reactTodoWriteItem, error) {
	if value == nil {
		return reactTodoWriteItem{}, fmt.Errorf("todo_write.todos item must be object")
	}
	id := strings.TrimSpace(decodeOptionalTodoString(value["id"]))
	if id == "" {
		return reactTodoWriteItem{}, fmt.Errorf("todo_write.todos.id is required")
	}
	item := reactTodoWriteItem{ID: id}
	if rawContent, ok := value["content"]; ok {
		item.HasContent = true
		item.Content = strings.TrimSpace(decodeOptionalTodoString(rawContent))
	}
	if rawStatus, ok := value["status"]; ok {
		item.HasStatus = true
		item.Status = strings.TrimSpace(decodeOptionalTodoString(rawStatus))
		if !isReactTodoStatus(item.Status) {
			return reactTodoWriteItem{}, fmt.Errorf("todo_write.todos.status invalid: %s", item.Status)
		}
	}
	return item, nil
}

func replaceReactTodoItems(updates []reactTodoWriteItem, now int64) ([]reactTodoItem, error) {
	items := make([]reactTodoItem, 0, len(updates))
	seen := make(map[string]bool, len(updates))
	for _, update := range updates {
		if seen[update.ID] {
			return nil, fmt.Errorf("duplicate todo id: %s", update.ID)
		}
		seen[update.ID] = true
		if !update.HasContent || update.Content == "" {
			return nil, fmt.Errorf("new todo content is required: %s", update.ID)
		}
		items = append(items, reactTodoItem{
			ID:        update.ID,
			Content:   update.Content,
			Status:    normalizeReactTodoWriteStatus(update),
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return items, nil
}

func mergeReactTodoItems(current []reactTodoItem, updates []reactTodoWriteItem, now int64) ([]reactTodoItem, error) {
	items := cloneReactTodoItems(current)
	indexByID := make(map[string]int, len(items)+len(updates))
	for index, item := range items {
		indexByID[item.ID] = index
	}
	for _, update := range updates {
		if index, ok := indexByID[update.ID]; ok {
			if update.HasContent && update.Content != "" {
				items[index].Content = update.Content
			}
			if update.HasStatus {
				items[index].Status = update.Status
			}
			items[index].UpdatedAt = now
			continue
		}
		if !update.HasContent || update.Content == "" {
			return nil, fmt.Errorf("new todo content is required: %s", update.ID)
		}
		indexByID[update.ID] = len(items)
		items = append(items, reactTodoItem{
			ID:        update.ID,
			Content:   update.Content,
			Status:    normalizeReactTodoWriteStatus(update),
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return items, nil
}

func normalizeReactTodoWriteStatus(item reactTodoWriteItem) string {
	if item.HasStatus && isReactTodoStatus(item.Status) {
		return item.Status
	}
	return reactTodoStatusPending
}

func encodeReactTodoState(state reactTodoState) string {
	data, _ := json.Marshal(state)
	return string(data)
}

func reactTodoStateFromResult(result reactTodoWriteResult, updatedAt int64) reactTodoState {
	return reactTodoState{Version: reactTodoStateVersion, Items: cloneReactTodoItems(result.Todos), UpdatedAt: updatedAt}
}

func renderReactTodoReminder(todoStateJSON string) string {
	state, err := decodeReactTodoState(todoStateJSON, nowUnixMilli())
	if err != nil || len(state.Items) == 0 {
		return renderReactTodoPolicyReminder()
	}
	if !hasActiveReactTodoItems(state.Items) {
		return strings.Join([]string{
			"<todo_status>",
			"当前没有活跃 todo。历史上的任务已全部 completed 或 cancelled；你可以随时使用 todo_write 创建新的 todo 来追踪当前任务。",
			"如果后续创建了 todo，任务推进、完成、跳过或无法继续时必须继续使用 todo_write 更新任务状态。",
			"</todo_status>",
			renderReactTodoPolicyReminder(),
		}, "\n")
	}
	return strings.Join([]string{
		"<todo_list>",
		renderReactTodoList(state.Items),
		"强制规则：",
		"1. 在输出最终答复前，必须先调用 todo_write 更新所有 pending 或 in_progress todo。",
		"2. 已完成的任务标记为 completed；未完成、跳过、无法继续或不再需要的任务标记为 cancelled。",
		"3. 只要还有 pending 或 in_progress todo，就不要直接输出最终答复。",
		"4. 不要为了收口创建无关的新 todo。",
		"</todo_list>",
	}, "\n")
}

func renderReactTodoPolicyReminder() string {
	return strings.Join([]string{
		"<todo_policy>",
		"如果当前任务是复杂任务，且当前没有活跃 todo，必须先调用 todo_write 创建 todo。",
		"复杂任务包括：3 步以上、跨文件、多需求、排障、重构、联调、验证类任务。",
		"简单问答或 1-2 步小任务不要用 todo。",
		"</todo_policy>",
	}, "\n")
}

func renderReactTodoList(items []reactTodoItem) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- [%s] %s: %s", item.Status, item.ID, item.Content))
	}
	return strings.Join(lines, "\n")
}

func hasActiveReactTodoItems(items []reactTodoItem) bool {
	for _, item := range items {
		if item.Status == reactTodoStatusPending || item.Status == reactTodoStatusInProgress {
			return true
		}
	}
	return false
}

func cloneReactTodoItems(items []reactTodoItem) []reactTodoItem {
	cloned := make([]reactTodoItem, 0, len(items))
	cloned = append(cloned, items...)
	return cloned
}

func isReactTodoStatus(status string) bool {
	return reactTodoStatusSet[strings.TrimSpace(status)]
}

func normalizeReactTodoTimestamp(value interface{}, fallback int64) int64 {
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return int64(typed)
		}
	case int64:
		if typed > 0 {
			return typed
		}
	case int:
		if typed > 0 {
			return int64(typed)
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed != "" {
			var parsed int64
			if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return fallback
}

func readReactTodoString(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func decodeOptionalTodoString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return ""
}

func todoStateFromToolResultContent(content string, fallbackUpdatedAt int64) (reactTodoState, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return reactTodoState{}, false
	}
	var normalized NormalizedToolResult
	if err := json.Unmarshal([]byte(content), &normalized); err == nil && strings.TrimSpace(normalized.Content) != "" {
		content = normalized.Content
	}
	var result reactTodoWriteResult
	if err := json.Unmarshal([]byte(content), &result); err == nil && len(result.Todos) > 0 {
		updatedAt := fallbackUpdatedAt
		for _, item := range result.Todos {
			if item.UpdatedAt > updatedAt {
				updatedAt = item.UpdatedAt
			}
		}
		return reactTodoStateFromResult(result, updatedAt), true
	}
	var raw interface{}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return reactTodoState{}, false
	}
	state, err := normalizeReactTodoState(raw, fallbackUpdatedAt)
	if err != nil || len(state.Items) == 0 {
		return reactTodoState{}, false
	}
	return state, true
}

func sortedReactTodoIDs(items []reactTodoItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	sort.Strings(ids)
	return ids
}
