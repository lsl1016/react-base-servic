# OpenHands Software Agent SDK 运行时学习文档

> 调研对象：`../software-agent-sdk`（即 OpenHands 官方的 `OpenHands/software-agent-sdk` 仓库，本文简称 **OH SDK**）。
> 调研目的：为 react-base-service 引入「插件化安装、子 Agent 创建与委派、服务端代码 Workspace」能力提供设计参考。
> 文中源码引用均相对该仓库根目录（如 `openhands/sdk/agent/base.py` 指 `software-agent-sdk/openhands-sdk/openhands/sdk/agent/base.py`）。
> 撰写日期：2026-09-12。

---

## 1. 定位：它是什么，为谁服务

OpenHands 已经拆成两个仓库：

```text
OpenHands/OpenHands            Agent Canvas（React/TS Web 前端工作台），只消费 API
OpenHands/software-agent-sdk   Python Agent Runtime + Agent Server + 工具 + Workspace  ← 本次调研对象
```

OH SDK 仓库含四个包，职责边界非常干净：

| 包 | 职责 | 关键内容 |
|---|---|---|
| `openhands-sdk` | 运行时核心 | Agent、Conversation、Event、Tool、Skill、MCP、Subagent、Plugin、LLM、Security |
| `openhands-tools` | 内置工具 | terminal、file_editor、grep/glob、browser、task 委派、workflow 编排 |
| `openhands-agent-server` | 服务化 | FastAPI REST/WS，把 Conversation 变成多用户远程服务 |
| `openhands-workspace` | 执行环境 | Docker/Apptainer/Cloud Workspace，管理「装着 agent-server 的容器」 |

一句话定位：**OH SDK 是一个"服务端 Coding Agent 运行时"——把 Claude Code 式本地体验（终端、文件、代码检索、Skills、插件、子 Agent）搬到服务器上多用户运行**。它面向 Software Engineering 场景，但对任何需要「Agent + 工具 + 代码 Workspace」的领域（如数据平台运维）同样适用。

三条使用路径：

```text
① 嵌入式：Python 代码直接构造 Agent + LocalConversation.run()      （examples/01_standalone_sdk）
② 服务式：起 agent-server 容器，REST/WS 远程驱动 Conversation      （examples/02_remote_agent_server）
③ 编排式：上层平台（OpenHands Cloud/CLI/Canvas）通过暖池+webhook 管理一批 agent-server 实例
```

---

## 2. 核心概念一图流

```text
                    ┌─────────────────────────────────────────────┐
                    │  Agent（frozen Pydantic 配置体，无状态）     │
                    │  llm + tools + mcp_config + skills +        │
                    │  condenser + security_policy + instructions │
                    └──────────────────┬──────────────────────────┘
                                       │ 持有（1:1）
                    ┌──────────────────▼──────────────────────────┐
                    │  Conversation（有状态，事件溯源持久化）      │
                    │  ConversationState: status/workspace/       │
                    │  max_iterations/policies/leaf_event_id      │
                    │  EventLog: 每事件一个 JSON 文件 + parent_id  │
                    └──────────────────┬──────────────────────────┘
                                       │ run() 循环调 agent.step()
                    ┌──────────────────▼──────────────────────────┐
                    │  Agent Loop（每轮一次 step）                 │
                    │  备消息(view+condense) → LLM → 分类响应 →    │
                    │  ActionEvent → [确认?] → 并行执行 →          │
                    │  ObservationEvent 回填 → 下一轮              │
                    └──────────────────┬──────────────────────────┘
            ┌─────────────┬────────────┼──────────────┬────────────┐
            ▼             ▼            ▼              ▼            ▼
        Tool 体系      Skill 体系    MCP 工具      Subagent      Workspace
     （注册表按名     （SKILL.md   （fastmcp     （AgentDefinition  （BaseWorkspace:
      解析实例化，    + frontmatter 客户端，     注册表 + task      execute_command/
      Action/Obser-  + trigger，  动态适配成    工具 + 子会话       file/git，Local
      vation 强类型  摘要注入+    Tool，凭据    委派，继承          或容器内 agent-
      schema 对）    invoke_skill 联合类型）    workspace/策略）    server）
```

