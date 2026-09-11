package react

import (
	"react-base-service/components"
	"react-base-service/components/params"
	reactService "react-base-service/service/react"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// ListAsyncTasks 获取当前会话第三方已终态但本地尚未处理的异步任务。
// @Summary ReAct 会话待处理异步任务列表
// @Description 游标分页返回指定会话的待处理终态任务，并返回是否仍有 Provider 跟踪中的任务
// @Tags React
// @Accept json
// @Produce json
// @Param req body params.ReactAsyncTaskListReq true "异步任务列表请求体"
// @Success 200 {object} components.DefaultRenderWithTrace{data=params.ReactAsyncTaskListResp}
// @Failure 400 {object} components.DefaultRenderWithTrace
// @Router /react/async_task/list [post]
func ListAsyncTasks(ctx *gin.Context) {
	var req params.ReactAsyncTaskListReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[React.ListAsyncTasks] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf(err.Error()))
		return
	}
	resp, err := reactService.ListSessionAsyncTasks(ctx, req)
	if err != nil {
		zlog.Errorf(ctx, "[React.ListAsyncTasks] 查询失败: sessionId=%s err=%v", req.SessionID, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	components.RenderJsonSucc(ctx, resp)
}
