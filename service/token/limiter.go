package token

import (
	"fmt"
	"unicode"

	"react-base-service/api/llm"
	"react-base-service/components"
	"react-base-service/conf"
)

// EstimateTokens 粗略估算文本的 token 数量：
// 按字符类型分段估算，并额外保留少量安全冗余，减少低估。
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	units := 0
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			units += 2
		case isCJKRune(r):
			units += 10
		case r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			units += 3
		case r <= unicode.MaxASCII:
			units += 5
		default:
			units += 8
		}
	}
	return unitsToTokens(units)
}

func unitsToTokens(units int) int {
	// units 以 0.1 token 为粒度，最后统一附加 10% 安全冗余并向上取整。
	safeUnits := (units*11 + 9) / 10
	return (safeUnits + 9) / 10
}

func isCJKRune(r rune) bool {
	return unicode.In(r,
		unicode.Han,
		unicode.Hiragana,
		unicode.Katakana,
		unicode.Hangul,
	)
}

// CheckTokenLimit 检查消息总 token 数是否超出模型上限，超限直接报错
// modelKey 支持旧枚举（gpt/claude/minimax）和新枚举（OpenAI/Anthropic 等），内部统一归一化
func CheckTokenLimit(modelKey string, messages []llm.LLMMessage) error {
	if normalized := llm.NormalizeModelKey(modelKey); normalized != "" {
		modelKey = normalized
	}
	catalog := conf.GetModelCatalog(modelKey)
	if catalog == nil {
		return components.ErrorModelNotFound.Sprintf(modelKey)
	}
	if catalog.MaxContextTokens <= 0 {
		return nil
	}

	total := 0
	for _, msg := range messages {
		total += EstimateTokens(msg.Content)
	}
	if total > catalog.MaxContextTokens {
		return components.ErrorTokenExceeded.Sprintf(
			fmt.Sprintf("当前输入约 %d tokens，模型 %s 上限为 %d tokens",
				total, modelKey, catalog.MaxContextTokens))
	}
	return nil
}
