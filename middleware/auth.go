package middleware

import (
	"strings"

	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
)

// AnonymousAuth 免登录鉴权：不做任何身份校验，直接放行。
// 请求携带 X-User-Name 头时以其作为操作人，否则默认 anonymous；
// 下游会话归属校验、playground 白名单等按 userName 运转的逻辑保持不变。
func AnonymousAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userName := strings.TrimSpace(ctx.GetHeader("X-User-Name"))
		if userName == "" {
			userName = "anonymous"
		}
		helpers.SetUserName(ctx, userName)
		ctx.Next()
	}
}
