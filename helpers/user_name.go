package helpers

import "github.com/gin-gonic/gin"

const UserNameKey = "user_name"

// SetUserName 将 IPS 登录用户名存入 gin.Context
func SetUserName(ctx *gin.Context, userName string) {
	ctx.Set(UserNameKey, userName)
}

// GetUserName 从 gin.Context 中获取 IPS 登录用户名
func GetUserName(ctx *gin.Context) string {
	if ctx == nil {
		return "system"
	}
	userName, exists := ctx.Get(UserNameKey)
	if !exists {
		return "unknown"
	}
	strUserName, ok := userName.(string)
	if !ok {
		return "unknown"
	}
	return strUserName
}
