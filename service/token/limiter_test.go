package token

import (
	"os"
	"testing"
	"unicode/utf8"
)

func TestEstimateTokensBasic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		text string
		want int
	}{
		{name: "empty text", text: "", want: 0},
		{name: "single chinese rune", text: "你", want: 2},
		{name: "ascii word", text: "hello", want: 2},
		// 推导：hello=5×3，全角逗号=default 8，你好=2×10，123=3×3 → 52 units；
		// +10% 冗余向上取整 → (52*11+9)/10=58 → (58+9)/10=6 tokens。
		{name: "mixed text", text: "hello，你好123", want: 6},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := EstimateTokens(tc.text); got != tc.want {
				t.Fatalf("EstimateTokens(%q) = %d, want %d", tc.text, got, tc.want)
			}
		})
	}
}

// TestEstimateTokensFromLocalFile 用于本地手动估算文件 token 数。
func TestEstimateTokensFromLocalFile(t *testing.T) {
	filePath := "testdata/token_estimate_sample.txt"

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read local file %q: %v", filePath, err)
	}

	text := string(data)
	tokens := EstimateTokens(text)
	t.Logf("file=%s bytes=%d runes=%d estimated_tokens=%d",
		filePath, len(data), utf8.RuneCountInString(text), tokens)
}