---

## 3. 基本原理

### 3.1 Agent：无状态的 frozen 配置体

`openhands/sdk/agent/base.py` 的 `AgentBase` 是 **frozen Pydantic 模型**——Agent 完全由配置定义，没有任何会话状态，因此**可跨会话共享、可序列化传输**（Agent Server 就是把 Agent 配置随请求 JSON 传入）。核心字段：

```python
Agent(
    llm: LLM,
    tools: list[Tool],            # 轻量 spec {name, params}，运行时才解析成完整定义
    mcp_config: dict[str, MCPServer],
    filter_tools_regex,            # 工具集二次过滤
    include_default_tools=True,    # 自动附加 FinishTool/ThinkTool/InvokeSkillTool
    agent_context: AgentContext,   # skills、secrets 等上下文
    system_prompt | system_prompt_filename,
    condenser,                     # 上下文压缩器（LLMSummarizingCondenser 等）
    critic, tool_concurrency_limit,
)
```

关键机制：

- **工具延迟解析**：`tools` 只存名字/参数 spec。会话初始化时 `_initialize(state)` 用线程池并行调 `tool/registry.py::resolve_tool` 物化为 `ToolDefinition`，再做去重、regex 过滤、内置工具注入。
- **配置可变、历史可续**：`verify(persisted, events)` 恢复会话时校验——工具**只能增不能减**（旧事件引用的工具必须还能解析），LLM/提示词/condenser 可随意换。这是"配置体与状态分离"带来的运维红利。
- **系统提示两段式**：`static_system_message`（可跨会话缓存，提示词缓存友好）与 `dynamic_context`（skills/secrets/当前时间，每会话不同）分开组装。

### 3.2 Conversation：事件溯源的有状态外壳

`openhands/sdk/conversation/` 三件套：

- **`ConversationState`**（state.py）：持有 `agent`、`workspace`、`execution_status`、`confirmation_policy`、`security_analyzer`、`secret_registry`、`max_iterations`、`leaf_event_id`（事件树 HEAD）等。公共字段赋值即自动落盘（autosave），`with state:` 可批量合并一次 I/O。
- **`EventLog`**（event_store.py）：持久化层。**每个事件写成一个独立 JSON 文件**（`event-{idx}-{uuid}.json`），flock 文件锁保护并发，base_state.json 存状态快照。事件带 `parent_id`，天然构成**对话树**（支持分支）。
- **`LocalConversation`**（impl/local_conversation.py，约 2600 行，运行时主体）：

```text
__init__  → ConversationState.create（读到 base_state 即 resume：agent.verify + 重建缓存视图）
          → 插件懒加载（首次 run 时 _ensure_plugins_loaded → 注册文件型子代理 → 发 SystemPromptEvent）
run()     → while 循环：逐迭代持 FIFOLock 调 agent.step()
             检查 PAUSED/STUCK/FINISHED 退出；stuck 检测；迭代/预算上限触发 ConversationErrorEvent
pause()   → 下一迭代生效（等当前 LLM 调用结束）
interrupt()→ CancellationToken 立即取消，孤儿 tool call 回填 AgentErrorEvent
fork(from_event_id) → 深拷贝事件树（分支/检查点）
navigate_to(event_id)→ 移动 HEAD 做树形回溯
```

执行状态机：`IDLE / RUNNING / PAUSED / WAITING_FOR_CONFIRMATION / FINISHED / ERROR / STUCK / DELETING`。

**没有独立的 checkpoint 机制**——"逐事件持久化 + 状态快照"本身就是 checkpoint：进程挂掉后 `LocalConversation(conversation_id=...)` 从磁盘完整重建。这一设计与基座"消息即上下文、全量落库可回放"异曲同工（OH 用文件、基座用 MySQL）。

### 3.3 Agent Loop：一步的精确流程

`agent/agent.py::Agent._step` + `agent/response_dispatch.py`：

