package llm

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

const (
	StreamTerminationCompleted              = "completed"
	StreamTerminationCancelled              = "cancelled"
	StreamTerminationUpstreamEOFWithoutDone = "upstream_eof_without_done"
	StreamTerminationUpstreamParseError     = "upstream_parse_error"
	StreamTerminationUpstreamReadError      = "upstream_read_error"
	StreamTerminationUpstreamErrorEvent     = "upstream_error_event"
)

type streamFinalState struct {
	provider          string
	terminationReason string
	receivedDone      bool
	finishReason      string
	scannerErr        error
	chunkCount        int
	inputTokens       int
	outputTokens      int
	cacheReadTokens   int
	cacheCreateTokens int
	lastChunkAt       time.Time
}

func isAcceptedFinishReason(reason string) bool {
	switch normalizeFinishReason(reason) {
	case "stop", "end_turn":
		return true
	default:
		return false
	}
}

func isToolCallFinishReason(reason string) bool {
	switch normalizeFinishReason(reason) {
	case "tool_calls", "toolcalls":
		return true
	default:
		return false
	}
}

func normalizeFinishReason(reason string) string {
	return strings.ToLower(strings.TrimSpace(reason))
}

func finishReasonTerminationReason(reason string) string {
	normalized := normalizeFinishReason(reason)
	if normalized == "" {
		return StreamTerminationUpstreamEOFWithoutDone
	}

	var sb strings.Builder
	for _, r := range normalized {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	return fmt.Sprintf("upstream_finish_reason_%s", strings.Trim(sb.String(), "_"))
}

func logStreamParseError(ctx context.Context, provider string, chunkIndex int, data string, err error) {
	if ginCtx := streamLogContext(ctx); ginCtx != nil {
		zlog.Errorf(ginCtx,
			"[%s.ChatStream] invalid upstream chunk: chunk_index=%d data_len=%d data_preview=%q err=%v",
			provider, chunkIndex, len(data), trimLogPreview(data), err)
		return
	}
	log.Printf("%s[%s.ChatStream] invalid upstream chunk: chunk_index=%d data_len=%d data_preview=%q err=%v",
		streamLogPrefix(ctx), provider, chunkIndex, len(data), trimLogPreview(data), err)
}

func logStreamFinal(ctx context.Context, state streamFinalState) {
	lastChunkAt := ""
	if !state.lastChunkAt.IsZero() {
		lastChunkAt = state.lastChunkAt.Format(time.RFC3339Nano)
	}

	if ginCtx := streamLogContext(ctx); ginCtx != nil {
		zlog.Infof(ginCtx,
			"[%s.ChatStream] stream finished: termination_reason=%s received_done=%t finish_reason=%q scanner_err=%v chunk_count=%d input_tokens=%d output_tokens=%d cache_read_tokens=%d cache_create_tokens=%d last_chunk_at=%s",
			state.provider,
			state.terminationReason,
			state.receivedDone,
			state.finishReason,
			state.scannerErr,
			state.chunkCount,
			state.inputTokens,
			state.outputTokens,
			state.cacheReadTokens,
			state.cacheCreateTokens,
			lastChunkAt,
		)
		return
	}
	log.Printf("%s[%s.ChatStream] stream finished: termination_reason=%s received_done=%t finish_reason=%q scanner_err=%v chunk_count=%d input_tokens=%d output_tokens=%d cache_read_tokens=%d cache_create_tokens=%d last_chunk_at=%s",
		streamLogPrefix(ctx),
		state.provider,
		state.terminationReason,
		state.receivedDone,
		state.finishReason,
		state.scannerErr,
		state.chunkCount,
		state.inputTokens,
		state.outputTokens,
		state.cacheReadTokens,
		state.cacheCreateTokens,
		lastChunkAt,
	)
}

func trimLogPreview(data string) string {
	const limit = 240
	if len(data) <= limit {
		return data
	}
	return data[:limit] + "..."
}

func streamLogContext(ctx context.Context) *gin.Context {
	ginCtx, _ := ctx.(*gin.Context)
	return ginCtx
}

func streamLogPrefix(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value("requestId").(string)
	logID, _ := ctx.Value("logID").(string)
	if requestID == "" && logID == "" {
		return ""
	}
	return fmt.Sprintf("[request_id=%s log_id=%s] ", requestID, logID)
}
