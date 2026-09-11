package components

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	RequestSourceDatamapKnowledgeDingding = "datamap-knowledge-dingding"

	CallerRuntimeContextKey = "caller-runtime-context"
)

type CallerRuntimeContext struct {
	CallerKey     string `json:"callerKey"`
	RequestSource string `json:"requestSource,omitempty"`
	UserName      string `json:"userName,omitempty"`
	SessionID     string `json:"sessionId,omitempty"`
	UpdateMode    string `json:"updateMode,omitempty"`
}

// NormalizeCallerRuntimeContext 归一化调用方运行时上下文，随 run 生命周期注入 context 与 HTTP 工具请求。
func NormalizeCallerRuntimeContext(ctx *gin.Context, callerKey, requestSource, userName, sessionID, updateMode string) (CallerRuntimeContext, error) {
	return CallerRuntimeContext{
		CallerKey:     strings.TrimSpace(callerKey),
		RequestSource: strings.TrimSpace(requestSource),
		UserName:      strings.TrimSpace(userName),
		SessionID:     strings.TrimSpace(sessionID),
		UpdateMode:    strings.TrimSpace(updateMode),
	}, nil
}

func ContextWithCallerRuntime(parent context.Context, runtimeCtx CallerRuntimeContext) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithValue(parent, CallerRuntimeContextKey, runtimeCtx)
}

func CallerRuntimeFromContext(ctx context.Context) (CallerRuntimeContext, bool) {
	if ctx == nil {
		return CallerRuntimeContext{}, false
	}
	if v, ok := ctx.Value(CallerRuntimeContextKey).(CallerRuntimeContext); ok {
		return v, true
	}
	callerKey, _ := ctx.Value("skill-caller-key").(string)
	requestSource, _ := ctx.Value("skill-request-source").(string)
	updateMode, _ := ctx.Value("x-skill-update-mode").(string)
	sessionID, _ := ctx.Value("x-skill-session-id").(string)
	userName, _ := ctx.Value("x-skill-user-name").(string)
	if strings.TrimSpace(callerKey) == "" && strings.TrimSpace(requestSource) == "" {
		return CallerRuntimeContext{}, false
	}
	return CallerRuntimeContext{
		CallerKey:     strings.TrimSpace(callerKey),
		RequestSource: strings.TrimSpace(requestSource),
		UserName:      strings.TrimSpace(userName),
		SessionID:     strings.TrimSpace(sessionID),
		UpdateMode:    strings.TrimSpace(updateMode),
	}, true
}

// ApplyHTTPToolHeaders 将调用方运行时上下文透传给后端 HTTP Business Tool，保持调用态一致。
func ApplyHTTPToolHeaders(ctx context.Context, headers http.Header) {
	if headers == nil {
		return
	}
	runtimeCtx, ok := CallerRuntimeFromContext(ctx)
	if !ok {
		return
	}
	if runtimeCtx.UpdateMode != "" {
		headers.Set("X-Update-Mode", runtimeCtx.UpdateMode)
	}
	if runtimeCtx.SessionID != "" {
		headers.Set("X-Session-Id", runtimeCtx.SessionID)
	}
	if runtimeCtx.UserName != "" {
		headers.Set("X-User-Name", runtimeCtx.UserName)
	}
}
