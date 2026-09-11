package caller

import (
	"react-base-service/components"
	"react-base-service/components/params"
	callerService "react-base-service/service/caller"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// BatchDelete 软删除单个 Caller 及其全部配置资源。
// @Summary      删除Caller及其配置
// @Description  软删除指定的单个Caller，并在同一事务内删除其Skill、系统提示词、Tool、Tool用户策略、API Key等全部关联资源
// @Tags         caller
// @Accept       json
// @Produce      json
// @Param        req  body     params.DeleteCallerReq  true  "Caller删除请求体"
// @Success      200  {object} components.DefaultRenderWithTrace{data=params.DeleteCallerResp}  "删除成功，返回callerKey及各资源删除数量"
// @Failure      400  {object} components.DefaultRenderWithTrace  "参数校验失败或Caller不存在"
// @Router       /caller/batch_delete [post]
func BatchDelete(ctx *gin.Context) {
	var req params.DeleteCallerReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[Caller.BatchDelete] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf(err.Error()))
		return
	}

	resp, err := callerService.BatchDelete(ctx, req.CallerKey)
	if err != nil {
		zlog.Errorf(ctx, "[Caller.BatchDelete] 删除失败: callerKey=%v, err=%v", req.CallerKey, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	components.RenderJsonSucc(ctx, resp)
}
