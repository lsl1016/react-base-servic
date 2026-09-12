package router

import (
	"context"
	"net/http"
	"time"

	"react-base-service/golib/base"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
)

// registerHealthRoutes 在 engine 根路径挂载健康检查（不走业务前缀与中间件）：
//   - GET /healthz：liveness，进程存活即 200；
//   - GET /readyz ：readiness，探测 MySQL/Redis 依赖，任一失败返回 503 与明细。
//
// python 沙箱与 COS 属可选能力，不纳入 ready 判定（其故障不影响纯对话链路）。
func registerHealthRoutes(engine *gin.Engine) {
	engine.GET("/healthz", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "ok")
	})
	engine.GET("/readyz", readyzHandler)
}

func readyzHandler(ctx *gin.Context) {
	probeCtx, cancel := context.WithTimeout(ctx.Request.Context(), 2*time.Second)
	defer cancel()

	checks := gin.H{
		"mysql": checkMysql(probeCtx),
		"redis": checkRedis(probeCtx),
	}

	status := http.StatusOK
	for _, result := range checks {
		if result != "ok" {
			status = http.StatusServiceUnavailable
			break
		}
	}

	state := "ok"
	if status != http.StatusOK {
		state = "unavailable"
	}
	// 供 RegReadyProbe 注册的业务自定义探针在此统一执行，结果合并进 checks。
	for name, probe := range base.ReadyProbes() {
		if err := probe(probeCtx); err != nil {
			checks[name] = err.Error()
			status = http.StatusServiceUnavailable
			state = "unavailable"
		}
	}

	ctx.JSON(status, gin.H{"status": state, "checks": checks})
}

func checkMysql(ctx context.Context) string {
	if helpers.MysqlClientLLM == nil {
		return "not initialized"
	}
	sqlDB, err := helpers.MysqlClientLLM.DB()
	if err != nil {
		return "no raw db: " + err.Error()
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return "ping failed: " + err.Error()
	}
	return "ok"
}

func checkRedis(ctx context.Context) string {
	if err := helpers.PingRedis(ctx); err != nil {
		return "ping failed: " + err.Error()
	}
	return "ok"
}