```text
step(state):
  1. 有未回填的 pending action（如确认模式下的遗留调用）→ 先执行它们，return
  2. llm.resolve_runtime_metadata()           # 运行时探测真实端点限额
  3. msgs = prepare_llm_messages(state.view, condenser, llm)
     │ view 是活动分支的增量缓存投影；condenser 可能返回 Condensation（压缩请求）→ 发压缩事件，下轮重来
  4. resp = make_llm_completion(llm, msgs, tools)
     │ 工具 schema 统一注入 security_risk / summary 两个元字段
     │ 异常分级：参数校验错→当 user 消息回喂；上下文超限→有 condenser 则转压缩否则抛；
     │   内容策略拒答→nudge；历史损坏→rebuild_view
  5. classify_response(resp):
     TOOL_CALLS     → 逐个解析成 ActionEvent（含 malformed 参数修复、risk 提取）
                      → 需要用户确认？→ 状态置 WAITING_FOR_CONFIRMATION，本轮结束（人答复后恢复）
                      → _ActionBatch.prepare：FinishTool 截断批次 / hook 拦截分区 / 可并行的进 ParallelToolExecutor
                      → 按序回填 ObservationEvent / UserRejectObservation
                      → 含 FinishTool → FINISHED
     CONTENT        → MessageEvent → FINISHED（即 "awaiting user input"；下次 send_message 复位后继续）
     REASONING_ONLY/EMPTY → nudge 继续
```

要点：

- **"等用户输入"不是特殊状态**，就是 FINISHED + 再入；只有"工具确认"才引入 WAITING_FOR_CONFIRMATION。语义比想象中简单。
- **重试在 LLM 层**（`llm/utils/retry_mixin.py`，tenacity，5 次、8–64s 指数退避），Agent Loop 不感知。
- **工具并行**：`tool_concurrency_limit` 控制同批工具并行度；不可并行的（如编辑同一文件的 file_editor 用 `declared_resources: file:{path}` 声明资源键）串行。

### 3.4 Event 体系：可重放的强类型事件

`openhands/sdk/event/`：

- 基类 `Event(id/timestamp/source/parent_id)`，frozen；
- `LLMConvertibleEvent` 抽象 `to_llm_message()`——**事件如何变成模型上下文是事件自己的责任**，而非引擎拼装；
- `events_to_messages()` 把同一 `llm_response_id` 的多个 ActionEvent 合并成一条 assistant 消息（并行函数调用对 LLM 的正确呈现）；
- 事件类型全集：SystemPromptEvent、MessageEvent、ActionEvent、ObservationEvent / UserRejectObservation / AgentErrorEvent、TokenEvent、StreamingDeltaEvent、Condensation 系列、ConversationStateUpdateEvent、ConversationErrorEvent、HookExecutionEvent、LLMCompletionLogEvent、PauseEvent / InterruptEvent 等；
- **硬性规范**（sdk/AGENTS.md）：旧版本事件文件必须永远能加载（deprecation field handler 永久保留）——事件格式一旦落盘就是长期契约。

### 3.5 Tool 体系：强类型 Action/Observation 对 + 注册表

`openhands/sdk/tool/`：

```python
class MyTool(ToolDefinition[MyAction, MyObservation]):   # frozen Pydantic
    description = "..."
    action_type = MyAction        # 参数 schema（Pydantic 模型）
    observation_type = MyObservation
    def create(cls, conv_state, **params) -> Sequence[Self]   # 会话初始化钩子，注入 workspace 等

class MyExecutor(ToolExecutor):
    def __call__(self, action, conversation) -> MyObservation

register_tool("my_tool", factory)          # 全局注册表（名字 → 工厂）
resolve_tool(Tool(name="my_tool"), state)  # 运行时按 spec 实例化
```

- 工具名由类名自动转 snake_case（TerminalTool → terminal）；
- 参数 schema 由 Pydantic `model_json_schema()` 生成，`Schema.to_mcp_schema()` 兼容 MCP 格式；发起 LLM 请求前动态给每个工具注入 `security_risk`（LLM 自评风险）与 `summary`（一句话说明）元字段；
- **没有 `@tool` 函数装饰器**——一切工具都是 Action/Observation 类型对 + Executor，客户端侧工具（前端执行）另有 `ClientToolSpec`，仅用 JSON Schema 声明；
- 执行结果 Observation 统一为 `content: list[TextContent|ImageContent] + is_error`，回填前经 secret_registry 脱敏。

