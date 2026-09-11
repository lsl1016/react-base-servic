package llm

import (
	"context"
	"net/http"
)

// applyTraceHeadersFromContext 与 service/tool/executor.go 中 ExecuteHTTPTool/executeHTTPToolGET 一致：
// 从 context 透传 X-Log-Id、Uber-Trace-Id，便于下游与接入层日志关联。
func applyTraceHeadersFromContext(ctx context.Context, req *http.Request) {
	if logID, _ := ctx.Value("logID").(string); logID != "" {
		req.Header.Set("X-Log-Id", logID)
	}
	if requestID, _ := ctx.Value("requestId").(string); requestID != "" {
		req.Header.Set("Uber-Trace-Id", requestID)
	}
}
