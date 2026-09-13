package react

import (
	"react-base-service/components"
	"react-base-service/service/workspace"

	"github.com/gin-gonic/gin"
)

// ListActiveWorkspaces 查询活跃代码工作区
// @Summary      查询活跃代码工作区
// @Description  返回当前进程内全部活跃的 run 代码工作区（worktree）清单：runId、service@ref(commit)、路径、挂载工具前缀与时间。run 终态即释放，空列表为正常态。
// @Tags         React
// @Produce      json
// @Success      200  {object} components.DefaultRenderWithTrace{data=[]workspace.ActiveAllocation}  "活跃清单"
// @Router       /workspace/active [post]
func ListActiveWorkspaces(ctx *gin.Context) {
	manager := workspace.Default()
	if manager == nil {
		components.RenderJsonSucc(ctx, []workspace.ActiveAllocation{})
		return
	}
	components.RenderJsonSucc(ctx, manager.ActiveSnapshot())
}
