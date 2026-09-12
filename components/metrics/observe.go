package metrics

import (
	"time"
)

// ObserveToolCall 记录一次工具调用的计数与耗时（status: success/error）。
func ObserveToolCall(tool string, start time.Time, isError bool) {
	status := "success"
	if isError {
		status = "error"
	}
	ToolCallsTotal.WithLabelValues(tool, status).Inc()
	ToolDuration.WithLabelValues(tool).Observe(time.Since(start).Seconds())
}

// ObserveHTTPRequest 记录一次 HTTP 请求的计数与耗时。
func ObserveHTTPRequest(method, path string, code int, start time.Time) {
	codeLabel := httpStatusClass(code)
	HTTPRequestsTotal.WithLabelValues(method, path, codeLabel).Inc()
	HTTPRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
}

// httpStatusClass 用 2xx/3xx/4xx/5xx 分组，避免状态码标签基数过高。
func httpStatusClass(code int) string {
	switch {
	case code < 200 || code >= 600:
		return "unknown"
	case code < 300:
		return "2xx"
	case code < 400:
		return "3xx"
	case code < 500:
		return "4xx"
	default:
		return "5xx"
	}
}
