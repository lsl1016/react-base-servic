package react

import (
	"strings"
	"testing"
)

func TestParseMcpServersJSON(t *testing.T) {
	// 标准 mcpServers 包装
	drafts, err := parseMcpServersJSON(`{
		"mcpServers": {
			"mcp-server": {
				"url": "http://127.0.0.1:18080/api/mcp",
				"headers": { "Authorization": "Bearer k:s" }
			}
		}
	}`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}
	if drafts[0].Name != "mcp-server" || drafts[0].Kind != "http" || drafts[0].Endpoint != "http://127.0.0.1:18080/api/mcp" {
		t.Fatalf("draft mismatch: %+v", drafts[0])
	}
	if !strings.Contains(drafts[0].HeadersJSON, "Authorization") {
		t.Fatalf("headers not serialized: %s", drafts[0].HeadersJSON)
	}

	// 去掉外层包装的单映射也接受
	drafts, err = parseMcpServersJSON(`{"solo": {"url": "https://mcp.example.com/mcp"}}`)
	if err != nil || len(drafts) != 1 || drafts[0].Name != "solo" {
		t.Fatalf("bare map parse failed: %v %+v", err, drafts)
	}

	// command/args 形式必须明确拒绝（stdio 只开放白名单适配器）
	_, err = parseMcpServersJSON(`{"mcpServers": {"bad": {"command": "npx", "args": ["-y", "x"]}}}`)
	if err == nil || !strings.Contains(err.Error(), "command/args") {
		t.Fatalf("command style should be rejected with reason, got %v", err)
	}

	// 缺 url 拒绝
	if _, err := parseMcpServersJSON(`{"mcpServers": {"bad": {}}}`); err == nil || !strings.Contains(err.Error(), "url") {
		t.Fatalf("missing url should be rejected, got %v", err)
	}

	// 非法 JSON 拒绝
	if _, err := parseMcpServersJSON(`not-json`); err == nil {
		t.Fatal("invalid json should be rejected")
	}
}

func TestParseMcpCreateDrafts_Structured(t *testing.T) {
	// status 未传 → 默认启用（指针语义）
	drafts, err := parseMcpCreateDrafts(mcpCreateRequest{Name: "demo", Kind: "http", Endpoint: "http://127.0.0.1:18090/mcp"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if drafts[0].Status != 1 || drafts[0].TimeoutMs != 30000 {
		t.Fatalf("default status/timeout mismatch: %+v", drafts[0])
	}

	// 显式停用
	zero := 0
	drafts, err = parseMcpCreateDrafts(mcpCreateRequest{Name: "demo", Kind: "http", Endpoint: "http://127.0.0.1:18090/mcp", Status: &zero})
	if err != nil || drafts[0].Status != 0 {
		t.Fatalf("explicit disable mismatch: %v %+v", err, drafts)
	}

	// 非白名单 kind 拒绝
	if _, err := parseMcpCreateDrafts(mcpCreateRequest{Name: "demo", Kind: "npx"}); err == nil {
		t.Fatal("non-whitelisted kind should be rejected")
	}

	// rawConfig 优先且缺 name 时可用
	drafts, err = parseMcpCreateDrafts(mcpCreateRequest{RawConfig: `{"mcpServers":{"a":{"url":"http://x.example.com/mcp"}}}`})
	if err != nil || len(drafts) != 1 || drafts[0].Name != "a" {
		t.Fatalf("rawConfig precedence failed: %v %+v", err, drafts)
	}

	// 无 rawConfig 且无 name → 报错
	if _, err := parseMcpCreateDrafts(mcpCreateRequest{}); err == nil {
		t.Fatal("missing name without rawConfig should be rejected")
	}
}

func TestNormalizeMcpTimeout(t *testing.T) {
	if got := normalizeMcpTimeout(0); got != 30000 {
		t.Fatalf("default timeout want 30000, got %d", got)
	}
	if got := normalizeMcpTimeout(5000); got != 5000 {
		t.Fatalf("passthrough timeout want 5000, got %d", got)
	}
	if got := normalizeMcpTimeout(999999); got != 300000 {
		t.Fatalf("capped timeout want 300000, got %d", got)
	}
}

func TestMarshalMcpHeaders(t *testing.T) {
	if got, err := marshalMcpHeaders(nil); err != nil || got != "" {
		t.Fatalf("nil headers want empty, got %q err %v", got, err)
	}
	got, err := marshalMcpHeaders(map[string]string{"Authorization": "Bearer k"})
	if err != nil || !strings.Contains(got, "Authorization") {
		t.Fatalf("headers serialize failed: %q %v", got, err)
	}
}
