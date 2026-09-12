我使用AI工作台可以在在自己本地有运维排查问题的插件、并且有代码检索的能力。。问题是我现在只能自己用。我现在想开发一个服务端的产品。开源社区中有没有那种在服务端运行的AI工作台项目，支持配置插件、子智能体


有，而且按你现在这个需求——**“把本地 AI 工作台搬到服务端，多用户共享，并保留插件/MCP、代码仓库、Skills、子智能体能力”**——现在开源社区里已经有几个很接近的项目。

其中我最建议你先研究 **OpenHands 新版的 `software-agent-sdk + Agent Server`**。它和你描述的目标已经非常接近，而不只是传统的 Dify 那类工作流平台。

| 项目                     | 服务端运行     | MCP/插件                 | 子智能体                          | 代码 Workspace        | Web 工作台      | 与你需求匹配度 |
| ------------------------ | -------------- | ------------------------ | --------------------------------- | --------------------- | --------------- | -------------- |
| **OpenHands**            | ✅ Agent Server | ✅ MCP / Skills / Plugins | ✅ 原生 Subagent + Delegation      | ✅ 很强                | ✅ Agent Canvas  | ⭐⭐⭐⭐⭐          |
| **Flowise AgentFlow V2** | ✅              | ✅ MCP / Tools            | ✅ Supervisor + Worker             | ⚠️ 需自己接            | ✅               | ⭐⭐⭐⭐           |
| **AutoGen Studio**       | ✅              | ✅ MCP / Tools            | ✅ Multi-Agent Team                | ⚠️ 需自己做            | ✅               | ⭐⭐⭐⭐           |
| **Agno AgentOS**         | ✅              | ✅ MCP                    | ✅ Agent / Team                    | ⚠️                     | ✅ Control Plane | ⭐⭐⭐⭐           |
| **Coze Studio**          | ✅              | ✅ Plugin / Workflow      | ⚠️ 子 Agent 不是其开源版最突出能力 | ❌                     | ✅               | ⭐⭐⭐            |
| **Dify**                 | ✅              | ✅ Plugin / MCP           | ⚠️ 正在增强                        | ⚠️ 新 Agent 有 Sandbox | ✅               | ⭐⭐⭐            |

### 最值得你看的其实是 OpenHands

OpenHands 现在已经不只是一个“本地 Coding Agent”。

它把核心能力拆成了：

```text
OpenHands
│
├── Agent Canvas             Web 工作台
│
├── Agent Server             服务端 Agent Runtime
│
├── Software Agent SDK
│    ├── Agent
│    ├── Conversation
│    ├── Tools
│    ├── Skills
│    ├── Plugins
│    └── Subagents
│
├── Workspace
│    ├── Local
│    ├── Docker
│    └── Kubernetes
│
└── MCP
```