### 3.6 Skill 体系：摘要注入 + 按需加载

`openhands/sdk/skills/skill.py` + `context/agent_context.py`：

- **发现**（优先级 public < user < project）：
  - 项目级：`{workdir}/.agents/skills/<name>/SKILL.md`、`.openhands/skills/`（兼容第三方 `AGENTS.md/CLAUDE.md/.cursorrules` 等文件转成 skill）；
  - 用户级：`~/.agents/skills/`；公共市场：从 extensions marketplace 仓库拉取；
- **格式**：AgentSkills 标准——目录 + SKILL.md（YAML frontmatter：name/description/license/allowed-tools/triggers…，正文是完整说明）；同目录可带 `scripts/ references/ assets/` 资源和 `.mcp.json`（skill 自带 MCP）；
- **触发器**：`paths:` → PathTrigger（工具用到相关路径时注入）；`inputs:` → TaskTrigger；`triggers:` → KeywordTrigger（用户消息含关键词时，把 skill 提示追加到该条用户消息后缀）；无触发器 → 常驻；
- **两级注入**（与基座现有设计同构）：常驻/legacy skill 全文进 `<REPO_CONTEXT>`；AgentSkills 格式的只进 `<available_skills>` XML——**仅列 name+description**（故意不暴露文件路径，防止模型绕过工具直接读文件），模型用内置 `invoke_skill` 工具加载全文；
- `invoke_skill`（`tool/builtins/invoke_skill.py`）：加载时还会执行 SKILL.md 内联的 `` !`cmd` `` 动态命令块（如现场采集环境信息），并把执行结果一并注入。

### 3.7 MCP 集成

`openhands/sdk/mcp/`：

- `MCPClient` 继承 fastmcp.Client（同步桥接）；`MCPServer` 模型描述传输：stdio（command/args/env）或 http/streamable-http/sse（url/headers）；
- `create_mcp_tools(mcp_config)`：连接 → `list_tools` → 每个远程工具动态生成 `MCPToolDefinition`（用 `Schema.from_mcp_schema()` 为其动态建 Pydantic Action 类型，LRU 缓存 512）；订阅 `tools/list_changed` 通知动态刷新；
- **MCP 工具与本地工具完全同构**——进了注册表就是普通 ToolDefinition，统一享受 schema 注入/脱敏/确认策略；
- 凭据：`MCPAuthCredential` 判别联合（none/apikey/bearer/basic/header/oauth），字段全 SecretStr，支持 Fernet 加密序列化与 `${VAR}` 占位符运行时展开；
- 调用超时 300s、断线重连一次、输出脱敏。

### 3.8 Subagent 与委派（本次调研重点）

四个部件：**定义（AgentDefinition）→ 注册表（registry）→ 注入（task 工具描述）→ 执行（子会话）**。

**① 定义**：`subagent/schema.py::AgentDefinition`——**一个 Markdown 文件 + YAML frontmatter**，正文即 system prompt：

```markdown
---
name: dba-agent
description: >
  Analyze database related problems.
  <example>should trigger when: 用户问慢查询/锁/索引问题</example>
tools: [terminal]              # 引用工具名
skills: [mysql-troubleshooting]
model: inherit                 # 或 LLM profile 名
permission_mode: confirm_risky # always_confirm / never_confirm / confirm_risky
max_iteration_per_run: 50
max_budget_per_run: 2.5        # 美元预算
mcp_config:                    # 子代理可自带 MCP
  dba: {url: "...", headers: {Authorization: "Bearer ${DBA_TOKEN}"}}
---
你是数据库排障专家。负责 SQL、索引、慢查询、锁……
```

**② 发现与注册**（`subagent/load.py` + `registry.py`）：目录优先级 `{project}/.agents/agents` > `{project}/.openhands/agents` > `~/.agents/agents` > `~/.openhands/agents`；加载时机是**首次 run 时懒加载**（`_register_file_agents`）。来源级别 `programmatic > plugin > project > user`。`agent_definition_to_factory()` 把定义编译成 Agent 工厂闭包（tools→spec、skills→解析成对象、model→LLM profile、condenser 默认压缩器）。`register_agent(name, factory)` 进全局注册表。内置子代理：general-purpose / code-explorer / bash-runner / web-researcher（`openhands/tools/preset/subagents/*.md`）。

