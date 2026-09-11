package llmmodel

import (
	"react-base-service/components"
	llmmodelService "react-base-service/service/llmmodel"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// GetWhitelist 获取模型管理白名单
// @Summary      获取模型管理白名单
// @Description  返回白名单用户列表及当前用户是否在白名单内，前端据此做权限管控
// @Tags         llmmodel
// @Produce      json
// @Success      200  {object}  components.DefaultRenderWithTrace{data=params.WhitelistResp}  "成功"
// @Failure      500  {object}  components.DefaultRenderWithTrace
// @Router       /model/whitelist [get]
func GetWhitelist(ctx *gin.Context) {
	zlog.Debugf(ctx, "[GetWhitelist] 获取模型管理白名单")

	resp := llmmodelService.GetWhitelist()
	components.RenderJsonSucc(ctx, resp)
}
