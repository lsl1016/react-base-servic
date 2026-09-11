package helpers

import (
	"react-base-service/conf"

	"react-base-service/golib/env"
	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// 基础资源（必须）
func PreInit() {
	// 用于日志中展示模块的名字，开发环境需要手动指定，容器中无需手动指定
	env.SetAppName("react-base-service")

	// 配置加载
	conf.InitConf()

	// 日志初始化
	zlog.InitLog(conf.BasicConf.Log)

	// 敏感field处理
	// zlog.RegisterHookField(maskSensitiveField)

	// 如果对外输出的 errNo/errMsg 与框架中默认定义不同，可在此修改json tag
	// initRender()
}

func maskSensitiveField(fields []zlog.Field) {
	for idx, field := range fields {
		if field.Key == "cell" {
			fields[idx].String = "***"
		}
	}
}

func Clear() {
	// 服务结束时的清理工作，对应 Init() 初始化的资源
	zlog.CloseLogger()
}

// web服务启动所需init的资源
func InitResource(engine *gin.Engine) {
	// 初始化全局变量
	InitMysql()
	InitRedis()
	InitCos()
}

// 任务启动所需init的资源
func InitResourceForCron(engine *gin.Engine) {
	InitMysql()
}

func Release() {
}