**③ 注入主 Agent**：`openhands/tools/task/definition.py::TaskToolSet.create()` 调 `get_factory_info()` 把**所有已注册子代理的 name/description/tools 渲染进 task 工具的描述文本**——主 LLM 因此"看见"有哪些专家可派。`TaskAction{description, prompt, subagent_type, resume}`。

**④ 执行**：`task/manager.py::TaskManager`：

```text
start_task(agent_type, prompt)
  → get_agent_factory(agent_type)              # 按名取工厂
  → factory(parent_llm.copy(reset_metrics, 关流))   # 子代理默认继承父模型配置
  → LocalConversation(                          # 子会话：
       agent=子Agent,
       workspace=父.state.workspace.working_dir,   # 继承 workspace（同一代码目录）
       persistence=父目录/subagents/,               # 子会话事件独立落盘
       confirmation_policy/hooks 继承,
       delete_on_close=True)                       # 用完即删（防泄漏）
  → 阻塞运行至 FINISHED
  → get_agent_final_response()                 # 取 finish tool 调用或最后一条 agent 消息
  → 作为 task 工具的 Observation 回填父循环；子代理 metrics 以 delegate:{id} 汇入父会话
```

**⑤ 并行**：主 Agent 在一轮里发出多个 `task` 调用 + `tool_concurrency_limit` 并行执行（示例 `examples/01_standalone_sdk/25_agent_delegation.py`）；旧版 DelegateExecutor 则是内部 threading.Thread 并行（max_children=5）。另有 `workflow` 工具（`WorkflowAction`）提供脚本化编排（run_agent/map_agents/pipeline/reduce_agent）。

**设计精髓**：子代理 = **纯配置资源**（一个 md 文件），运行时 = **复用同一个 Conversation 引擎起隔离子会话**，对主 Agent 的呈现 = **一个普通工具**。没有任何新的执行引擎。

### 3.9 Plugin：资源打包分发单位

`openhands/sdk/plugin/`：

- **兼容 Claude Code 插件格式**：`.claude-plugin/plugin.json`（manifest：name/version/entry_command…）+ `commands/` + `agents/` + `skills/` + `hooks/hooks.json` + `.mcp.json`；
- `Plugin.load(path)` 经格式探测分发；`Plugin.fetch("github:owner/repo")` 支持远端拉取；安装管理（install/uninstall/enable/disable 于 `~/.openhands/plugins`）；
- **plugin 本身不是新概念，是分发单位**：加载时 skills 并入 AgentContext.skills（同名覆盖）、`.mcp.json` 并入 mcp_config（同键覆盖）、`agents/` 经 register_plugin_agents 进子代理注册表、hooks 拼接进 hook 链。**一次安装 = 同时注入技能 + 子代理 + MCP + 钩子**。

### 3.10 Workspace 与代码检索

- **抽象**（`sdk/workspace/base.py::BaseWorkspace`）：只有 `execute_command / file_upload / file_download / git_changes / git_diff` + 成本登记 + pause/resume。文件读写没有独立抽象（走命令/upload/download）。
- **两种形态**：
  - `LocalWorkspace`：宿主直接 bash 执行——**嵌入式用法**；
  - `RemoteWorkspace`（`sdk/workspace/remote/base.py`）：HTTP 客户端，连一个**跑在容器里的 agent-server**（`X-Session-API-Key` 鉴权）；
