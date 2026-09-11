package tool

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateToolConfigAsyncTask(t *testing.T) {
	t.Run("托管异步任务配置合法", func(t *testing.T) {
		raw := json.RawMessage(`{"url":"http://example.com","inputSchema":{"type":"object"},"async":true,"asyncTask":{"schedulerType":"demo"}}`)
		require.NoError(t, validateToolConfig(raw, ToolTypeHTTP))
	})

	t.Run("旧异步配置保持兼容", func(t *testing.T) {
		raw := json.RawMessage(`{"url":"http://example.com","inputSchema":{"type":"object"},"async":true,"asyncHint":"稍后查询"}`)
		require.NoError(t, validateToolConfig(raw, ToolTypeHTTP))
	})

	t.Run("asyncTask要求async为true", func(t *testing.T) {
		raw := json.RawMessage(`{"url":"http://example.com","inputSchema":{"type":"object"},"asyncTask":{"schedulerType":"demo"}}`)
		require.ErrorContains(t, validateToolConfig(raw, ToolTypeHTTP), "async=true")
	})

	t.Run("schedulerType必填", func(t *testing.T) {
		raw := json.RawMessage(`{"url":"http://example.com","inputSchema":{"type":"object"},"async":true,"asyncTask":{"schedulerType":" "}}`)
		require.ErrorContains(t, validateToolConfig(raw, ToolTypeHTTP), "schedulerType")
	})
}
