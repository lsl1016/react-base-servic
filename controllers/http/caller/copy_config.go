package caller

import (
	"react-base-service/components"
	"react-base-service/components/params"
	"react-base-service/helpers"
	callerService "react-base-service/service/caller"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// CopyConfig 复制 Caller 及其全部配置资源。
// @Summary      复制Caller配置
// @Description  以源Caller为模板创建新Caller，并复制其Skill、系统提示词、Tool、Tool用户策略、API Key等全部配置资源，整个复制在单事务内完成
// @Tags         caller
// @Accept       json
// @Produce      json
// @Param        req  body     params.CopyCallerConfigReq  true  "Caller配置复制请求体"
// @Success      200  {object} components.DefaultRenderWithTrace{data=params.CopyCallerConfigResp}  "复制成功，返回目标callerKey及各资源复制数量"
// @Failure      400  {object} components.DefaultRenderWithTrace  "参数校验失败、源Caller不存在或目标callerKey已存在"
// @Router       /caller/copy_config [post]
func CopyConfig(ctx *gin.Context) {
	var req params.CopyCallerConfigReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[Caller.CopyConfig] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf(err.Error()))
		return
	}

	resp, err := callerService.CopyConfig(ctx, &req, helpers.GetUserName(ctx))
	if err != nil {
		zlog.Errorf(ctx, "[Caller.CopyConfig] 复制失败: sourceCallerKey=%s, targetCallerKey=%s, err=%v",
			req.SourceCallerKey, req.TargetCaller.CallerKey, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	components.RenderJsonSucc(ctx, resp)
}
