package react

import (
	"context"
	"testing"
)

func TestClassifyToolInterruption(t *testing.T) {
	t.Run("用户主动取消", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(ErrReactRunCancelled)
		status, _, ok := classifyToolInterruption(ctx, context.Canceled)
		if !ok || status != toolExecutionStatusCancelled {
			t.Fatalf("expected cancelled, got status=%q ok=%v", status, ok)
		}
	})

	t.Run("客户端断连保持错误", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(ErrReactClientDisconnected)
		status, _, ok := classifyToolInterruption(ctx, context.Canceled)
		if !ok || status != toolExecutionStatusError {
			t.Fatalf("expected error, got status=%q ok=%v", status, ok)
		}
	})

	t.Run("普通工具错误不属于中断", func(t *testing.T) {
		status, _, ok := classifyToolInterruption(context.Background(), context.DeadlineExceeded)
		if ok || status != "" {
			t.Fatalf("expected ordinary error, got status=%q ok=%v", status, ok)
		}
	})
}
