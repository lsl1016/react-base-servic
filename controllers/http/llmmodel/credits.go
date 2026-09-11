package llmmodel

import (
	"react-base-service/components"
	"react-base-service/components/params"
	"react-base-service/helpers"
	model "react-base-service/models/llm"
	creditsService "react-base-service/service/credits"
	llmmodelService "react-base-service/service/llmmodel"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// AdjustCredits 手动调整赠送积分（仅白名单用户）
// @Summary      调整赠送积分
// @Description  管理员手动调整指定用户在指定平台默认模型上的赠送积分（正数增加，负数减少），仅白名单用户可操作；仅支持平台默认模型（isPlatformDefault=1），非平台默认模型无积分概念
// @Tags         llmmodel
// @Accept       json
// @Produce      json
// @Param        req  body     params.AdjustCreditsReq  true  "积分调整请求（userName 目标用户英文名，modelHash 平台默认模型的hash，delta 调整量正负均可）"
// @Success      200  {object} components.DefaultRenderWithTrace  "调整成功"
// @Failure      400  {object} components.DefaultRenderWithTrace  "参数校验失败、权限不足或模型不存在"
// @Failure      500  {object} components.DefaultRenderWithTrace  "服务内部错误"
// @Router       /model/credits/adjust [post]
func AdjustCredits(ctx *gin.Context) {
	var req params.AdjustCreditsReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[Credits.Adjust] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf(err.Error()))
		return
	}

	userName := helpers.GetUserName(ctx)

	// 仅白名单用户可操作
	if !llmmodelService.IsWhitelisted(userName) {
		zlog.Warnf(ctx, "[Credits.Adjust] 非白名单用户尝试调整积分: userName=%s", userName)
		components.RenderJsonFail(ctx, components.ErrorUserModelNoPermission)
		return
	}

	// 校验模型存在且为平台默认模型（仅平台默认模型有积分概念）
	targetModel, err := model.GetUserModelByHash(ctx, req.ModelHash)
	if err != nil {
		zlog.Errorf(ctx, "[Credits.Adjust] 查询模型失败: modelHash=%s, err=%v", req.ModelHash, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	if targetModel == nil {
		components.RenderJsonFail(ctx, components.ErrorUserModelNotFound.Sprintf(req.ModelHash))
		return
	}
	if targetModel.IsPlatformDefault != 1 {
		components.RenderJsonFail(ctx, components.ErrorModelNotPlatformDefault)
		return
	}

	if err := creditsService.AdjustCredits(ctx, req.UserName, req.ModelHash, req.Delta); err != nil {
		zlog.Errorf(ctx, "[Credits.Adjust] 调整失败: targetUser=%s, modelHash=%s, delta=%d, err=%v",
			req.UserName, req.ModelHash, req.Delta, err)
		components.RenderJsonFail(ctx, err)
		return
	}

	zlog.Infof(ctx, "[Credits.Adjust] 调整成功: operator=%s, targetUser=%s, modelHash=%s, delta=%d",
		userName, req.UserName, req.ModelHash, req.Delta)

	components.RenderJsonSucc(ctx, gin.H{})
}
