// Package metrics 集中注册 Prometheus 指标，业务代码仅调用打点函数。
// 暴露端点：GET /metrics（主端口根路径，见 router/metrics.go）。
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// WSConnections 当前 WebSocket 连接数。
	WSConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "react_ws_connections",
		Help: "Current react websocket connections.",
	})

	// RunsActive 当前活跃 run 数。
	RunsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "react_runs_active",
		Help: "Current active react runs.",
	})

	// RunsTotal run 终态计数（status: finished/error/cancelled/expired）。
	RunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "react_runs_total",
		Help: "Total react runs by final status.",
	}, []string{"status"})

	// SessionsCreatedTotal 新建会话计数。
	SessionsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "react_sessions_created_total",
		Help: "Total react sessions created.",
	})

	// ModelRoundDuration 模型单轮耗时分布（model: 平台/版本）。
	ModelRoundDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "react_model_round_duration_seconds",
		Help:    "React model round duration in seconds.",
		Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30, 60, 120},
	}, []string{"model"})

	// ModelFailoversTotal 模型互备切换次数。
	ModelFailoversTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "react_model_failovers_total",
		Help: "Total react model failover switches.",
	})

	// ToolCallsTotal 工具调用计数（tool, status: success/error/cancelled）。
	ToolCallsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "react_tool_calls_total",
		Help: "Total react tool calls by tool and status.",
	}, []string{"tool", "status"})

	// ToolDuration 工具执行耗时分布。
	ToolDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "react_tool_duration_seconds",
		Help:    "React tool execution duration in seconds.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
	}, []string{"tool"})

	// PythonExecTimeoutsTotal python_exec 沙箱超时次数。
	PythonExecTimeoutsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "react_python_exec_timeouts_total",
		Help: "Total python_exec sandbox timeouts.",
	})

	// HTTPRequestsTotal HTTP 请求计数（method, path, code）。
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by method, path and status code.",
	}, []string{"method", "path", "code"})

	// HTTPRequestDuration HTTP 请求耗时分布。
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"method", "path"})
)
