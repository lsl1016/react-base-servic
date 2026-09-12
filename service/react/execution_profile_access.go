package react

func (p ExecutionProfile) allowsInternalTool(name string) bool {
	switch name {
	case metaToolReadToolResult, metaToolInspectData, metaToolPythonExec, metaToolDisplayFiles, metaToolReadAttachment, metaToolInspectAttachment:
		return true
	case metaToolTodoWrite:
		return p.AllowTodo
	case metaToolCreatePlan:
		return p.AllowPlan
	case metaToolGetTool, metaToolExecuteTool, metaToolListTools:
		return p.AllowDynamicTools
	case metaToolAskQuestion:
		return p.AllowUserQuestion
	case metaToolResolveAsyncTask, metaToolGetAsyncTask:
		return p.AllowAsyncTaskTools
	case metaToolGetSkill, metaToolListSkills:
		return p.AllowSkills
	case metaToolMemoryList, metaToolMemoryRead, metaToolMemoryWrite:
		return p.AllowMemory
	default:
		return false
	}
}