- **容器形态**（`openhands-workspace` 包）：`DockerWorkspace` 起容器（镜像 `ghcr.io/openhands/agent-server`，内含 agent-server 监听 8000）→ 轮询 /health → 之后一切命令/文件/git 操作走容器内 API；`__exit__` docker stop 清理。另有 Apptainer（HPC 无 root）、Cloud、DevWorkspace（现场构建镜像）变体。
- **架构含义**：**agent-server 进程本身就跑在 workspace 容器内部**——terminal/file_editor/grep 等工具是"会话进程本地执行"的（tmux/子进程/直接文件 IO），所以远程沙箱场景下它们天然运行在容器里；宿主侧 RemoteWorkspace 主要做**会话外的准备性操作**（如 clone_repos）。
- **clone_repos**（`sdk/workspace/repo.py`）：`RepoSource{url, ref, provider}`（github/gitlab/bitbucket，URL 自动探测 provider 防伪造）；token 按 provider 从 secret registry 取（github_token 等），注入格式各异（`{token}@` / `oauth2:{token}@` / `x-token-auth:{token}@`）；SHA ref 全量 clone+checkout，分支/tag 用 `--depth 1 --branch`；单仓超时 300s；**串行无并发**。
- **代码检索方式**：不是把仓库塞进上下文，而是 grep/glob 工具 + file_editor 逐步收敛（ripgrep 优先，回退 Python 扫描）——与 Claude Code 同款"搜索→读→再搜索"循环。

### 3.11 LLM 层与安全

- `LLM` 经 **litellm** 适配全部 provider；能力协商（ModelFeatures：vision/prompt_cache/responses_api/reasoning…）+ 运行时探测真实限额；`RouterLLM` 多模型路由；非原生函数调用模型用 prompt 模拟（NonNativeToolCallingMixin）。
- 上下文裁剪：`CondenserBase.condense(view)` → `LLMSummarizingCondenser`（滚动摘要，max_size 240 条、keep_first 2）/ `PipelineCondenser`；上下文超限时由 Agent Loop 转成 CondensationRequest 自动压缩。
- 安全（`sdk/security/`）：`SecurityRisk` 四级（UNKNOWN/LOW/MEDIUM/HIGH）；`ConfirmationPolicy` 三态（NeverConfirm 默认 / AlwaysConfirm / ConfirmRisky 阈值）；`SecurityAnalyzer` 体系——LLM 自评（工具 schema 里的 security_risk 字段）+ ensemble 取最坏值 + defense_in_depth（pattern/policy_rails/shell AST 解析）等实现；PreToolUse hook 可拦截（blocked_actions → UserRejectObservation）。

---

## 4. Agent Server：服务端化关键

`openhands-agent-server`（FastAPI，包名 `openhands.agent_server`）：

**API 面**（`/api` 前缀，约 25 个 router）：

| 域 | 路由要点 |
|---|---|
| 会话 | `POST /conversations`（幂等创建，Agent/LLM/workspace/initial_message 随 JSON 传入）、`GET/PATCH/DELETE /conversations/{id}`、`POST …/run`（后台执行）、`/pause`（等当前 LLM 调用结束）、`/interrupt`（立即取消）、`/fork`、`/navigate`、`/condense`、`/switch_llm`、`/load_plugin`、`/ask_agent`、`/confirmation_policy` 等 |
| 事件 | `GET /conversations/{id}/events`（search/count/按 id）、`POST …/events`（发消息）、`POST …/respond_to_confirmation`（HITL 答复） |
| 能力 | `/bash/*`、`/file/*`、`/git/changes|diff|commits`、`/skills`、`/plugins`、`/mcp`、`/sub-agents`、`/settings`（schema/secrets）、`/llm/providers|models` |
| 兼容 | **OpenAI 兼容网关** `/v1/chat/completions`（SSE 流式，响应头回传 conversation id） |

**Run 语义**：没有独立 Run 实体——run = conversation 的一次后台执行，状态由 `ConversationExecutionStatus` 承载，重复 run 返回 409。这与基座「session（会话）/ run（一次执行）」两实体模型不同，OH 的 conversation ≈ 基座的 session+run 合体。

**WebSocket**（`/sockets`）：`/events/{conversation_id}` 首帧 auth（session_api_key）→ 按 `resend_mode=all|since&after_timestamp` **从磁盘重放历史**再接实时流（断线重连友好）；事件信封就是 Event 模型 JSON 原样。另有 `/session/{id}` 帧协议（`sync/durable{seq}/transient/delta`，seq 去重，exactly-once 语义）与 `/bash-events`（终端输出流）。

