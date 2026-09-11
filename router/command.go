package router

import (
	"react-base-service/service/asynctask"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// Commands 注册 CLI 子命令。基座服务默认只有 http 入口；
// go run main.go 默认启动 http 服务，接入方可按需在此追加定时任务子命令。
func Commands(rootCmd *cobra.Command, engine *gin.Engine) {
}

// Tasks 启动随 HTTP 服务运行的后台任务（异步任务状态同步框架，无 Provider 时为空转）。
func Tasks(engine *gin.Engine) {
	asynctask.Start(engine)
}

// StopTasks 停止随 HTTP 服务启动的后台任务。
func StopTasks() {
	asynctask.Stop()
}
