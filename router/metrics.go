package router

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// registerMetricsRoute 在根路径挂载 Prometheus 抓取端点（默认注册表）。
// 挂主端口（零配置）；如后续需独立 metrics 端口再拆分。
func registerMetricsRoute(engine *gin.Engine) {
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
