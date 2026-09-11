// @title           react-base-service API
// @version         1.0
// @description     ReAct Agent 基座服务：提供 ReAct 运行时、会话管理与回放、工具/Skill/系统提示词注册管理等能力
// @host            localhost:8080
// @BasePath        /react-base-service
package main

import (
	"react-base-service/components"
	"react-base-service/conf"
	"react-base-service/helpers"
	"react-base-service/router"

	"react-base-service/golib"
	"react-base-service/golib/base"
	"react-base-service/golib/server/http"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

func main() {
	// gin
	engine := gin.New()

	// 初始化基础配置
	helpers.PreInit()
	defer helpers.Clear()

	golib.Bootstraps(engine, golib.BootstrapConf{
		// 业务自定义recover handler
		HandleRecovery: func(c *gin.Context, err interface{}) {
			base.RenderJsonAbort(c, components.ErrorSystemError)
		},
	})

	var rootCmd = &cobra.Command{
		Use:   "goweb",
		Short: "react-base-service application",
		Run: func(cmd *cobra.Command, args []string) {
			httpServer(engine)
		},
	}

	// 加载支持的子命令行（基座服务默认无子命令）
	router.Commands(rootCmd, engine)

	if err := rootCmd.Execute(); err != nil {
		panic(err.Error())
	}
}

func httpServer(engine *gin.Engine) {
	// web 服务所需资源初始化
	helpers.InitResource(engine)
	defer helpers.Release()

	// 初始化http服务路由
	router.Http(engine)

	// 启动后台任务（异步任务状态同步框架，无 Provider 时为空转）
	router.Tasks(engine)
	defer router.StopTasks()

	// 启动web server
	if err := http.Start(engine, conf.BasicConf.Server); err != nil {
		panic(err.Error())
	}
}
