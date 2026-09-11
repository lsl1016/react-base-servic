package react

import (
	"react-base-service/components"
	"react-base-service/components/params"
	reactService "react-base-service/service/react"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// UpdateRunFeedback 更新某一轮(run)的点赞/点踩/问题反馈
// @Summary      更新 ReAct 轮次反馈
// @Description  按 runId 更新当前用户某一轮回答的点赞/点踩(feedback: 1/-1/0)和问题反馈文本(problemFeedback，最长500字符)。feedback 和 problemFeedback 至少传一个。
// @Tags         React
// @Accept       json
// @Produce      json
// @Param        req  body     params.ReactRunFeedbackReq  true  "轮次反馈请求体"
// @Success      200  {object} components.DefaultRenderWithTrace{data=params.ReactRunFeedbackResp}
// @Failure      400  {object} components.DefaultRenderWithTrace
// @Router       /react/run/feedback [post]
func UpdateRunFeedback(ctx *gin.Context) {
	var req params.ReactRunFeedbackReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[React.UpdateRunFeedback] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf(err.Error()))
		return
	}

	resp, err := reactService.UpdateReactRunFeedback(ctx, &req)
	if err != nil {
		zlog.Errorf(ctx, "[React.UpdateRunFeedback] 更新轮次反馈失败: runId=%s, err=%v", req.RunID, err)
		components.RenderJsonFail(ctx, err)
		return
	}

	components.RenderJsonSucc(ctx, resp)
}

// ListSessionFeedback 查询某会话下所有轮次反馈，用于历史回显
// @Summary      ReAct 会话轮次反馈列表
// @Description  返回当前用户某会话下所有轮次(run)的反馈状态，供历史会话回显使用。
// @Tags         React
// @Accept       json
// @Produce      json
// @Param        req  body     params.ReactSessionFeedbackReq  true  "会话反馈请求体"
// @Success      200  {object} components.DefaultRenderWithTrace{data=params.ReactSessionFeedbackResp}
// @Failure      400  {object} components.DefaultRenderWithTrace
// @Router       /react/session/feedback [post]
func ListSessionFeedback(ctx *gin.Context) {
	var req params.ReactSessionFeedbackReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[React.ListSessionFeedback] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf(err.Error()))
		return
	}

	resp, err := reactService.ListReactSessionFeedback(ctx, &req)
	if err != nil {
		zlog.Errorf(ctx, "[React.ListSessionFeedback] 查询会话反馈失败: sessionId=%s, err=%v", req.SessionID, err)
		components.RenderJsonFail(ctx, err)
		return
	}

	components.RenderJsonSucc(ctx, resp)
}
