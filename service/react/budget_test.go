package react

import "testing"

// TestTokenBudgetExceeded 预算判定纯函数（P3 子代理预算）：
// 0/负 = 不限；递归口径含委派孙代理的 delegated_*；恰好等于上限不视为超限（> 才超）。
func TestTokenBudgetExceeded(t *testing.T) {
	if tokenBudgetExceeded(0, 1_000_000, 1_000_000, 1_000_000, 1_000_000) {
		t.Fatal("budget=0 应不限")
	}
	if tokenBudgetExceeded(-1, 100, 100, 0, 0) {
		t.Fatal("负数预算按不限处理")
	}
	if tokenBudgetExceeded(1000, 400, 400, 100, 100) {
		t.Fatal("恰好等于上限不应超限")
	}
	if !tokenBudgetExceeded(1000, 400, 400, 150, 100) {
		t.Fatal("递归口径（含 delegated）超限应命中")
	}
	if tokenBudgetExceeded(1000, 100, 100, 0, 0) {
		t.Fatal("未超限不应命中")
	}
}
