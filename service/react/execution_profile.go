package react

import "react-base-service/conf"

// ExecutionProfile 描述当前 Run 的能力开关集合。基座服务只保留 outer 执行域，
// 字段保留用于约束 Runtime 内置工具的暴露范围（如后续需要裁剪某个 meta tool）。
type ExecutionProfile struct {
	AllowTodo           bool
	AllowPlan           bool
	AllowClientTools    bool
	AllowDynamicTools   bool
	AllowUserQuestion   bool
	AllowAsyncTaskTools bool
	AllowSkills         bool
	AllowMemory         bool
	// AllowGraphMemory 控制 graph_memory_search/graph_memory_write（时序图谱记忆）工具；
	// 外层与子 run 跟随 graph_memory.enabled 配置，reflection 等受限执行域恒关闭。
	AllowGraphMemory bool
	// AllowSubagent 控制 delegate_agent（子 Agent 委派）工具；外层 run 跟随 subagent.enabled 配置，
	// reflection 等受限执行域恒关闭。
	AllowSubagent bool
	// AllowAnalysisTools 控制 read_tool_result/inspect_data/python_exec 等分析类内置工具；
	// 主对话默认开启，reflection 等受限执行域关闭。
	AllowAnalysisTools      bool
	InjectAsyncTaskReminder bool
	RestoreOuterHistory     bool
}

func outerExecutionProfile() ExecutionProfile {
	return ExecutionProfile{
		AllowTodo:               true,
		AllowPlan:               conf.CustomConf.LLM.React.AllowPlanEnabled(),
		AllowClientTools:        true,
		AllowDynamicTools:       true,
		AllowUserQuestion:       true,
		AllowAsyncTaskTools:     true,
		AllowSkills:             true,
		AllowMemory:             conf.CustomConf.LLM.React.Memory.MemoryEnabled(),
		AllowGraphMemory:        conf.CustomConf.LLM.React.GraphMemory.GraphMemoryEnabled(),
		AllowSubagent:           conf.CustomConf.LLM.React.SubAgent.SubAgentEnabled(),
		AllowAnalysisTools:      true,
		InjectAsyncTaskReminder: true,
		RestoreOuterHistory:     true,
	}
}

// subAgentExecutionProfile 是 delegate_agent 子 run 的执行档案：与外层对话同等能力，
// 但不注入会话级异步任务提醒（子 run 上下文由委派任务主导，与 session 任务无关）。
func subAgentExecutionProfile() ExecutionProfile {
	profile := outerExecutionProfile()
	profile.InjectAsyncTaskReminder = false
	return profile
}
