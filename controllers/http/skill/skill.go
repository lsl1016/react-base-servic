package skill

import (
	"react-base-service/components"
	"react-base-service/components/params"
	"react-base-service/helpers"
	skillService "react-base-service/service/skill"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// CreateSkill 创建 Skill
// @Summary      创建 Skill
// @Description  创建指定 caller 下的 Skill 配置。
// @Tags         skill
// @Accept       json
// @Produce      json
// @Param        req  body     params.CreateSkillReq  true  "创建 Skill 请求体"
// @Success      200  {object} components.DefaultRenderWithTrace{data=params.SkillResp}  "成功返回 Skill 详情"
// @Failure      400  {object} components.DefaultRenderWithTrace  "参数校验失败或创建失败"
// @Router       /skill/create [post]
func CreateSkill(ctx *gin.Context) {
	var req params.CreateSkillReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[Skill.Create] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf(err.Error()))
		return
	}

	userName := helpers.GetUserName(ctx)
	s, err := skillService.CreateSkill(ctx, &req, userName)
	if err != nil {
		zlog.Errorf(ctx, "[Skill.Create] 创建失败: %v", err)
		components.RenderJsonFail(ctx, err)
		return
	}

	components.RenderJsonSucc(ctx, skillService.ToSkillResp(s))
}

// UpdateSkill 更新 Skill
// @Summary      更新 Skill
// @Description  根据 skillId 更新 Skill 的配置内容或状态。
// @Tags         skill
// @Accept       json
// @Produce      json
// @Param        req  body     params.UpdateSkillReq  true  "更新 Skill 请求体"
// @Success      200  {object} components.DefaultRenderWithTrace{data=params.SkillResp}  "更新成功并返回 Skill 详情"
// @Failure      400  {object} components.DefaultRenderWithTrace  "参数校验失败或更新失败"
// @Router       /skill/update [post]
func UpdateSkill(ctx *gin.Context) {
	var req params.UpdateSkillReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[Skill.Update] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf(err.Error()))
		return
	}

	s, err := skillService.UpdateSkill(ctx, &req)
	if err != nil {
		zlog.Errorf(ctx, "[Skill.Update] 更新失败: skillId=%s, err=%v", req.SkillID, err)
		components.RenderJsonFail(ctx, err)
		return
	}

	components.RenderJsonSucc(ctx, skillService.ToSkillResp(s))
}

// DeleteSkill 删除 Skill
// @Summary      删除 Skill
// @Description  根据 skillId 删除指定 Skill。
// @Tags         skill
// @Accept       json
// @Produce      json
// @Param        req  body     params.DeleteSkillReq  true  "删除 Skill 请求体"
// @Success      200  {object} components.DefaultRenderWithTrace{data=params.SkillResp}  "删除成功并返回 Skill 详情"
// @Failure      400  {object} components.DefaultRenderWithTrace  "参数校验失败或删除失败"
// @Router       /skill/delete [post]
func DeleteSkill(ctx *gin.Context) {
	var req params.DeleteSkillReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[Skill.Delete] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf(err.Error()))
		return
	}

	s, err := skillService.DeleteSkill(ctx, req.SkillID)
	if err != nil {
		zlog.Errorf(ctx, "[Skill.Delete] 删除失败: skillId=%s, err=%v", req.SkillID, err)
		components.RenderJsonFail(ctx, err)
		return
	}

	components.RenderJsonSucc(ctx, skillService.ToSkillResp(s))
}

// ListSkills 查询 Skill 列表
// @Summary      查询 Skill 列表
// @Description  根据 callerKey 查询该调用方下的 Skill 列表；传入 routeValues 时按 routeValues 层级前缀返回对应列表，未传时兼容旧逻辑返回 callerKey 下全部列表。
// @Tags         skill
// @Accept       json
// @Produce      json
// @Param        req  body     params.ListSkillsReq  true  "查询 Skill 列表请求体"
// @Success      200  {object} components.DefaultRenderWithTrace{data=[]params.SkillResp}  "成功返回 Skill 列表"
// @Failure      400  {object} components.DefaultRenderWithTrace  "参数校验失败或查询失败"
// @Router       /skill/list [post]
func ListSkills(ctx *gin.Context) {
	var req params.ListSkillsReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[Skill.List] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf(err.Error()))
		return
	}

	skills, err := skillService.ListByCallerAndRoute(ctx, req.CallerKey, req.RouteValues)
	if err != nil {
		zlog.Errorf(ctx, "[Skill.List] 查询失败: %v", err)
		components.RenderJsonFail(ctx, err)
		return
	}

	resp := make([]params.SkillResp, 0, len(skills))
	for i := range skills {
		resp = append(resp, skillService.ToSkillResp(&skills[i]))
	}
	components.RenderJsonSucc(ctx, resp)
}

// GetSkillDetail 查询 Skill 详情
// @Summary      查询 Skill 详情
// @Description  根据 skillId 查询单个 Skill 的详细配置。
// @Tags         skill
// @Accept       json
// @Produce      json
// @Param        req  body     params.SkillDetailReq  true  "查询 Skill 详情请求体"
// @Success      200  {object} components.DefaultRenderWithTrace{data=params.SkillResp}  "成功返回 Skill 详情"
// @Failure      400  {object} components.DefaultRenderWithTrace  "参数校验失败或查询失败"
// @Router       /skill/detail [post]
func GetSkillDetail(ctx *gin.Context) {
	var req params.SkillDetailReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		zlog.Errorf(ctx, "[Skill.Detail] 请求参数绑定失败: %v", err)
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf(err.Error()))
		return
	}

	s, err := skillService.GetDetail(ctx, req.SkillID)
	if err != nil {
		zlog.Errorf(ctx, "[Skill.Detail] 查询失败: skillId=%s, err=%v", req.SkillID, err)
		components.RenderJsonFail(ctx, err)
		return
	}

	components.RenderJsonSucc(ctx, skillService.ToSkillResp(s))
}
