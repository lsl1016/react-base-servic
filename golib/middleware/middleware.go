// Package middleware 提供框架级 gin 中间件的最小实现，替代原内部框架 middleware 包。
package middleware

import (
	"react-base-service/golib/zlog"

	"github.com/gin-gonic/gin"
)

// AddField 为请求级日志附加字段（本地实现为空操作，仅保持调用兼容）。
func AddField(_ ...zlog.Field) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
