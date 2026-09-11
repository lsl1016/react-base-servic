package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"

	"react-base-service/golib/zlog"
)

type prefixDebugContextKey struct{}

type PrefixDebugInfo struct {
	Key   string
	Step  int
	RunID string
}

func WithPrefixDebug(ctx context.Context, key string, step int) context.Context {
	return WithPrefixDebugRun(ctx, key, step, "")
}

func WithPrefixDebugRun(ctx context.Context, key string, step int, runID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, prefixDebugContextKey{}, PrefixDebugInfo{
		Key:   key,
		Step:  step,
		RunID: runID,
	})
}

func ClearPrefixDebug(key string) {
	if key == "" {
		return
	}
	llmPrefixDebugStore.Delete(key)
}

func prefixDebugInfoFromContext(ctx context.Context) (PrefixDebugInfo, bool) {
	if ctx == nil {
		return PrefixDebugInfo{}, false
	}
	info, ok := ctx.Value(prefixDebugContextKey{}).(PrefixDebugInfo)
	return info, ok && info.Key != ""
}

type llmPrefixSnapshot struct {
	Step  int
	RunID string
	Body  []byte
	Hash  string
}

var llmPrefixDebugStore sync.Map

func logLLMPrefixDebug(ctx context.Context, provider string, body []byte) {
	info, ok := prefixDebugInfoFromContext(ctx)
	if !ok {
		return
	}

	currentHash := sha256Hex(body)
	prevValue, hasPrev := llmPrefixDebugStore.Load(info.Key)
	llmPrefixDebugStore.Store(info.Key, llmPrefixSnapshot{
		Step:  info.Step,
		RunID: info.RunID,
		Body:  append([]byte(nil), body...),
		Hash:  currentHash,
	})

	if !hasPrev {
		logLLMPrefixDebugf(ctx, "[%s.PrefixDebug] key=%s run_id=%s step=%d current_bytes=%d current_sha256=%s first_request=true",
			provider, info.Key, info.RunID, info.Step, len(body), currentHash)
		return
	}

	prev, ok := prevValue.(llmPrefixSnapshot)
	if !ok {
		return
	}
	common := commonPrefixLen(prev.Body, body)
	ratio := 0.0
	if len(body) > 0 {
		ratio = float64(common) / float64(len(body))
	}
	prevRatio := 0.0
	if len(prev.Body) > 0 {
		prevRatio = float64(common) / float64(len(prev.Body))
	}
	if prev.RunID != "" && info.RunID != "" && prev.RunID != info.RunID {
		logLLMPrefixDebugf(ctx, "[%s.PrefixDebug.CrossRun] key=%s prev_run_id=%s run_id=%s prev_step=%d step=%d prev_bytes=%d current_bytes=%d common_prefix_bytes=%d current_prefix_ratio=%.4f prev_prefix_ratio=%.4f",
			provider,
			info.Key,
			prev.RunID,
			info.RunID,
			prev.Step,
			info.Step,
			len(prev.Body),
			len(body),
			common,
			ratio,
			prevRatio,
		)
	}
	logLLMPrefixDebugf(ctx, "[%s.PrefixDebug] key=%s prev_run_id=%s run_id=%s prev_step=%d step=%d prev_bytes=%d current_bytes=%d common_prefix_bytes=%d current_prefix_ratio=%.4f prev_prefix_ratio=%.4f prev_is_current_prefix=%t prev_sha256=%s current_sha256=%s diff_current_preview=%q",
		provider,
		info.Key,
		prev.RunID,
		info.RunID,
		prev.Step,
		info.Step,
		len(prev.Body),
		len(body),
		common,
		ratio,
		prevRatio,
		common == len(prev.Body) && len(prev.Body) <= len(body),
		prev.Hash,
		currentHash,
		previewAround(body, common),
	)
}

func logLLMPrefixDebugf(ctx context.Context, format string, args ...interface{}) {
	if ginCtx := streamLogContext(ctx); ginCtx != nil {
		zlog.Infof(ginCtx, format, args...)
		return
	}
	log.Printf("%s%s", streamLogPrefix(ctx), fmt.Sprintf(format, args...))
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func commonPrefixLen(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func previewAround(body []byte, offset int) string {
	if offset < 0 {
		offset = 0
	}
	start := offset - 80
	if start < 0 {
		start = 0
	}
	end := offset + 160
	if end > len(body) {
		end = len(body)
	}
	return string(body[start:end])
}