**并发与生命周期**（`conversation_service.py`，约 2900 行）：共享 ThreadPoolExecutor（max_concurrent_runs 默认 10），原生 arun 优先不占线程；每 conversation 一把 asyncio.Lock；会话懒加载（启动只读 meta.json 目录）；多实例共享存储用**文件租约**（ConversationLease，TTL 45s 后台续约）防双写；空闲会话 20 分钟回收。

**安全模型**：`X-Session-API-Key` 鉴权（空列表=不设防；无 key 时只绑 127.0.0.1）；`OH_SECRET_KEY` 派生密钥加密落盘 secrets；**暖池模式**（deferred_init）：容器先就绪但所有 API 503，编排层 `POST /api/init`（X-Init-API-Key）注入每用户 keys/路径/webhooks——这是 Cloud 多租户按需激活容器的机制；webhooks 把事件回推编排层。

**部署**：多阶段 Dockerfile（source/binary 两目标，PyInstaller 单文件；可选 VSCode Web/Chromium/DinD）。每 conversation 一个容器（用完即毁）是其隔离模型。

---

## 5. 主要流程走读

### 5.1 嵌入式：最小可运行单元

```python
llm = LLM(model="anthropic/claude-...")          # examples/01_standalone_sdk/01_quickstart.py
agent = Agent(llm=llm, tools=[Tool(name="TerminalTool")],
              system_prompt="You are a helpful assistant")
conv = LocalConversation(agent=agent,
                         workspace=LocalWorkspace(working_dir="./repo"),
                         max_iterations=100)
conv.run("Fix the failing test")                  # 阻塞至 FINISHED
for event in conv.get_events(): ...               # 事件即完整轨迹
```

### 5.2 服务端：一次远程 run 的时序

```text
编排层/前端                 agent-server 容器
    │ POST /api/conversations (Agent+LLM+workspace 配置)
    │ ───────────────────────────▶  ConversationService 创建（落盘 meta.json）
    │ ◀──── conversation_id ──────
    │ WS /sockets/events/{id}（首帧 auth，resend_mode=since）
    │ ───────────────────────────▶  重放历史 + 订阅实时
    │ POST /api/conversations/{id}/run
    │ ───────────────────────────▶  EventService.run()（arun/线程池）
    │ ◀── SystemPromptEvent / MessageEvent / ActionEvent / ObservationEvent / … 流式推送
    │ （需要确认时）POST …/events/respond_to_confirmation
    │ ◀── FinishTool → FINISHED
```

### 5.3 子代理委派时序（运维排障示例）

```text
主 Agent（ops-main）                    注册表/工厂                    子会话（dba-agent）
     │ LLM 决定派活：task(subagent_type="dba-agent", prompt="分析慢查询…")
     │        （task 工具描述里已渲染所有可用子代理清单）
     ├────────────▶ TaskManager.start_task
     │                get_agent_factory("dba-agent")  ──  md 定义 → Agent 工厂
     │                factory(parent_llm.copy())      ──  继承父模型，重置计量
     ├──────────────────────────────────────────▶ LocalConversation(
     │                                              workspace=父 working_dir,
     │                                              persistence=父/subagents/,
     │                                              confirmation_policy 继承)
     │                                              run("分析慢查询…")
     │                （子会话独立事件流、独立历史、独立迭代/预算上限；
     │                  危险操作按子代理自己的 permission_mode 确认）
     ◀────────────────────────────────────────── FINISHED + 最终回复
     │ get_agent_final_response() → Observation 回填
     │ 子代理 metrics 汇入父会话（delegate:{id}）
     │ 主 LLM 继续推理/汇总
```

### 5.4 插件安装与生效

```text
Plugin.fetch("github:org/ops-kit") 或本地目录
  → detect_format（.claude-plugin/plugin.json）
  → load：解析 manifest + skills/ + agents/ + .mcp.json + hooks
  → Agent 首次 run 时 _ensure_plugins_loaded：
       skills   并入 agent_context.skills（同名覆盖）
       agents/  register_plugin_agents → 子代理注册表（task 工具描述随之更新）
       mcp      并入 mcp_config → create_mcp_tools
       hooks    拼接进 PreToolUse/PostToolUse 链
```

