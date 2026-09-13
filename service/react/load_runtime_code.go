package react

// load_runtime_code（P2-1 服务端代码 Workspace 的业务入口，方案 §4.2.1）：
//
// 模型（主 Agent 或 code-agent）调用后，workspace.Manager 按静态解析表把目标服务代码
// 落到本 run 专属 git worktree，并动态挂载只读 repo MCP——工具名前缀 ws_<service>_*
//（list_files / read_file / search_code / find_symbol / get_repo_map），
// 后续按普通业务工具两段式使用（get_tool 加载 → execute_tool 执行），引擎零特殊分支。
// run 终态时统一 ReleaseRun（run() 与 delegate 收尾路径都挂了幂等清理）。

import (
	"encoding/json"

	llm "react-base-service/api/llm"
	"react-base-service/service/workspace"
)

func loadRuntimeCodeDefinition() llm.ToolDefinition {
	return objectTool(metaToolLoadRuntimeCode,
		"把目标服务的线上代码加载到本次运行的只读工作区，并挂载代码检索工具（前缀 ws_<service>_：文件列表/读取/文本检索/符号查找/仓库地图）。加载完成后用 get_tool 加载具体检索工具、execute_tool 执行。适合定位报错代码、核对线上逻辑、查看近期变更。",
		map[string]interface{}{
			"service": stringSchema("目标服务名（须在 workspace.resolvers 白名单内，如 react-base-service）"),
			"env":     stringSchema("环境标识（如 prod/test，可选；未配置用默认 ref）"),
		})
}

func (s *reactEngineState) executeLoadRuntimeCode(input json.RawMessage) (string, bool, error) {
	var req struct {
		Service string `json:"service"`
		Env     string `json:"env"`
	}
	_ = json.Unmarshal(input, &req)
	if req.Service == "" {
		return "load_runtime_code requires service", true, nil
	}
	alloc, err := workspace.Default().Load(s.ctx, s.runID, s.req.payload.CallerKey, req.Service, req.Env)
	if err != nil {
		return err.Error(), true, nil
	}
	result, _ := json.Marshal(map[string]string{
		"service": alloc.Service,
		"env":     alloc.Env,
		"commit":  alloc.Commit,
		"ref":     alloc.Ref,
		"tools":   "ws_" + alloc.Service + "_list_files / ws_" + alloc.Service + "_read_file / ws_" + alloc.Service + "_search_code / ws_" + alloc.Service + "_find_symbol / ws_" + alloc.Service + "_get_repo_map",
		"note":    "代码已锁定到该 commit 的只读工作区；先 get_tool 加载上述工具再 execute_tool 调用；run 结束自动释放",
	})
	return string(result), false, nil
}
