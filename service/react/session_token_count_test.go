package react

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	llm "react-base-service/api/llm"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 一次性脚本：统计指定 session 在 tblLlmReactMessage 中存储的所有消息所占的 token。
// 运行：go test ./service/react -run TestCountSessionTokens -v -count=1
func TestCountSessionTokens(t *testing.T) {
	const sessionID = "session_80d73ee5e3814182b5aefd5baad908d0"

	dsn := "root:root@tcp(127.0.0.1:3307)/llm?charset=utf8mb4&parseTime=true&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("connect mysql failed: %v", err)
	}

	type row struct {
		ID          uint   `gorm:"column:id"`
		MessageID   string `gorm:"column:message_id"`
		RunID       string `gorm:"column:run_id"`
		Seq         int    `gorm:"column:seq"`
		StepIndex   int    `gorm:"column:step_index"`
		Role        string `gorm:"column:role"`
		MessageType string `gorm:"column:message_type"`
		ContentJSON string `gorm:"column:content_json"`
	}

	var rows []row
	if err := db.Table("tblLlmReactMessage").
		Where("session_id = ?", sessionID).
		Order("created_at ASC, seq ASC").
		Find(&rows).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(rows) == 0 {
		t.Skipf("no rows found for session %s", sessionID)
	}

	type stat struct {
		Role        string
		MessageType string
		Seq         int
		Step        int
		MessageID   string
		Tokens      int
		Bytes       int
	}
	var stats []stat
	totalTokens := 0
	totalBytes := 0

	byType := map[string]int{}

	for _, r := range rows {
		// content_json 直接是 ChatMessage 序列化（assistant / tool_result / compact / user_input 等不同结构）。
		// 大多数情况是 ChatMessage；user_input 是 {content,controlContext,llmContext,modelMessage}；compact 是 {summary}。
		// 统一按 ChatMessage 解析；对纯文本 content（user_input/compact）也能落到 Content 字段或 raw 字符串。
		msg := parseAsChatMessage(r.ContentJSON, r.Role, r.MessageType)
		toks := estimateMessagesTokens([]llm.ChatMessage{msg})

		stats = append(stats, stat{
			Role:        r.Role,
			MessageType: r.MessageType,
			Seq:         r.Seq,
			Step:        r.StepIndex,
			MessageID:   r.MessageID,
			Tokens:      toks,
			Bytes:       len(r.ContentJSON),
		})
		totalTokens += toks
		totalBytes += len(r.ContentJSON)
		byType[r.MessageType] += toks
	}

	t.Logf("session=%s rows=%d totalContentBytes=%d totalTokens=%d",
		sessionID, len(rows), totalBytes, totalTokens)

	// 按消息类型聚合
	keys := make([]string, 0, len(byType))
	for k := range byType {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t.Logf("---- breakdown by message_type ----")
	for _, k := range keys {
		t.Logf("  %-30s tokens=%d", k, byType[k])
	}

	t.Logf("---- per-message detail ----")
	t.Logf("%-6s %-6s %-30s %-10s %-10s %s",
		"seq", "step", "message_type", "tokens", "bytes", "message_id")
	for _, s := range stats {
		t.Logf("%-6d %-6d %-30s %-10d %-10d %s",
			s.Seq, s.Step, s.MessageType, s.Tokens, s.Bytes, s.MessageID)
	}

	fmt.Printf("\n=== SESSION %s TOTAL TOKENS = %d (bytes=%d, rows=%d) ===\n",
		sessionID, totalTokens, totalBytes, len(rows))
}

// parseAsChatMessage 尝试将存储的 content_json 解析为 llm.ChatMessage。
// 不同 message_type 的 schema 不同：assistant / tool_result 是标准 ChatMessage；
// user_input 形如 {content,controlContext,llmContext,modelMessage}；compact 形如 {summary}。这里做兼容处理。
func parseAsChatMessage(raw, role, messageType string) llm.ChatMessage {
	if raw == "" {
		return llm.ChatMessage{Role: role}
	}
	var msg llm.ChatMessage
	if err := json.Unmarshal([]byte(raw), &msg); err == nil {
		if msg.Role != "" || msg.Content != "" || len(msg.Parts) > 0 {
			return msg
		}
	}
	// fallback：把 raw 当文本计入
	return llm.ChatMessage{Role: role, Content: raw}
}