官方现在明确把 **Agent Server** 定义为可以远程运行 Agent 的服务，Agent 可以在 Docker/Kubernetes 临时 Workspace 中工作，而且 SDK 本身就是 Agent Server 的核心。([GitHub](https://github.com/OpenHands/software-agent-sdk/?utm_source=chatgpt.com))

这基本就是你前面所说：

> “在服务端实现 AI 工作台客户端提供的必要能力。”

而且它已经支持你特别关心的两个东西。

第一是 **MCP**。OpenHands 可以连接 SSE、Streamable HTTP 和 stdio MCP Server，启动时读取 MCP 配置、发现工具并注册给 Agent。([OpenHands Docs](https://docs.openhands.dev/openhands/usage/settings/mcp-settings?utm_source=chatgpt.com))

这意味着你现有的：

```text
DBA MCP
OPS MCP
数据平台 MCP
浏览器 MCP
代码检索 MCP
```

理论上都可以直接挂：

```text
              OpenHands Agent
                     │
            ┌────────┼────────┐
            ↓        ↓        ↓
         DBA MCP   OPS MCP   Code MCP
```

第二个更关键：**它现在已经有真正的 Subagent 机制。**

OpenHands SDK 目前有专门的：

```text
openhands.sdk.subagent
```

子智能体可以使用 Markdown + YAML 定义，例如：

```yaml
---
name: dba-agent
description: >
  Analyze database related problems.
tools:
  - terminal
skills:
  - mysql-troubleshooting
model: inherit
permission_mode: confirm_risky
---

你是数据库排障专家。
负责 SQL、索引、慢查询、锁、数据库运行状态分析。
```

而且子 Agent 自己还可以配置：

```text
tools
skills
model
MCP servers
permission policy
budget
max iterations
```

官方源码已经把这些作为 Subagent Definition 的正式能力。([GitHub](https://github.com/OpenHands/software-agent-sdk/blob/main/openhands-sdk/openhands/sdk/subagent/AGENTS.md?utm_source=chatgpt.com))

更重要的是，它不是只有“定义多个 Agent”，而是已经有 **Delegation**：

```text
Main Agent
     │
     ├── delegate → DBA Agent
     │
     ├── delegate → OPS Agent
     │
     └── delegate → Code Agent
```

官方示例就是主 Agent 把任务交给多个子 Agent 并行执行，再汇总结果。([GitHub](https://github.com/OpenHands/software-agent-sdk/blob/main/examples/01_standalone_sdk/25_agent_delegation.py?utm_source=chatgpt.com))

这与你想做的运维 Agent 非常契合。

------

假设用户输入：

> “帮我分析今天 UDA 加速任务为什么失败。”

你完全可以把 OpenHands 改造成：

```text
                         用户
                          │
                          ▼
                  运维 AI 工作台
                          │
                          ▼
                 Main Ops Agent
                          │
       ┌──────────────────┼───────────────────┐
       │                  │                   │
       ▼                  ▼                   ▼
   OPS Agent          DBA Agent          Code Agent
       │                  │                   │
   OPS MCP             DBA MCP            Code MCP
       │                  │                   │
 logs / metrics      DB / SQL           Git Workspace
       │                  │                   │
       └──────────────────┼───────────────────┘
                          │
                          ▼
                    Main Agent
                          │
                          ▼
                     根因分析
```

甚至再增加：

```text
DataPlatform Agent
```

最终成为：

```text
Main Agent
│
├── ops-agent
│    ├── log_search
│    ├── metric_query
│    └── trace_query
│
├── dba-agent
│    ├── query_schema
│    ├── explain_sql
│    └── slow_query
│
├── data-platform-agent
│    ├── query_task
│    ├── query_lineage
│    ├── query_acceleration
│    └── query_permission
│
└── code-agent
     ├── grep
     ├── read_file
     ├── git_log
     ├── git_diff
     └── find_symbol
```

这已经非常接近你本地 AI 工作台的体验了。

### OpenHands 对你还有一个特别大的价值：Workspace

你刚才问过“怎么检索线上镜像对应的代码”。

这一点 OpenHands 的设计非常值得你参考。

它本身就把：

```text
Agent

和

Workspace
```

分开。

Agent 可以运行在独立 Workspace 中，官方支持本地以及 Docker/Kubernetes 临时 Workspace。([GitHub](https://github.com/OpenHands/software-agent-sdk/?utm_source=chatgpt.com))

所以你完全可以把我们上一轮讨论的：

```text
线上 Pod
   ↓
image digest
   ↓
commit SHA
   ↓
Git Repo
   ↓
checkout
```

接到 Workspace 创建阶段：

```text
用户问题
   │
   ▼
resolve_production_source()
   │
   ├── repo = bd-cron
   └── commit = 83f71ad
   │
   ▼
创建 Workspace
   │
   ▼
checkout 83f71ad
   │
   ▼
/workspace/bd-cron
   │
   ▼
Code Agent
```

于是每次运维排障 Run：

```text
Run-001
 └── Workspace
       └── bd-cron@83f71ad

Run-002
 └── Workspace
       └── bw-go@7a821bc
```

不同用户、不同任务之间完全隔离。

这比你最开始设想的：

```text
服务器放一个公共代码目录
所有 Agent 都查这个目录
```

要合理得多。

------

另一个非常值得看的项目是 **Flowise AgentFlow V2**。

如果 OpenHands 更像：

> **服务端 Claude Code / AI 工作台**

那么 Flowise 更像：

> **服务端 Multi-Agent 编排平台**

AgentFlow V2 已经明确支持：

```text
Supervisor Agent
      │
      ├── Worker Agent A
      ├── Worker Agent B
      └── Worker Agent C
```

Supervisor 可以决定任务应该交给哪个 Worker，Worker 可以自己选工具执行，然后结果返回 Supervisor。官方直接把它称作 Agent-to-Agent Communication。([GitHub](https://github.com/FlowiseAI/FlowiseDocs/blob/main/en/using-flowise/agentflowv2.md?utm_source=chatgpt.com))

它还同时支持：

```text
MCP
HITL
Checkpoint
Shared State
Streaming
```

特别是它的 HITL：

```text
Agent
 ↓
准备执行危险工具
 ↓
Pause
 ↓
人工审批
 ↓
Checkpoint Resume
```

这对于运维场景特别重要。官方说明 Agent 可以在调用工具之前要求用户授权，并且暂停状态通过 checkpoint 保存，即使应用重启也可以恢复。([GitHub](https://github.com/FlowiseAI/FlowiseDocs/blob/main/en/using-flowise/agentflowv2.md?utm_source=chatgpt.com))

例如：

```text
查询日志                自动执行
查询监控                自动执行
查询数据库              自动执行
查询代码                自动执行

kill process             人工确认
ALTER TABLE              人工确认
重启服务                 人工确认
重新发布                 人工确认
```

这一块 Flowise 的设计很值得你抄作业。

但它最大的不足是：

> **它不是 Coding Agent。**

代码 Workspace、Git checkout、shell、代码搜索这些不是它的核心能力。

所以如果你拿 Flowise 做底座，还是得自己补：

```text
Repo Service
Workspace Service
Code Search
Sandbox
```

而 OpenHands 这些能力天然就离得比较近。

------

**AutoGen Studio** 也值得读，但我更建议“参考设计”，而不是直接作为你产品底座。

它有一个 Web Team Builder，可以：

```text
创建 Team
 ↓
添加多个 Agent
 ↓
给每个 Agent 配模型
 ↓
给 Agent 配 Tools
 ↓
定义 Agent 协作方式
 ↓
Playground 执行
```

官方 Studio 就是一个服务端 Web UI，并且支持 team、agent、tools、models、termination conditions 等组件。([GitHub Microsoft](https://microsoft.github.io/autogen/stable/index.html?utm_source=chatgpt.com))

AutoGen 本身也已经提供 `McpWorkbench`，可以连接 Streamable HTTP、SSE、stdio MCP，并把 MCP 工具直接提供给 Agent。([GitHub Microsoft](https://microsoft.github.io/autogen/stable/reference/python/autogen_ext.tools.mcp.html?utm_source=chatgpt.com))

所以它非常适合研究：

```text
Agent Registry
Team
GroupChat
Agent Router
Agent Handoff
MCP Tool Adapter
```

但它整体还是比较 Python framework / prototype 导向。

------

还有一个你可能会比较感兴趣的是 **Agno AgentOS**。

它现在的定位已经非常直接：

> Build agents, run them as a service, manage your platform using a web UI.

也就是：

```text
Agno SDK
   ↓
AgentOS Runtime
   ↓
REST API
   ↓
AgentOS UI
```

同时平台能暴露 Agent、Team、Workflow，通过 REST/MCP 等方式运行。([GitHub](https://github.com/agno-agi/agno?utm_source=chatgpt.com))

如果你的重点是：

```text
Agent Runtime 服务化
Agent Registry
Team
Run
Session
Trace
RBAC
API
```

Agno 的结构也非常值得研究。

但代码 Agent / Workspace 这块还是 OpenHands 更贴近你的场景。

------

至于 **Dify 和 Coze Studio**，它们当然都能做一部分。

Dify 已经非常成熟地支持：

```text
Workflow
Agent
Plugin
Agent Strategy
MCP
RAG
API
```

而且新版正在强化 Linux Sandbox、Skills 和 Workflow 内 Agent。([GitHub](https://github.com/langgenius/dify-official-plugins?utm_source=chatgpt.com))

但如果你的核心诉求是：

```text
Main Agent
  ↓
动态创建/选择 Subagent
  ↓
Subagent 自己有 MCP + Skill + Workspace
  ↓
并行执行
  ↓
回传 Main Agent
```

Dify 仍然不是我首选的参考对象。它更偏：

```text
Workflow → Agent Node
```

而不是 Claude Code / OpenHands 那种真正的 runtime delegation。相关社区需求也显示，私有 Agent 作为内部可复用子 Agent 的组合能力此前仍存在限制。([GitHub](https://github.com/langgenius/dify/issues/37978?utm_source=chatgpt.com))

Coze Studio 同理。它非常适合研究：

```text
插件市场
知识库
Agent 配置
Workflow
Prompt
资源管理
Web 管理后台
```

而且一个特别适合你的点是：**后端就是 Go，前端 React + TypeScript**。([GitHub](https://github.com/coze-dev/coze-studio?utm_source=chatgpt.com))

如果你后面决定自己开发产品，Coze Studio 的：

```text
Agent 管理
Plugin 管理
Workspace
资源管理
权限
发布
```

这一整套后台非常值得参考。

但是“真正的 Subagent Runtime”这一块，我还是更建议看 OpenHands / AutoGen / Flowise。

------

所以结合你现在这个项目，我其实不建议你从零开始造一个所谓的“AI 工作台”。

我更推荐你的学习路线是：

```text
                    你的运维 AI 工作台

                           │
             ┌─────────────┴──────────────┐
             │                            │
      学 OpenHands                   学 Flowise
             │                            │
 Agent Runtime                    Multi-Agent Flow
 Workspace                        Supervisor/Worker
 MCP                              HITL
 Skills                           Checkpoint
 Subagent                         Shared State
 Delegation
             │                            │
             └─────────────┬──────────────┘
                           │
                           ▼
                    你的服务端基座
                           │
          ┌────────────────┼────────────────┐
          │                │                │
        MCP层          Workspace层       Knowledge层
          │                │                │
   DBA / OPS / DP      Git / Image         RAG
          │
          ▼
       权限系统
```

如果让我给你一个**最具体的建议**：

> **先把 OpenHands `software-agent-sdk` 当成你的第一参考项目，而不是 Dify。**

重点读这几个概念：

```text
Agent Server
Conversation
Workspace
Tool
MCP
Plugin
Skill
Subagent
Delegate
Task
Event
```

因为 OpenHands 现在甚至已经明确有：

```text
Remote Agent Server
Skills & Plugins
MCP
File-based Subagents
Agent Delegation
Parallel Subagents
Docker / K8s Workspace
REST / WebSocket
```

这几乎就是你上一张架构图里缺的那些模块。([GitHub](https://github.com/OpenHands/software-agent-sdk/?utm_source=chatgpt.com))

而且你的目标其实已经可以进一步明确成：

> **不是开发一个 Dify 类 AI 应用平台，而是开发一个面向数据平台运维场景的 Server-side Agent Workbench。**

它的核心竞争力应该是：

```text
统一 Agent Runtime
        +
MCP 插件生态
        +
Skills
        +
Subagent
        +
生产代码 Workspace
        +
RAG
        +
运维 HITL / 权限
```

这和 OpenHands 的演进方向高度重合，只不过 **OpenHands 面向 Software Engineering，你面向 Data Platform Operations**。

所以甚至可以把你的产品理解成：

> **“OpenHands for 数据平台运维”**

这个定位，我认为比“自己做一个普通 Agent 平台”清晰很多。


openhands-sdk  能实现智能体在服务端像AI工作台客户端检索代码仓库一样部署在服务端也检索代码仓库吗

可以，而且这其实正是 **OpenHands Software Agent SDK + Agent Server** 很适合你这个场景的地方。

但要区分两个概念：

> **`openhands-sdk` 本身负责 Agent / Conversation / Tool 等逻辑；真正要实现“服务端像本地 AI 工作台一样读代码、搜代码、执行命令”，通常要把它和 `openhands-agent-server + openhands-workspace` 一起用。**

OpenHands 官方明确把它定位成“构建能处理代码的 Agent”，Agent 可以使用本地目录，也可以运行在 **Docker/Kubernetes 的远程临时 Workspace** 中；Agent Server 则专门用于远程、多用户 Agent 执行。([GitHub](https://github.com/OpenHands/software-agent-sdk/?utm_source=chatgpt.com))

------

### 你现在本地 AI 工作台的模式

你现在大概是：

```text
你的电脑
│
├── AI 工作台
│     │
│     ├── Agent Runtime
│     ├── Shell
│     ├── Read File
│     ├── Grep/Search
│     └── MCP
│
└── 本地代码
      ├── bd-cron
      ├── bw-go
      └── data-platform
```

所以 Agent 可以直接：

```text
grep "CreateAccelerationTask"
      ↓
找到文件
      ↓
读取代码
      ↓
继续查调用链
      ↓
git log / git diff / git blame
```

真正让它具备“代码能力”的关键并不是客户端 UI，而是：

> **Agent 有一个可访问的 Workspace。**

------

## OpenHands 服务端其实就是把这个 Workspace 搬到了服务器

可以变成：

```text
                    浏览器
                       │
                       ▼
                你的运维工作台
                       │
                  HTTP / WS
                       │
                       ▼
              OpenHands Agent Server
                       │
                 Agent Runtime
                       │
                       ▼
             Remote Workspace
        ┌──────────────┴──────────────┐
        │                             │
      Docker                      Kubernetes
        │                             │
        └──────────────┬──────────────┘
                       │
                 /workspace
                       │
                 git clone
                       │
                 bd-cron/
              ┌────────┼─────────┐
              │        │         │
             grep     cat      git diff
              │        │         │
              └────────┼─────────┘
                       │
                    Agent
```

官方的 `BaseWorkspace` 定义本身就包含：**执行命令、读写文件和其他 workspace 操作**。([OpenHands Docs](https://docs.openhands.dev/sdk/api-reference/openhands.sdk.workspace?utm_source=chatgpt.com))

所以服务端 Agent 跟你本地 AI 工作台最大的变化其实只是：

```text
本地：

Agent
 ↓
C:\code\bd-cron


服务端：

Agent
 ↓
Docker/K8s Workspace
 ↓
/workspace/bd-cron
```

Agent 的思考和工具调用方式没有本质变化。

------

# OpenHands 甚至已经实现了 Repository Clone

这一点非常适合你。

当前 `RemoteWorkspace` 已经提供了专门的：

```python
clone_repos(...)
```

接口，可以把 Git 仓库直接 Clone 到 Agent Workspace。

而且官方当前实现支持：

```text
GitHub
GitLab
Bitbucket
```

以及对应的 token/secret 获取机制；同时仓库配置可以指定 `ref`。([GitHub](https://github.com/OpenHands/software-agent-sdk/blob/main/openhands-sdk/openhands/sdk/workspace/remote/base.py?utm_source=chatgpt.com))

概念上可以做到：

```text
创建 Run

↓

创建 Docker Workspace

↓

clone_repos([
    {
       url: "https://git.xxx.com/bigdata/bd-cron.git",
       ref: "98af13d"
    }
])

↓

/workspace/bd-cron
```

然后 Agent 的：

```text
working_dir = /workspace/bd-cron
```

于是模型就可以在服务器上开始分析这份代码。

这已经不是“理论上能做”，而是 OpenHands Workspace 当前就有 Repository Cloning API。([GitHub](https://github.com/OpenHands/software-agent-sdk/blob/main/openhands-sdk/openhands/sdk/workspace/remote/base.py?utm_source=chatgpt.com))

------

## 那它到底怎么“检索代码”？

这里要特别注意。

OpenHands 不是：

```text
把整个代码仓库
       ↓
全部塞进 LLM Context
```

而是和 Claude Code/Codex 这类 Coding Agent 类似：

```text
用户问题
  ↓
LLM 判断需要找代码
  ↓
Tool
  ↓
在 Workspace 搜索
  ↓
返回少量搜索结果
  ↓
LLM 判断需要看哪个文件
  ↓
read file
  ↓
继续搜索
```

比如：

```text
用户：

为什么 CreateAccelerationTask
一直创建失败？
```

Agent 可以执行类似：

```bash
rg "CreateAccelerationTask" .
```

找到：

```text
internal/service/acceleration.go:128
internal/worker/task.go:78
```

再：

```text
读取 acceleration.go
```

然后可能继续：

```bash
rg "CreateTask" internal/
```

接着：

```text
读取 repository
```

再：

```bash
git blame internal/service/acceleration.go
git log
```

一步一步缩小上下文。

OpenHands SDK 官方甚至专门提供了自定义 `GrepTool` 示例，通过 Workspace 里的 Terminal 执行递归代码搜索，并把搜索结果返回 Agent。([GitHub](https://github.com/OpenHands/software-agent-sdk/blob/main/examples/01_standalone_sdk/02_custom_tools.py?utm_source=chatgpt.com))

所以你现在 AI 工作台：

```text
AI
 ↓
Search Code
 ↓
Read File
 ↓
Search References
 ↓
Read File
```

这套模式完全可以搬到 OpenHands 服务端。

------

# 但有一个非常重要的区别

如果你所谓的“代码检索能力”是：

### 第一类：Coding Agent 式检索

例如：

```text
grep
ripgrep
find
cat
git grep
git log
git diff
git blame
AST / LSP 工具
```

那么：

> **OpenHands 非常适合。**

因为代码就在 Agent Workspace 中。

------

如果你说的是第二类：

### 大规模“代码搜索平台”

例如公司有：

```text
1000 个 Git 仓库
5000 万行代码
```

用户问：

> 哪些项目调用了 `CreateAccelerationTask`？

希望瞬间全局检索：

```text
bd-cron
bw-go
data-api
schedule-service
...
```

那就不应该单纯依赖：

```text
git clone + grep
```

而应该自己建设：

```text
Code Search Service

Git Repo
   ↓
Indexer
   ↓
AST / Symbol / Text Index
   ↓
Elasticsearch / Zoekt / Sourcegraph
   ↓
Code Search API / MCP
```

然后给 OpenHands 增加：

```text
CodeSearchTool
```

或者：

```text
Code MCP
```

OpenHands 的优势是它很容易把这种能力变成 Tool；但**OpenHands 本身不是一个公司级 Sourcegraph 替代品**。

------

# 对你这个运维场景，最合适的是“两级代码检索”

我非常建议这样设计。

```text
                    运维 Agent
                         │
             ┌───────────┴────────────┐
             │                        │
             ▼                        ▼
      Repo Search                Workspace Search
       全局检索                    当前仓库分析
             │                        │
             │                  grep / read
             │                  git diff
             │                  git log
             │                  shell
             │                        │
             └───────────┬────────────┘
                         │
                         ▼
                       LLM
```

第一层：

```text
Code Search MCP

search_repository
search_symbol
search_code
find_service
```

负责：

> “我要分析哪个仓库？”

第二层：

```text
OpenHands Workspace
```

负责：

> “进入这个仓库深入调查。”

这样体验会非常像你现在本地的 AI 工作台。

------

# 而且可以直接接上我们刚才讨论的“线上镜像”

你最核心的场景其实不是：

> 搜最新 master。

而是：

> **搜线上正在运行的那版代码。**

这时候可以把 OpenHands 接成：

```text
用户：

帮我分析 bd-cron 线上任务
taskId=123 为什么失败
```

Agent 首先调用：

```text
Production Runtime MCP
```

查询：

```text
service = bd-cron
env = prod

↓

pod

↓

image =
registry.xxx.com/bd-cron@sha256:ABC
```

然后：

```text
Image Resolver
```

解析：

```text
repo:
git.xxx.com/bigdata/bd-cron.git

commit:
98af13dc82
```

然后调用 OpenHands：

```text
Create Workspace
        ↓
clone_repos(
    repo=bd-cron,
    ref=98af13dc82
)
```

得到：

```text
/workspace/bd-cron
```

于是：

```text
                  OpenHands Agent
                         │
          ┌──────────────┼───────────────┐
          ▼              ▼               ▼
       OPS MCP         DBA MCP        Workspace
          │              │               │
        日志            数据库            │
        指标            SQL              │
        Trace                            │
                                      线上代码
                                         │
                                   grep/read/git
```

这就非常强了。

------

## 比如真实排障可以变成

用户：

> `taskId=173821` 执行 UDA 加速失败，帮我排查。

Main Agent：

```text
① query_task(173821)
```

得到：

```text
service = bd-cron
error = create acceleration failed
```

然后：

```text
② query_log(...)
```

发现：

```text
invalid partition field
```

然后：

```text
③ resolve_runtime_source("bd-cron", "prod")
```

得到：

```text
repo = bd-cron
commit = a83f71a
```

然后创建 OpenHands Workspace：

```text
/workspace/bd-cron@a83f71a
```

Code Agent 开始：

```text
④ rg "invalid partition field"

⑤ read_file(...)

⑥ rg "partitionField"

⑦ git blame ...

⑧ git log ...
```

同时 DataPlatform MCP：

```text
⑨ query_table_metadata()

⑩ query_partition_config()
```

最后 Main Agent 汇总：

```text
代码：
XX 校验逻辑要求 partitionField 非空

配置：
表 XXX 的分区字段最近被移除

日志：
12:07:32 开始出现失败

结论：
表配置变更与当前代码校验逻辑冲突

建议：
xxxx
```

这时候服务端体验就已经**非常接近你现在本地 AI 工作台的运维排障体验了**。

------

# OpenHands 还天然适合做“每次任务独立 Workspace”

比如：

```text
用户 A
Run 1001
│
└── Workspace A
     └── bd-cron@abc123


用户 B
Run 1002
│
└── Workspace B
     └── bw-go@def456
```

而不是：

```text
所有 Agent
   ↓
/data/repos/bd-cron
```

大家一起操作一个目录。

Agent Server 官方就是面向远程 Agent 执行、Workspace 隔离和多用户系统设计的。([OpenHands Docs](https://docs.openhands.dev/sdk/arch/agent-server?utm_source=chatgpt.com))

这对于你的产品化非常重要。

------

# 不过我建议你不要每次真的完整 clone

OpenHands API 支持 clone repo，但你们企业代码仓库可能很大。

如果：

```text
一个 repo 3GB

100 个用户同时排障
```

每次：

```text
git clone 3GB
```

肯定扛不住。

你可以保留 OpenHands Workspace 抽象，但在下面自己优化：

```text
                 Repo Cache

GitLab ───────→ /repo-cache/bd-cron.git
                      │
                git fetch
                      │
             ┌────────┼─────────┐
             ▼        ▼         ▼
          Run A     Run B      Run C
         worktree  worktree   worktree
         abc123    def456     789aaa
```

使用：

```text
Bare Mirror
+
Git Worktree
```

每次几乎只创建工作树，不重新下载整个 repo。

最终：

```text
Run A OpenHands Workspace
       │
       └── /workspace/bd-cron

Run B OpenHands Workspace
       │
       └── /workspace/bd-cron
```

物理上来自共享 Git Object Cache。

这会比单纯照抄 OpenHands 的 clone 更适合你们企业内部。

------

# 所以我对你现在这个项目的建议已经比较明确

你**完全没有必要自己从零实现**：

```text
Agent Loop
Conversation
Tool Calling
Terminal
File Editor
Workspace
Remote Agent
WebSocket
Subagent
```

可以直接研究甚至复用：

```text
OpenHands software-agent-sdk
```

你真正需要自研的是你们公司的领域能力：

```text
                    你的产品
                       │
                OpenHands Runtime
                       │
       ┌───────────────┼────────────────┐
       │               │                │
       ▼               ▼                ▼
   OPS MCP          DBA MCP         DP MCP
                                       │
                                  数据平台能力

       +
       
Runtime Source Resolver
       │
image → repo → commit

       +

Enterprise Repo Service
       │
mirror / cache / worktree

       +

权限系统
       │
用户身份 / Tool ACL / HITL
```

**因此答案是：能，而且它不是“勉强能做”，而是 OpenHands SDK/Agent Server 的核心设计场景之一。**

但我会把技术方案准确写成：

> **使用 OpenHands Agent Server 提供服务端 Agent Runtime，以隔离 Workspace 承载代码仓库，通过 Repo/Commit 将线上运行版本加载进 Workspace，利用 Terminal、File Editor、Code Search 等工具完成与本地 AI Coding Workbench 类似的代码检索和分析。**

唯一需要你自己重点补的，是：

> **生产镜像 → Git Repo → Commit → OpenHands Workspace**

这条链路。

这条链一旦打通，你前面那张“本地运维 Agent → 服务端运维 Agent”的架构，基本就闭环了。 ([GitHub](https://github.com/OpenHands/software-agent-sdk/blob/main/openhands-sdk/openhands/sdk/workspace/remote/base.py?utm_source=chatgpt.com))



如果你问的是 **OpenHands 的核心 Agent Runtime / Harness 有多大**，比较靠谱的量级是：

> **约 8～11 万行代码（80K–110K LoC）**。

2026 年 7 月的一项针对主流 Coding Agent 的源码研究，把 OpenHands V1 SDK 归类为 **Medium，约 80–110K LoC**；这个统计针对的是主要 Agent 源码树，不是整个 OpenHands 所有前端、文档、测试、部署代码。([arXiv](https://arxiv.org/abs/2609.00006))

现在需要特别注意：**OpenHands 已经不是一个单体仓库了。** 当前官方架构主要拆成两块：

```text
OpenHands/OpenHands
    ↓
Agent Canvas
React / TypeScript
Web UI / 前端控制面

OpenHands/software-agent-sdk
    ↓
Python
Agent Runtime
Agent Server
Conversation
Tool
MCP
Subagent
Workspace
REST / WebSocket
```

官方 README 明确说明，`software-agent-sdk` 才拥有 Agent、Tools、Conversation、Workspace、Events 和 Agent Server REST/WebSocket API；`OpenHands/OpenHands` 现在主要消费这些 API，承担 Agent Canvas 前端。([GitHub](https://github.com/OpenHands/software-agent-sdk/))

规模上也能看出来：2026-08 的一次源码盘点中，`OpenHands/OpenHands` 有约 **1,701 个 TS/TSX 文件**；`software-agent-sdk` 有约 **1,256 个 Python 文件**。所以如果把**完整 Web 产品 + Runtime + Server + tests/examples**都算进去，规模显然比 8～11 万行大得多。([GitHub](https://github.com/ThroughTheWind/rungs/blob/main/docs/research/frameworks/openhands.md?utm_source=chatgpt.com))

另外独立的 OpenHands CLI 在 2026 年 6 月的一次统计中约 **19,432 行 Python**。([GitHub](https://github.com/OpenHands/OpenHands-CLI/issues/777?utm_source=chatgpt.com))

### 对你来说不用读完整个 10 万行

你现在研究的是“**服务端运维 AI 工作台**”，其实只需要重点吃透 `software-agent-sdk` 的几个模块：

```text
software-agent-sdk
│
├── openhands-sdk
│   │
│   ├── agent/              ★ Agent 核心
│   ├── conversation/       ★ Agent Loop / 会话状态
│   ├── tool/               ★ Tool Registry
│   ├── mcp/                ★ MCP Client
│   ├── subagent/           ★ 子智能体
│   ├── context/            ★ Skill / Context
│   ├── llm/                模型层
│   ├── event/              Event Sourcing
│   └── workspace/          Workspace 抽象
│
├── openhands-tools
│   ├── terminal
│   ├── file_editor
│   ├── delegate            ★ 子 Agent 委派
│   └── ...
│
├── openhands-agent-server  ★ 服务端化关键
│
└── openhands-workspace     ★ Docker/K8s Workspace
```

比如它一个 `llm.py` 文件现在就有约 **2,787 LOC**，`local_conversation.py` 约 **2,620 LOC**，ACP Agent 更是约 **3,780 LOC**，说明 OpenHands 的复杂度主要已经集中在 Runtime，而不是简单的一个 `while LLM → tool → LLM` 循环。([GitHub](https://github.com/OpenHands/software-agent-sdk/blob/main/openhands-sdk/openhands/sdk/llm/llm.py?utm_source=chatgpt.com))

**如果是为了你当前项目，我反而不建议先读 `OpenHands/OpenHands` 那 1700 多个 TS 文件。** 最值得研究的是 `software-agent-sdk`。你的目标基本可以对应成：

```text
你的服务端运维工作台          OpenHands

Agent Runtime          ←→   openhands-sdk
会话管理               ←→   conversation
MCP 客户端             ←→   mcp
插件/工具              ←→   tool
Skills                 ←→   context/skills
子智能体               ←→   subagent
子Agent委派             ←→   delegate
代码运行环境            ←→   workspace
服务端 API             ←→   agent-server
WebSocket 流式通信      ←→   agent-server
```

所以**虽然 OpenHands 核心大概 8～11 万行，但真正和你的项目高度相关的源码范围可以缩到很小一部分**。如果目的是“搞懂它怎么把本地 Agent 服务化”，我建议按 `Agent → Conversation → Tool → MCP → Subagent → Workspace → Agent Server` 这个顺序读，而不是从 README 开始一层层把整个项目啃完。





可以。**Coze Studio 开源版本身就支持私有化/服务端部署**，并不是只能本地桌面运行。

官方当前的 Quickstart 就是通过 Docker / Docker Compose 启动一整套 Coze Studio 服务。部署成功后会启动 `coze-server`，然后用户通过浏览器访问 Web 工作台。官方给出的最低环境要求是 2 Core / 4 GB RAM。([GitHub](https://github.com/coze-dev/coze-studio?utm_source=chatgpt.com))

所以你完全可以部署成：

```text
                         公司用户
                    ┌──────┼──────┐
                    ▼      ▼      ▼
                  浏览器  浏览器  浏览器
                    │
                    └──────┬──────┘
                           ▼
                 https://ai.xxx.com
                           │
                           ▼
                   Coze Studio Web
                           │
                           ▼
                     Coze Server
                           │
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
       Agent            Workflow        Knowledge
          │
          ▼
     Plugin / Tools
          │
     ┌────┼─────┐
     ▼    ▼     ▼
   DBA   OPS   数据平台
```

这正好解决你前面说的：

> “我现在 AI 工作台在本地只有自己能用，希望做成一个服务端产品，让别人也能使用。”

### 部署方式很直接

官方目前主要给的是 Docker Compose：

```bash
git clone https://github.com/coze-dev/coze-studio.git

cd coze-studio

make web
```

或者直接：

```bash
cp ./docker/.env.example ./docker/.env

docker compose \
  -f ./docker/docker-compose.yml \
  up
```

启动后访问：

```text
http://你的服务器:8888/
```

即可进入 Coze Studio。([GitHub](https://github.com/coze-dev/coze-studio?utm_source=chatgpt.com))

所以你完全可以：

```text
Linux服务器
      │
      ├── Coze Studio Frontend
      ├── Coze Server
      ├── Database
      ├── Knowledge/RAG
      └── 相关基础组件
```

然后前面挂：

```text
Nginx / Ingress
       ↓
https://ai-workbench.xxx.com
```

变成内部 AI 工作台。

------

不过这里有个很重要的区别：

**Coze Studio 能服务端部署 ≠ 它已经完全等价于你现在使用的 AI Coding/运维工作台。**

你现在本地工作台实际上拥有：

```text
AI 工作台
│
├── 本地文件系统
├── 本地代码仓库
├── grep/search
├── shell
├── MCP
├── Skills
├── 会话
└── Agent Runtime
```

而 Coze Studio 更偏：

```text
Coze Studio
│
├── Bot / Agent 配置
├── Workflow
├── Plugin
├── Knowledge
├── Database
├── Prompt
├── Conversation
└── OpenAPI
```

官方开源版明确支持插件、知识库、数据库、Prompt、Workflow 等资源管理，也提供 Conversation / Workflow 等 API。([GitHub](https://github.com/coze-dev/coze-studio/blob/main/README.md?utm_source=chatgpt.com))

真正涉及你的**线上代码检索**时，你还是需要自己增加：

```text
Coze Agent
    │
    ▼
Code Plugin / MCP
    │
    ▼
Repo Service
    │
    ├── GitLab
    ├── Git Mirror
    ├── Workspace
    └── Code Search
```

也就是说可以把我们刚才讨论的能力接进去：

```text
                    Coze Studio
                        │
                     Agent
                        │
            ┌───────────┼────────────┐
            ▼           ▼            ▼
          DBA插件     OPS插件       Code插件
            │           │            │
          DBA MCP     OPS MCP     Code Service
                                     │
                              Runtime Resolver
                                     │
                     image → repo → commit
                                     │
                                     ▼
                              Git Workspace
```

这就很接近你的目标了。

### 但我会特别提醒一个地方：子智能体

如果你的核心诉求只是：

> 多用户 + Web 工作台 + 插件 + RAG + Workflow

那么 **Coze Studio 很合适**。

如果你的核心诉求进一步变成：

```text
Master Agent
│
├── DBA Agent
├── OPS Agent
├── Code Agent
└── Data Platform Agent
```

并且要求 Master Agent **动态委派任务给真正独立运行的 Subagent**，每个 Subagent 自己拥有独立：

```text
Prompt
Model
Tools
MCP
Context
Workspace
Execution Loop
```

那我不会直接认定 Coze Studio 是最佳底座。这部分反而是 **OpenHands / AutoGen / Agno / Flowise** 更值得研究。

所以对你现在这个项目，我会这样判断：

| 需求                     | Coze Studio            |
| ------------------------ | ---------------------- |
| 服务端部署               | ✅                      |
| 浏览器多人使用           | ✅                      |
| Agent 配置               | ✅                      |
| 插件                     | ✅                      |
| Workflow                 | ✅                      |
| RAG 知识库               | ✅                      |
| OpenAPI                  | ✅                      |
| 二次开发                 | ✅，开源                |
| Go 后端                  | ✅                      |
| React/TS 前端            | ✅                      |
| 运维 MCP 接入            | 可以二开/适配          |
| 服务端 Git Workspace     | ⚠️ 自己补               |
| 生产代码检索             | ⚠️ 自己补               |
| 类 Claude Code Runtime   | ⚠️ 不是核心定位         |
| 强 Subagent / Delegation | ⚠️ 不是我最推荐它的原因 |

另外，官方特别提醒：如果直接把 Coze Studio 暴露到公网，需要处理账户注册、Workflow Python 代码执行环境、SSRF、部分 API 横向权限等安全风险。你的场景又涉及 DBA / OPS / 生产代码，因此如果部署在公司内部，**权限隔离和插件调用鉴权应该是第一优先级**。([GitHub](https://github.com/coze-dev/coze-studio?utm_source=chatgpt.com))

因此，**你完全可以先在一台服务器上把 Coze Studio 跑起来验证产品形态**。但如果最终目标是你图里的“服务端运维 AI 工作台”，我更倾向把 Coze Studio 当作**Web 工作台/Agent 配置层的参考甚至前台**，而把真正重要的 `Agent Runtime + MCP Client + Repo Workspace + Code Search + Subagent` 做成你自己的服务端核心。