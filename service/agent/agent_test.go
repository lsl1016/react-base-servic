package agent

import (
	"strings"
	"testing"
)

func TestSplitAgentMarkdown(t *testing.T) {
	markdown := "---\n" +
		"agent_key: dba-agent\n" +
		"name: DBA 专家\n" +
		"description: |\n" +
		"  适用：数据库性能排查\n" +
		"  不适用：非数据库问题\n" +
		"tools:\n" +
		"  - query_schema\n" +
		"  - explain_sql\n" +
		"max_steps: 6\n" +
		"---\n" +
		"你是一名资深 DBA。\n" +
		"排查慢查询时先看执行计划。\n"

	frontmatter, body := splitAgentMarkdown(markdown)
	if !strings.Contains(frontmatter, "agent_key: dba-agent") {
		t.Fatalf("frontmatter 缺少 agent_key: %q", frontmatter)
	}
	if !strings.Contains(frontmatter, "tools:") || !strings.Contains(frontmatter, "- query_schema") {
		t.Fatalf("frontmatter 缺少 tools 列表: %q", frontmatter)
	}
	if body != "你是一名资深 DBA。\n排查慢查询时先看执行计划。\n" {
		t.Fatalf("body 不符合预期: %q", body)
	}
}

func TestSplitAgentMarkdownCRLF(t *testing.T) {
	markdown := "---\r\nagent_key: ops-agent\r\nname: OPS\r\n---\r\n正文提示词"
	frontmatter, body := splitAgentMarkdown(markdown)
	if !strings.Contains(frontmatter, "ops-agent") {
		t.Fatalf("CRLF frontmatter 解析失败: %q", frontmatter)
	}
	if body != "正文提示词" {
		t.Fatalf("CRLF body 解析失败: %q", body)
	}
}

func TestSplitAgentMarkdownInvalid(t *testing.T) {
	if _, body := splitAgentMarkdown("没有 frontmatter 的纯文本"); body != "没有 frontmatter 的纯文本" {
		t.Fatalf("无 frontmatter 时应整体视为 body: %q", body)
	}
	// 只有开头 --- 没有闭合 --- 时不应把正文误当 frontmatter
	frontmatter, _ := splitAgentMarkdown("---\nagent_key: x\n")
	if frontmatter != "" {
		t.Fatalf("未闭合 frontmatter 应返回空: %q", frontmatter)
	}
}

func TestValidateAgentKey(t *testing.T) {
	valid := []string{"dba-agent", "ops_agent", "code-agent-2"}
	for _, key := range valid {
		if err := validateAgentKey(key); err != nil {
			t.Fatalf("agent_key %q 应合法: %v", key, err)
		}
	}
	invalid := []string{"", "dba agent", "dba/agent", "dba:agent", "数据库专家"}
	for _, key := range invalid {
		if err := validateAgentKey(key); err == nil {
			t.Fatalf("agent_key %q 应非法", key)
		}
	}
}

func TestValidateReferenceList(t *testing.T) {
	if err := validateReferenceList("tools", nil, 64); err != nil {
		t.Fatalf("空白名单应合法: %v", err)
	}
	if err := validateReferenceList("tools", []string{"a", "b"}, 64); err != nil {
		t.Fatalf("正常白名单应合法: %v", err)
	}
	if err := validateReferenceList("tools", []string{"a", "a"}, 64); err == nil {
		t.Fatal("重复项应非法")
	}
	if err := validateReferenceList("tools", []string{"a", " "}, 64); err == nil {
		t.Fatal("空白项应非法")
	}
	if err := validateReferenceList("tools", []string{"a", "b"}, 1); err == nil {
		t.Fatal("超限应非法")
	}
}

func TestSanitizeAgentKeyFromName(t *testing.T) {
	if got := sanitizeAgentKeyFromName("DBA 专家 Agent"); got != "dbaagent" {
		t.Fatalf("中文名清洗结果不符合预期: %q", got)
	}
	if got := sanitizeAgentKeyFromName("Ops-Agent 2"); got != "ops-agent2" {
		t.Fatalf("混合名清洗结果不符合预期: %q", got)
	}
}
