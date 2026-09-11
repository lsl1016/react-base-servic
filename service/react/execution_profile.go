package react

import "react-base-service/conf"

// ExecutionProfile 描述当前 Run 的能力开关集合。基座服务只保留 outer 执行域，
// 字段保留用于约束 Runtime 内置工具的暴露范围（如后续需要裁剪某个 meta tool）。
type ExecutionProfile struct {
	AllowTodo               bool
	AllowPlan               bool
	AllowClientTools        bool
	AllowDynamicTools       bool
	AllowUserQuestion       bool
	AllowAsyncTaskTools     bool
	AllowSkills             bool
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
		InjectAsyncTaskReminder: true,
		RestoreOuterHistory:     true,
	}
}
