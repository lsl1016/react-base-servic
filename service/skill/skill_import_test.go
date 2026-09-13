package skill

import (
	"strings"
	"testing"
)

func TestSplitSkillMarkdown(t *testing.T) {
	markdown := "---\n" +
		"name: daily-report\n" +
		"description: 生成运营日报\n" +
		"triggers:\n" +
		"  - 日报\n" +
		"  - daily report\n" +
		"---\n" +
		"# 日报生成流程\n" +
		"先汇总 GMV 再输出结论。\n"

	frontmatter, body := splitSkillMarkdown(markdown)
	if !strings.Contains(frontmatter, "name: daily-report") {
		t.Fatalf("frontmatter 缺少 name: %q", frontmatter)
	}
	if !strings.Contains(frontmatter, "triggers:") || !strings.Contains(frontmatter, "- 日报") {
		t.Fatalf("frontmatter 缺少 triggers 列表: %q", frontmatter)
	}
	if body != "# 日报生成流程\n先汇总 GMV 再输出结论。\n" {
		t.Fatalf("body 不符合预期: %q", body)
	}
}

func TestSplitSkillMarkdownCRLF(t *testing.T) {
	markdown := "---\r\nname: ops-runbook\r\n---\r\n正文"
	frontmatter, body := splitSkillMarkdown(markdown)
	if !strings.Contains(frontmatter, "ops-runbook") {
		t.Fatalf("CRLF frontmatter 解析失败: %q", frontmatter)
	}
	if body != "正文" {
		t.Fatalf("CRLF body 解析失败: %q", body)
	}
}

func TestSplitSkillMarkdownInvalid(t *testing.T) {
	if _, body := splitSkillMarkdown("没有 frontmatter 的纯文本"); body != "没有 frontmatter 的纯文本" {
		t.Fatalf("无 frontmatter 时应整体视为 body: %q", body)
	}
	// 只有开头 --- 没有闭合 --- 时不应把内容误当 frontmatter
	if frontmatter, _ := splitSkillMarkdown("---\nname: x\n"); frontmatter != "" {
		t.Fatalf("未闭合 frontmatter 应返回空: %q", frontmatter)
	}
}

func TestNormalizeSkillTriggers(t *testing.T) {
	triggers, err := normalizeSkillTriggers([]string{" 日报 ", "", "daily report", "日报", "daily report"})
	if err != nil {
		t.Fatalf("清洗应成功: %v", err)
	}
	if len(triggers) != 2 || triggers[0] != "日报" || triggers[1] != "daily report" {
		t.Fatalf("去空白/去重后不符合预期: %v", triggers)
	}

	many := make([]string, maxSkillTriggers+1)
	for i := range many {
		many[i] = "关键词" + strings.Repeat("k", i+1)
	}
	if _, err := normalizeSkillTriggers(many); err == nil {
		t.Fatal("超上限应报错而非静默截断")
	}

	long := strings.Repeat("长", maxSkillTriggerRuneLen+1)
	if _, err := normalizeSkillTriggers([]string{long}); err == nil {
		t.Fatal("单关键词超长应报错")
	}
}

func TestDeriveSkillDescription(t *testing.T) {
	if got := deriveSkillDescription("# 日报生成流程\n先汇总再输出。\n"); got != "日报生成流程" {
		t.Fatalf("标题行剥标记结果不符合预期: %q", got)
	}
	if got := deriveSkillDescription("\n\n  正文首行  \n第二行"); got != "正文首行" {
		t.Fatalf("空行跳过与 trim 结果不符合预期: %q", got)
	}
	if got := deriveSkillDescription(strings.Repeat("字", skillDescriptionMaxRunes+10)); !strings.HasSuffix(got, "…") {
		t.Fatalf("超长截断应带省略号: %q", got)
	}
	if got := deriveSkillDescription("   "); got != "" {
		t.Fatalf("全空白正文应返回空: %q", got)
	}
}

func TestIsSkillMdEntry(t *testing.T) {
	valid := []string{"daily-report/SKILL.md", "SKILL.md", "a/b/Skill.MD", "dir\\Skill.md"}
	for _, name := range valid {
		if !isSkillMdEntry(name) {
			t.Fatalf("%q 应识别为 SKILL.md 条目", name)
		}
	}
	invalid := []string{
		"docs/SKILL.md.bak", "README.md", "src/skill.mdx",
		"__MACOSX/daily-report/SKILL.md", ".hidden/SKILL.md", "daily-report/.skill.md",
	}
	for _, name := range invalid {
		if isSkillMdEntry(name) {
			t.Fatalf("%q 不应识别为 SKILL.md 条目", name)
		}
	}
}
