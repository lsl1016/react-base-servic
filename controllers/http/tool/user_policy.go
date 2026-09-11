package tool

import (
	"react-base-service/components"
	"react-base-service/components/params"
	"react-base-service/helpers"
	toolService "react-base-service/service/tool"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// CreateToolUserPolicy godoc
// @Summary 创建工具用户白名单策略
// @Tags Tool
// @Accept json
// @Produce json
// @Param request body params.CreateToolUserPolicyReq true "创建请求"
// @Success 200 {object} params.ToolUserPolicyResp
// @Router /tool/whitelist/create [post]
func CreateToolUserPolicy(ctx *gin.Context) {
	var req params.CreateToolUserPolicyReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.Create] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ParamInvalidf("%v", err))
		return
	}

	operator := helpers.GetUserName(ctx)
	policy, err := toolService.CreateToolUserPolicy(ctx, &req, operator)
	if err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.Create] 创建失败: toolId=%s, userCount=%d, operator=%s, err=%v",
			req.ToolID, len(req.WhiteUserList), operator, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	resp, err := toolService.ToToolUserPolicyResp(policy)
	if err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.Create] 响应转换失败: toolId=%s, err=%v", req.ToolID, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	zlog.Infof(ctx, "[ToolUserPolicy.Create] 创建成功: toolId=%s, userCount=%d, operator=%s",
		resp.ToolID, len(resp.WhiteUserList), operator)
	components.RenderJsonSucc(ctx, resp)
}

// UpdateToolUserPolicy godoc
// @Summary 更新工具用户白名单策略
// @Tags Tool
// @Accept json
// @Produce json
// @Param request body params.UpdateToolUserPolicyReq true "更新请求"
// @Success 200 {object} params.ToolUserPolicyResp
// @Router /tool/whitelist/update [post]
func UpdateToolUserPolicy(ctx *gin.Context) {
	var req params.UpdateToolUserPolicyReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.Update] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ParamInvalidf("%v", err))
		return
	}

	operator := helpers.GetUserName(ctx)
	policy, err := toolService.UpdateToolUserPolicy(ctx, &req, operator)
	if err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.Update] 更新失败: toolId=%s, userCount=%d, operator=%s, err=%v",
			req.ToolID, len(req.WhiteUserList), operator, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	resp, err := toolService.ToToolUserPolicyResp(policy)
	if err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.Update] 响应转换失败: toolId=%s, err=%v", req.ToolID, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	zlog.Infof(ctx, "[ToolUserPolicy.Update] 更新成功: toolId=%s, userCount=%d, operator=%s",
		resp.ToolID, len(resp.WhiteUserList), operator)
	components.RenderJsonSucc(ctx, resp)
}

// DeleteToolUserPolicy godoc
// @Summary 删除工具用户白名单策略
// @Tags Tool
// @Accept json
// @Produce json
// @Param request body params.DeleteToolUserPolicyReq true "删除请求"
// @Success 200 {object} map[string]interface{}
// @Router /tool/whitelist/delete [post]
func DeleteToolUserPolicy(ctx *gin.Context) {
	var req params.DeleteToolUserPolicyReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.Delete] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ParamInvalidf("%v", err))
		return
	}

	operator := helpers.GetUserName(ctx)
	if err := toolService.DeleteToolUserPolicy(ctx, req.ToolID, operator); err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.Delete] 删除失败: toolId=%s, operator=%s, err=%v",
			req.ToolID, operator, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	zlog.Infof(ctx, "[ToolUserPolicy.Delete] 删除成功: toolId=%s, operator=%s", req.ToolID, operator)
	components.RenderJsonSucc(ctx, gin.H{})
}

// GetToolUserPolicyDetail godoc
// @Summary 查询工具用户白名单策略详情
// @Tags Tool
// @Accept json
// @Produce json
// @Param request body params.ToolUserPolicyDetailReq true "详情请求"
// @Success 200 {object} params.ToolUserPolicyResp
// @Router /tool/whitelist/detail [post]
func GetToolUserPolicyDetail(ctx *gin.Context) {
	var req params.ToolUserPolicyDetailReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.Detail] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ParamInvalidf("%v", err))
		return
	}

	policy, err := toolService.GetToolUserPolicy(ctx, req.ToolID)
	if err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.Detail] 查询失败: toolId=%s, err=%v", req.ToolID, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	resp, err := toolService.ToToolUserPolicyResp(policy)
	if err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.Detail] 响应转换失败: toolId=%s, err=%v", req.ToolID, err)
		components.RenderJsonFail(ctx, err)
		return
	}
	components.RenderJsonSucc(ctx, resp)
}

// ListToolUserPolicies godoc
// @Summary 批量查询工具用户白名单策略
// @Tags Tool
// @Accept json
// @Produce json
// @Param request body params.ListToolUserPoliciesReq true "列表请求"
// @Success 200 {array} params.ToolUserPolicyResp
// @Router /tool/whitelist/list [post]
func ListToolUserPolicies(ctx *gin.Context) {
	var req params.ListToolUserPoliciesReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.List] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ParamInvalidf("%v", err))
		return
	}

	policies, err := toolService.ListToolUserPolicies(ctx, req.ToolIDs)
	if err != nil {
		zlog.Errorf(ctx, "[ToolUserPolicy.List] 查询失败: toolCount=%d, err=%v", len(req.ToolIDs), err)
		components.RenderJsonFail(ctx, err)
		return
	}
	resp := make([]params.ToolUserPolicyResp, 0, len(policies))
	for i := range policies {
		item, err := toolService.ToToolUserPolicyResp(&policies[i])
		if err != nil {
			zlog.Errorf(ctx, "[ToolUserPolicy.List] 响应转换失败: toolId=%s, err=%v", policies[i].ToolID, err)
			components.RenderJsonFail(ctx, err)
			return
		}
		resp = append(resp, item)
	}
	components.RenderJsonSucc(ctx, resp)
}
