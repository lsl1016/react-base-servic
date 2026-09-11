package react

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApplyReactTodoWriteReplaceCreatesNewState(t *testing.T) {
	state, result, err := applyReactTodoWrite(`{"version":1,"items":[{"id":"old","content":"旧任务","status":"in_progress","createdAt":1,"updatedAt":1}],"updatedAt":1}`, json.RawMessage(`{"merge":false,"todos":[{"id":"inspect","content":"查看当前实现","status":"in_progress"}]}`), 2)
	if err != nil {
		t.Fatalf("apply todo write failed: %v", err)
	}
	if result.WasMerge || result.TotalCount != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(state.Items) != 1 || state.Items[0].ID != "inspect" || state.Items[0].Content != "查看当前实现" || state.Items[0].Status != reactTodoStatusInProgress {
		t.Fatalf("unexpected state items: %+v", state.Items)
	}
	if state.Items[0].CreatedAt != 2 || state.Items[0].UpdatedAt != 2 || state.UpdatedAt != 2 {
		t.Fatalf("unexpected timestamps: %+v", state)
	}
}

func TestApplyReactTodoWriteMergeUpdatesExistingItem(t *testing.T) {
	current := `{"version":1,"items":[{"id":"inspect","content":"查看当前实现","status":"in_progress","createdAt":1,"updatedAt":1},{"id":"implement","content":"实现功能","status":"pending","createdAt":1,"updatedAt":1}],"updatedAt":1}`
	state, result, err := applyReactTodoWrite(current, json.RawMessage(`{"merge":true,"todos":[{"id":"inspect","status":"completed"}]}`), 3)
	if err != nil {
		t.Fatalf("apply todo write failed: %v", err)
	}
	if !result.WasMerge || result.TotalCount != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if state.Items[0].Content != "查看当前实现" || state.Items[0].Status != reactTodoStatusCompleted {
		t.Fatalf("existing item should keep content and update status: %+v", state.Items[0])
	}
	if state.Items[0].CreatedAt != 1 || state.Items[0].UpdatedAt != 3 {
		t.Fatalf("existing item should keep createdAt and refresh updatedAt: %+v", state.Items[0])
	}
	if state.Items[1].Status != reactTodoStatusPending {
		t.Fatalf("untouched item should be preserved: %+v", state.Items[1])
	}
}

func TestApplyReactTodoWriteMergeRequiresContentForNewItem(t *testing.T) {
	_, _, err := applyReactTodoWrite(``, json.RawMessage(`{"merge":true,"todos":[{"id":"inspect","status":"pending"}]}`), 1)
	if err == nil || !strings.Contains(err.Error(), "new todo content is required") {
		t.Fatalf("expected new item content error, got: %v", err)
	}
}

func TestApplyReactTodoWriteRejectsInvalidStatus(t *testing.T) {
	_, _, err := applyReactTodoWrite(``, json.RawMessage(`{"merge":false,"todos":[{"id":"inspect","content":"查看当前实现","status":"doing"}]}`), 1)
	if err == nil || !strings.Contains(err.Error(), "status invalid") {
		t.Fatalf("expected invalid status error, got: %v", err)
	}
}

func TestRenderReactTodoReminderUsesReadableList(t *testing.T) {
	state := encodeReactTodoState(reactTodoState{
		Version: reactTodoStateVersion,
		Items: []reactTodoItem{
			{ID: "inspect", Content: "查看当前实现", Status: reactTodoStatusInProgress, CreatedAt: 1, UpdatedAt: 1},
			{ID: "implement", Content: "实现功能", Status: reactTodoStatusPending, CreatedAt: 1, UpdatedAt: 1},
		},
		UpdatedAt: 1,
	})
	reminder := renderReactTodoReminder(state)
	for _, required := range []string{"<todo_list>", "- [in_progress] inspect: 查看当前实现", "- [pending] implement: 实现功能", "todo_write", "</todo_list>"} {
		if !strings.Contains(reminder, required) {
			t.Fatalf("todo reminder missing %q: %s", required, reminder)
		}
	}
	if strings.Contains(reminder, `"items"`) {
		t.Fatalf("todo reminder should not expose raw json: %s", reminder)
	}
}

func TestRenderReactTodoReminderShowsNoActiveStatus(t *testing.T) {
	state := encodeReactTodoState(reactTodoState{
		Version: reactTodoStateVersion,
		Items: []reactTodoItem{
			{ID: "inspect", Content: "查看当前实现", Status: reactTodoStatusCompleted, CreatedAt: 1, UpdatedAt: 1},
		},
		UpdatedAt: 1,
	})
	reminder := renderReactTodoReminder(state)
	if !strings.Contains(reminder, "<todo_status>") || !strings.Contains(reminder, "当前没有活跃 todo") {
		t.Fatalf("unexpected no-active reminder: %s", reminder)
	}
}
