package react

import (
	"fmt"

	llm "react-base-service/api/llm"
	"react-base-service/components"
	"react-base-service/service/token"
)

// estimateEntryTokens 估算 ReAct 入口"不可压缩部分"的 token：system prompt + tools schema + 当前 user input。
// history 是可压缩的，不在此函数范围内。
func estimateEntryTokens(systemPrompt string, userMessage llm.ChatMessage, tools []llm.ToolDefinition) int {
	total := 0
	if systemPrompt != "" {
		total += msgRoleOverheadTokens
		total += token.EstimateTokens(systemPrompt)
	}
	total += estimateMessagesTokens([]llm.ChatMessage{userMessage})
	total += estimateToolDefinitionsTokens(tools)
	return total
}

// checkEntryInputTokens 校验 ReAct 入口不可压缩部分是否超过 token_trigger。
// 超过则返回 ErrorReactInputTooLong；tokenTrigger <= 0 时跳过检查。
// 复用 token_trigger 作为上限的理由：压缩救不了不可压缩部分，超过压缩阈值已注定爆窗口。
func checkEntryInputTokens(systemPrompt string, userMessage llm.ChatMessage, tools []llm.ToolDefinition, tokenTrigger int) error {
	if tokenTrigger <= 0 {
		return nil
	}
	estimated := estimateEntryTokens(systemPrompt, userMessage, tools)
	if estimated > tokenTrigger {
		return components.ErrorReactInputTooLong.Sprintf(
			fmt.Sprintf("estimated=%d, tokenTrigger=%d", estimated, tokenTrigger))
	}
	return nil
}