---

## 6. 设计特点总结（值得吸收的十条）

1. **Agent 是 frozen 配置体，Conversation 是唯一有状态物**——配置可换、历史可续（verify 只要求工具不减），服务化时 Agent 就是随请求传输的 JSON。
2. **事件溯源 + "事件自己知道如何变成模型消息"**（LLMConvertibleEvent.to_llm_message）——回放、分支（fork/navigate_to）、压缩（condensation 也是一种事件）统一在一个模型里。
3. **等用户输入 = FINISHED 再入**，只有工具确认才需要专门状态——HITL 语义最小化。
4. **子代理是"配置资源 + 复用同一引擎的隔离子会话 + 一个普通工具"**，没有第二套执行系统；子代理清单渲染进工具描述是"让主 LLM 发现子代理"的全部机制。
5. **Skill 两级注入**（常驻全文 / 索引+invoke_skill 按需加载）+ 触发器（关键词/路径/任务）——上下文预算与信息量的平衡术。
6. **MCP 工具与本地工具完全同构**，进注册表后无差别对待；凭据统一 SecretStr + ${VAR} 运行时展开。
7. **Plugin 是分发单位而非运行时概念**：一次安装同时注入 skill+子代理+MCP+hooks，且直接兼容 Claude Code 插件生态。
8. **Workspace 抽象极薄**（execute_command + 文件/git 五个方法），容器隔离靠"agent-server 跑在 workspace 容器内"实现，而非远程过程调用工具。
9. **权限是会话级策略对象**（confirmation policy + security analyzer + 工具 schema 注入 risk 自评），可组合、可继承给子代理。
10. **服务化按"每会话一容器、用完即毁"隔离**，暖池 init + 文件租约 + webhook 回推支撑平台级编排——这是它和"中心化多租户服务"（基座形态）的根本路线差异。

## 7. 关键源码索引

| 主题 | 位置（相对 `software-agent-sdk/`） |
|---|---|
| Agent 抽象/初始化 | `openhands-sdk/openhands/sdk/agent/base.py`、`agent/agent.py`、`agent/response_dispatch.py`、`agent/utils.py` |
| Conversation/状态/事件存储 | `openhands-sdk/openhands/sdk/conversation/state.py`、`event_store.py`、`impl/local_conversation.py` |
| Event 体系 | `openhands-sdk/openhands/sdk/event/base.py`、`event/llm_convertible/*` |
| LLM/重试/能力协商 | `openhands-sdk/openhands/sdk/llm/llm.py`、`llm/utils/*`、`llm/mixins/non_native_fc.py` |
| 上下文压缩 | `openhands-sdk/openhands/sdk/context/condenser/*` |
| Tool 注册表 | `openhands-sdk/openhands/sdk/tool/{tool,schema,spec,registry,client_tool}.py`、`tool/builtins/*` |
| Skill | `openhands-sdk/openhands/sdk/skills/skill.py`、`context/agent_context.py` |
| MCP | `openhands-sdk/openhands/sdk/mcp/{client,config,tool,utils}.py` |
| Subagent | `openhands-sdk/openhands/sdk/subagent/{schema,registry,load}.py` |
| Plugin | `openhands-sdk/openhands/sdk/plugin/{plugin,loader,fetch,format/*}.py` |
| Workspace/clone_repos | `openhands-sdk/openhands/sdk/workspace/{base,local,repo}.py`、`workspace/remote/base.py`；`openhands-workspace/openhands/workspace/docker/workspace.py` |
| 内置工具 | `openhands-tools/openhands/tools/{terminal,file_editor,grep,glob,browser_use,ask_oracle,task,delegate,workflow,preset}/*` |
| Agent Server | `openhands-agent-server/openhands/agent_server/{api,conversation_router,conversation_service,event_router,event_service,sockets,session_socket,dependencies,init_router,conversation_lease}.py` |
| 示例 | `examples/01_standalone_sdk/{01_quickstart,02_custom_tools,07_mcp_integration,25_agent_delegation,42_file_based_subagents}.py`、`examples/02_remote_agent_server/*`、`examples/05_skills_and_plugins/*` |
