package react

import (
	"encoding/json"
	"strings"
	"testing"

	model "react-base-service/models/llm"
	toolService "react-base-service/service/tool"
)

func TestResolveEffectiveToolPermission(t *testing.T) {
	cases := []struct {
		tool, agent, want string
	}{
		{"auto", "", "auto"},
		{"", "inherit", "auto"},
		{"auto", "inherit", "auto"},
		{"auto", "confirm", "confirm"},
		{"auto", "confirm_risky", "confirm_risky"},
		{"confirm_risky", "inherit", "confirm_risky"},
		{"confirm", "confirm_risky", "confirm"},
		{"confirm", "auto", "confirm"},
		{"confirm_risky", "confirm", "confirm"},
		{"bogus", "", "auto"},
	}
	for _, c := range cases {
		if got := resolveEffectiveToolPermission(c.tool, c.agent); got != c.want {
			t.Fatalf("resolveEffectiveToolPermission(%q,%q)=%q want %q", c.tool, c.agent, got, c.want)
		}
	}
}

func TestShouldConfirmServerTool(t *testing.T) {
	httpTool := model.Tool{Name: "sql_exec", PermissionMode: toolService.ToolPermissionAuto, Config: `{}`}

	// auto：永不确认
	if d := shouldConfirmServerTool(httpTool, json.RawMessage(`{"sql":"drop table t"}`), ""); d.NeedConfirm {
		t.Fatal("auto 模式不应确认")
	}

	// confirm：无条件确认（agent 收紧同样生效）
	confirmTool := httpTool
	confirmTool.PermissionMode = toolService.ToolPermissionConfirm
	if d := shouldConfirmServerTool(confirmTool, json.RawMessage(`{}`), ""); !d.NeedConfirm {
		t.Fatal("confirm 模式应确认")
	}
	if d := shouldConfirmServerTool(httpTool, json.RawMessage(`{}`), toolService.ToolPermissionConfirm); !d.NeedConfirm {
		t.Fatal("agent 级 confirm 收紧应确认")
	}

	// confirm_risky：命中内置风险表（drop table / delete from / restart…）
	riskyTool := httpTool
	riskyTool.PermissionMode = toolService.ToolPermissionConfirmRisky
	for _, input := range []string{
		`{"sql":"DROP TABLE users"}`,
		`{"sql":"delete from orders where id=1"}`,
		`{"action":"restart service"}`,
	} {
		if d := shouldConfirmServerTool(riskyTool, json.RawMessage(input), ""); !d.NeedConfirm {
			t.Fatalf("confirm_risky 应命中风险正则: %s", input)
		}
	}
	// 未命中：直接放行
	if d := shouldConfirmServerTool(riskyTool, json.RawMessage(`{"sql":"select 1"}`), ""); d.NeedConfirm {
		t.Fatal("只读入参不应确认")
	}
	// 工具名命中
	riskyNamed := riskyTool
	riskyNamed.Name = "kill_query_tool"
	if d := shouldConfirmServerTool(riskyNamed, json.RawMessage(`{}`), ""); !d.NeedConfirm {
		t.Fatal("工具名命中风险词应确认")
	}

	// 自定义 riskPatterns 覆盖内置表：只匹配 foo，内置 drop table 不再触发。
	// 注意 Go regexp 默认区分大小写，自定义正则需自带 (?i) 才大小写不敏感。
	customTool := riskyTool
	customTool.Config = `{"riskPatterns":["foo"]}`
	if d := shouldConfirmServerTool(customTool, json.RawMessage(`{"sql":"drop table t"}`), ""); d.NeedConfirm {
		t.Fatal("自定义正则应覆盖内置表")
	}
	if d := shouldConfirmServerTool(customTool, json.RawMessage(`{"x":"foo"}`), ""); !d.NeedConfirm {
		t.Fatal("自定义正则应精确命中")
	}
	if d := shouldConfirmServerTool(customTool, json.RawMessage(`{"x":"FOO"}`), ""); d.NeedConfirm {
		t.Fatal("无 (?i) 的自定义正则应区分大小写")
	}

	// 非法正则按未命中处理
	badTool := riskyTool
	badTool.Config = `{"riskPatterns":["("]}`
	if d := shouldConfirmServerTool(badTool, json.RawMessage(`{"x":"anything"}`), ""); d.NeedConfirm {
		t.Fatal("非法正则应按未命中处理")
	}
}

func TestRenderToolRejectedResult(t *testing.T) {
	got := renderToolRejectedResult(model.Tool{Name: "sql_exec"}, "测试拒绝")
	if !strings.Contains(got, "sql_exec") || !strings.Contains(got, "测试拒绝") || !strings.Contains(got, "用户拒绝") {
		t.Fatalf("拒绝文案不符合预期: %s", got)
	}
}
