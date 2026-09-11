// Package golib 提供服务启动引导（recovery、请求 logID 注入）的最小实现，
// 替代原内部框架，保持 Bootstraps 调用方式不变。
package golib

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"react-base-service/golib/zlog"

	"github.com/gin-gonic/gin"
)

// BootstrapConf 启动配置；HandleRecovery 为业务自定义 panic 恢复处理。
type BootstrapConf struct {
	HandleRecovery func(c *gin.Context, err interface{})
}

// Bootstraps 注册全局 recovery 与请求基础中间件（logID/requestId 注入、访问日志）。
func Bootstraps(engine *gin.Engine, conf BootstrapConf) {
	engine.Use(requestContext())
	engine.Use(accessLog())
	engine.Use(gin.CustomRecovery(func(c *gin.Context, err interface{}) {
		if conf.HandleRecovery != nil {
			conf.HandleRecovery(c, err)
			return
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
}

// accessLog 输出每请求访问日志；被 SetNoLogFlag 标记的请求（如探针）跳过。
func accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if c.GetBool("zlog_no_log") {
			return
		}
		zlog.Infof(c, "[access] %s %s -> %d %s (%s, ip=%s)",
			c.Request.Method, c.Request.URL.RequestURI(), c.Writer.Status(),
			http.StatusText(c.Writer.Status()), time.Since(start).Round(time.Millisecond), c.ClientIP())
	}
}

// requestContext 为每个请求生成 logID / requestId，供日志与下游透传使用。
func requestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("logID") == "" {
			c.Set("logID", newID())
		}
		if c.GetString("requestId") == "" {
			c.Set("requestId", newID())
		}
		c.Next()
	}
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf)
}
