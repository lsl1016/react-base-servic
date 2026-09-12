# 多智能体编排调研：eino 与 Google ADK Go

> 目的：为基座（react-base-service）引入多智能体编排能力做设计前的学习与选型调研。
> 调研对象：`../eino`（github.com/cloudwego/eino，HEAD `9d983b36`，2026-09-01）、`../adk-go`（google.golang.org/adk/v2，HEAD `4e57df4`，2026-09-11）。
> 文中源码引用均相对各项目根目录（如 `adk/interface.go:453` 指 `eino/adk/interface.go`）。

---

## 1. 一页结论（TL;DR）

| | eino | adk-go |
|---|---|---|
| 定位 | Go LLM 应用框架（组件 + 编排 + ADK），借鉴 LangChain 与 ADK | Google ADK 的 Go 实现，**行为与 Python 版 parity 优先** |
| 多智能体底层 | 双轨：`compose` 图引擎（Pregel 式调度 + checkpoint）+ `adk` 自研事件管道，两套 checkpoint 并存 | 单轨：一切 Agent（含顶层）都包成 `workflow` 图节点，统一 scheduler 调度，RunState 存 session.State |
| 主推协作模式 | **AgentTool（子 Agent 即工具）+ DeepAgent**；transfer/Supervisor 全上下文共享模式官方标注 NOT RECOMMENDED | **sub_agents 树 + transfer_to_agent（AutoFlow）+ Mode 委托**（chat/task/single_turn 自动装委托工具）；另有 AgentTool 隔离模式 |
| Agent 抽象 | 泛型接口 `TypedAgent[M]`，Run 返回自研 `AsyncIterator[*AgentEvent]` | 最小接口 `Agent`（5 方法）+ 未导出 `internal()` 锁实现，Run 返回标准库 `iter.Seq2[*Event, error]` |
| 状态/历史共享 | 一次 run 全体 Agent 共享 runContext.Session（事件账本），转移时用 HistoryRewriter 把他人消息改写成 "For context: [agent] said: ..." | **默认隔离**：Branch 前缀 + IsolationScope 精确过滤组装各自 LLM 历史；跨 Agent 传状态走 StateDelta（`app:`/`user:`/`temp:` 三作用域）+ OutputKey |
| HITL | 中断三件套（无状态/有状态/复合）+ Address 分层寻址 + gob checkpoint（RunCtx 整体序列化） | 工具确认流（RequestConfirmation → FunctionResponse 回传）+ 工作流暂停（RequestedInput + InterruptID + RunState 进 session.State，RerunOnResume 决定重入/移交） |
| 对基座最值得学 | 官方演进结论（AgentTool 优于 transfer）、AgentAction 作用域规则、安全点取消、模型 retry/failover 包装器链、中间件接口 | **三层运行模型（invocation/agent call/step）**、Mode + 自动委托工具、Branch 隔离、事件协议字段（Author/Branch/NodeInfo/StateDelta 可序列化）、`internal()` 锁接口、编排三件套的实现简洁性 |

**给基座的核心建议**（详见 §6）：优先落地「子 Agent 即 Meta Tool + 事件路径（RunPath/Branch）+ 历史隔离」三件事，即两个框架殊途同归的共识路径（eino 官方把 transfer 降级为不推荐、adk-go 用 Branch 隔离修复 transfer 的上下文串扰，都指向"子 Agent 隔离执行、主 Agent 调度"）。编排容器（Sequential/Parallel/Loop）二期再加，图引擎不建议引入。

---

## 2. 基座现状与差距

当前基座是**单 Agent ReAct 运行时**（`service/react/`）：

- 主循环：`engine.go` 的 `executeReactLoop`（模型轮 → 解析 tool call → 执行 → 回填）；
- 工具体系：`meta_tools.go` 内置 Meta Tool 注册表 + 两段式业务工具加载（`list_tools`/`get_tool`/`execute_tool`），另有 `ask_question`、`create_plan`、`todo_write`、`python_exec`、`resolve_async_task` 等内置工具；
- 事件协议：WS/SSE 下发 `tool_use_start/end` 等事件，带 `step` 序号；run 状态机含 `running / waiting_client_message / expired` 等，run 与消息全量落 MySQL；
- HITL：`ask_question`（结构化澄清）与 `create_plan`（计划确认）均基于「run 进入 waiting + 等前端上行」；
- 容错：`model_failover.go` 模型互备；`runtime_support.go` 上下文压缩 / resultRef 落库。

引入多智能体缺失的能力，按依赖顺序：

| # | 缺失能力 | 说明 |
|---|---|---|
| 1 | Agent 抽象与子 Agent 声明 | 目前只有一种"引擎角色"，无法把某个 skill/工具组合/提示词配置声明为一个可被调度的 Agent |
| 2 | 事件归属标识 | 事件只有 `step`，无「哪个 Agent 产出」的路径标识，多 Agent 事件流无法区分归属 |
| 3 | 历史与上下文隔离 | 所有消息同属一个 session 历史；子 Agent 若共享将互相污染上下文（eino 官方实证不推荐） |
| 4 | Agent 间控制转移 / 委托 | 无 transfer、无 agent-as-tool、无 Mode 概念 |
| 5 | 跨 Agent 状态传递 | 无 OutputKey/StateDelta 之类受控共享通道 |
| 6 | 编排容器 | 无 Sequential/Parallel/Loop 等确定性编排 |
| 7 | 跨 Agent 的 HITL 与恢复 | `ask_question` 的 waiting 机制在单 Agent 语义下设计，嵌套子 Agent 等待用户作答时父循环如何挂起/恢复需要新的寻址与持久化设计 |

---

## 3. eino 的多智能体编排设计

### 3.1 分层结构：compose（图引擎）与 adk（Agent 套件）

- **compose**：通用图编排引擎。`Chain`（线性）/`Graph`（允许环，Pregel 式调度）/`Workflow`（依赖声明式、无环 DAG）三种形态，统一编译为 `Runnable[I,O]`，支持 Invoke/Stream/Collect/Transform 四种调用形态互转（`compose/runnable.go:32`）。
- **adk**：Agent 开发套件。核心单 Agent `ChatModelAgent` 的 ReAct 循环**构建在 compose.Graph 之上**（`adk/react.go:326` `type reactGraph = *compose.Graph[*reactInput, Message]`）；但多 Agent 间转移与编排（flow/workflow agent）是 adk **自研的运行时**（goroutine + AsyncIterator 事件管道 + runContext），不走 compose 图。
- 结果是**两套 checkpoint 并存**：compose 的 `checkpoint{Channels, Inputs, State, SubGraphs...}`（`compose/checkpoint.go:108`）与 adk 的 `serialization{RunCtx, InterruptID2State...}`（`adk/interrupt.go:210`），通过「ChatModelAgent 内嵌 compose 图的子图 checkpoint」衔接，甚至需要字节级兼容修补（`chatmodel.go:1728` `preprocessComposeCheckpoint`、`adk/interrupt.go:274` 对 v0.8.0–v0.8.3 旧 gob 线格式做等长替换迁移）。**这是双轨制付出的显著复杂度代价，选型时应引以为戒。**

### 3.2 Agent 核心抽象（adk/interface.go）

```go
// adk/interface.go:453
type TypedAgent[M MessageType] interface {
    Name(ctx context.Context) string
    Description(ctx context.Context) string
    Run(ctx context.Context, input *TypedAgentInput[M], options ...AgentRunOption) *AsyncIterator[*TypedAgentEvent[M]]
}
type Agent = TypedAgent[*schema.Message]        // :467
```

- 消息类型是密封联合（`*schema.Message | *schema.AgenticMessage`，`interface.go:43`），泛型 `M` 约束之；
- `TypedAgentEvent[M]{AgentName, RunPath []RunStep, Output, Action, Err}`（`interface.go:419`）——**RunPath 是框架维护的"根到当前事件源"执行路径**，多 Agent 事件的归属标识；
- `AgentAction`（`interface.go:357`）：`Exit / Interrupted / TransferToAgent / BreakLoop / CustomizedAction`，并有明确的**作用域规则**（`interface.go:346` 注释）：被包装为 AgentTool 的内层 Agent 发出的 Exit/Transfer/BreakLoop **不会穿透**到父 Agent，仅 Interrupted 经 CompositeInterrupt 上传——"动作不越权、中断必上报"。

### 3.3 子 Agent 组织：静态树 + flowAgent 管道

`flowAgent`（`adk/flow.go:42`）包装任意 Agent，赋予子 Agent 管理、转移、历史改写能力：

- 子 Agent 为**静态树**：`SetSubAgents()` 一次性注入（`flow.go:75`），重复设置/重复挂载报错；内层实现 `OnSubAgents` 接口可感知（ChatModelAgent 在首次运行后冻结，`chatmodel.go:697`）；
- 一次 run 全体 Agent 共享 `runContext.Session`（事件账本，带纳秒时间戳）与 `RootInput`；每个 Agent 运行前用 `genAgentInput`（`flow.go:275`）从 session 事件重建自己的消息历史，**他人消息经 `rewriteMessage`（`flow.go:188`）改写为 "For context: [agent] said: ..." 的 User 消息**——这就是"全上下文共享"的具体实现；
- `flowAgent.run`（`flow.go:481`）：消费内层事件 → 按 `exactRunPathMatch` 精确匹配本 Agent 路径的事件写入 session → 检查最后的 Action（Interrupted/Exit/TransferToAgent）→ transfer 则定位目标（子或父）继续跑。

### 3.4 编排原语（adk/workflow.go）

只有三种，**没有 Switch**（Supervisor 在 prebuilt 单独实现）：

- `NewSequentialAgent / NewParallelAgent / NewLoopAgent`（`workflow.go:694/703/712`），配置 `SubAgents []Agent`，子 Agent 统一 `WithDisallowTransferToParent()`；
- `runSequential`：顺序执行，事件直通；支持从 `sequentialWorkflowState{InterruptIndex}` 恢复，子 Agent 中断时用 CompositeInterrupt 包一层带本层 index；
- `runLoop`：`maxIterations==0` 表示无限；支持 `BreakLoopAction{From, Done, CurrentIterations}`，Done 标志防止内层 break 重复作用到外层循环；
- `runParallel`：goroutine + WaitGroup 并发。**关键设计是 laneEvents 泳道**：`forkRunCtx`（`runctx.go:478`）为每个子 Agent fork 独立 session 泳道（免锁追加事件），`joinRunCtxs`（`runctx.go:423`）按时间戳排序合并提交——并行写同一事件账本的并发问题用"泳道 + 合并"解决。

### 3.5 Agent 间转移：三种机制与官方立场

1. **Agentic 转移（LLM 决定）**：ChatModelAgent 挂了 sub-agent 后框架注入 `transfer_to_agent` 工具（`chatmodel.go:583-614`，参数 `agent_name`）；工具体内 `SendToolGenAction` 把 Action 暂存到 compose 图状态，工具事件发出时附着（`react.go:274`）；转移发生时框架伪造合成消息对（一条 assistant 的 transfer ToolCall + 对应 ToolMessage，`utils.go:96` `GenTransferMessages`）保证历史可回放。
2. **确定性转移**：`AgentWithDeterministicTransferTo`（`deterministic_transfer.go:43`）包装器，Agent 结束后必转指定目标；flowAgent 场景用**隔离 session**（只共享 Values）运行以免污染主账本。
3. **AgentTool（官方主推）**：`NewAgentTool`（`agent_tool.go:93`）把任意 Agent 包装成 `tool.BaseTool`——Agent 的 Name/Description 即工具名/描述，默认参数 schema `{request: string}`。要点：
   - 用 `bridgeStore`（内存 checkpoint 桥，固定 ID）构造**独立 Runner** 跑子 Agent → 子 Agent 拥有自己的 checkpoint 域，**中断可跨工具边界恢复**；
   - `WithFullChatHistoryAsInput()`：可把父历史（角色改写后）作为输入；
   - `EmitInternalEvents`：子 Agent 事件实时转发父事件流（RunPath 拼父前缀），但**不记入父 session/checkpoint**；
   - 子 Agent 中断以 `tool.CompositeInterrupt` 上抛（`agent_tool.go:251`）。

**官方立场（贯穿代码注释）**：*"NOT RECOMMENDED: Agent transfer with full context sharing between agents has not proven to be more effective empirically. Consider using ChatModelAgent with AgentTool or DeepAgent instead."* —— 全上下文共享的 transfer/Supervisor/WorkflowAgent 被降级为不推荐，主推 **主 Agent + 子 Agent 作为工具 + DeepAgent**。

### 3.6 预置模式（adk/prebuilt）

- **DeepAgent（主推）**（`prebuilt/deep/deep.go:116`）：本质是增强版 ChatModelAgent——内置 `write_todos` 计划工具、可选 filesystem/shell 工具、以及 `task` 工具把 `cfg.SubAgents` 全部经 **AgentTool** 暴露给主 Agent 调度；
- **Supervisor**（`prebuilt/supervisor/supervisor.go:101`）：星型拓扑，每个子 Agent 包确定性转移回 supervisor；标注 NOT RECOMMENDED；
- **Plan-Execute**（`prebuilt/planexecute/plan_execute.go:862`）：用编排原语组合 `Sequential(Planner, Loop(Executor, Replanner, max=10))`，Planner/Executor/Replanner 可独立替换。

> 对照基座：DeepAgent 的形态（主 Agent + todos 计划工具 + task 委托子 Agent）与基座现有的 `create_plan` + `todo_write` + `execute_tool` 高度同构——**基座补一个「把子 Agent 暴露为工具」的机制即可自然长出 DeepAgent 形态**。

### 3.7 中断与恢复（adk/interrupt.go）

- 三级构造：`Interrupt`（无状态，面向用户的说明）/ `StatefulInterrupt`（携带恢复用内部状态）/ `CompositeInterrupt`（聚合子中断信号，workflow/AgentTool 边界用）——全部转 `internal/core.Interrupt` 并附 RunPath；
- **Address 分层寻址**（`interrupt.go:161`）：段类型 `AddressSegmentAgent/AddressSegmentTool`，`InterruptCtx{ID, Address}` 是用户可见视图；`ResumeInfo{WasInterrupted, InterruptState, IsResumeTarget, ResumeData}` 供恢复侧消费；
- checkpoint 根结构 `serialization{RunCtx, Info, InterruptID2Address, InterruptID2State...}` gob 序列化整体落盘；`ResumeWithParams` 支持**定向恢复**（`ResumeParams.Targets` 以中断地址 ID 为键，注释详述"隐式全恢复 vs 定向恢复、叶子必须重新中断、组合 Agent 作为管道放行"的语义，`runner.go:129`）。

### 3.8 取消：安全点 + 中断吸收（adk/cancel.go）

- `CancelMode` 位掩码：`CancelImmediate / CancelAfterChatModel / CancelAfterToolCalls`（可 OR 组合，在先到的安全点取消）；安全点物理落位在 ReAct 图的 `CancelCheck`（模型产出 tool calls 后，`react.go:402`）与 `AfterToolCallsCancelCheck`（`react.go:476`）节点；
- `WithRecursive()`：取消传播到 checkpoint 感知的子 Agent 边界，各后代各自中断并级联上传，根 checkpoint 包含全部后代 → 可精确恢复；
- **中断吸收语义**（`cancel.go:162` 注释）：取消活跃期间任何业务中断都被吸收为 CancelError（并发时两种信号无法拆分），业务中断数据保留在 checkpoint，恢复重跑时自然再触发；
- 状态机 `stateRunning→stateCancelling→stateCancelHandled/Done` CAS int32 实现；配套 race/边缘测试（cancel_stream_race_test.go 等）。

另：`turn_loop.go` 的 TurnLoop 提供常驻 push 模型（`Push(item, WithPreempt(safePoint))` 抢占当前轮、Stop 的 graceful/immediate 等多级语义、TurnLoopExitState 完整退出态）——面向长时运行 Agent 的会话级生命周期管理，基座二期可参考。

### 3.9 中间件与模型容错

- **ChatModelAgentMiddleware**（`handler.go:139`）接口式钩子链：`BeforeAgent/AfterAgent`、`BeforeModelRewriteState/AfterModelRewriteState`（推荐的消息/工具改写点）、`WrapInvokableToolCall/WrapStreamableToolCall`（工具调用端点包装）、`WrapModel`（模型包装，重试/故障转移推荐点）；`ChatModelAgentContext` 允许运行期改 Instruction/Tools；配套 `SetRunLocalValue`（KV 随 checkpoint 持久化，gob 可编码性预检）与 `SendEvent`（中间件发自定义事件）。内置中间件：summarization（token 超限自动摘要压缩）、filesystem、plantask、toolsearch（动态工具发现）、reduction（工具输出两阶段精简）、patchtoolcalls（修补悬空 tool call）、skill、agentsmd；
- **retry**（`retry_chatmodel.go:222`）：`ShouldRetry` 返回 Decision（可改输入消息/控制延迟，指数退避 100ms→10s），流式先消费完整流再判定；`RetryExhaustedError/WillRetryError` 为事件流可观测设计；
- **failover**（`failover_chatmodel.go:128`）：`MaxRetries + ShouldFailover(可拿到流式部分累积消息) + GetFailoverModel(attempt/上次输出/上次错误)`，实现为包装器链 `failover → retry → eventSender → 用户 WrapModel → proxy → 真实模型`（`chatmodel.go:327` 注释）——与基座 `model_failover.go` 的目标一致，但其"包装器链 + 事件可观测重试"的形态更工程化。

---

## 4. Google ADK Go 的多智能体编排设计

### 4.0 总体：单轨图引擎 + 三条协作通路

adk-go 的多智能体有三种用法，全部由 Runner 驱动，产出统一为 `iter.Seq2[*session.Event, error]` 事件流：

1. **经典 sub_agents 树 + transfer_to_agent**（LLM 自主路由，对齐 Python AutoFlow）；
2. **Workflow 图引擎**（`workflow/` 包：Node/Edge/Route + scheduler），LlmAgent 可作为图节点，按 Mode 委托执行；
3. **AgentTool**（`tool/agenttool`，agent 包装成工具，独立会话运行）。

关键架构决策：**顶层 LlmAgent 也被包成合成单节点 workflow 跑图引擎**（`runner/run_node.go:127` 注释："Python 直接跑 agent.run_async，Go 统一走图引擎"）——与 eino 的双轨制相反，adk-go 选择"一切皆节点"的单轨制，RunState 直接序列化进 session.State，只有一套持久化模型。

### 4.1 三层运行模型（agent/context.go:28-62，全仓库最佳注释）

```
invocation  ── 用户消息 → 最终回复，由 Runner.Run 驱动，可含多次 transfer
  agent call ── agent.Run(ctx) 一次
    step     ── 一次 LLM 调用 + 工具调用
```

这个三层词汇表直接回答了"一次 run 里 Agent/LLM 调用如何嵌套"的建模问题，基座引入多 Agent 前应先对齐等价词汇（当前基座的 run ≈ invocation，executeReactLoop 的一轮 ≈ step）。

### 4.2 Agent 抽象：最小接口 + internal() 锁实现

```go
// agent/agent.go:44
type Agent interface {
    Name() string
    Description() string
    Run(InvocationContext) iter.Seq2[*session.Event, error]
    SubAgents() []Agent
    FindAgent(name string) Agent
    internal() *agent      // 未导出方法：包外无法自行实现 Agent，强制走构造函数
}
```

- `agent.New(Config{Name, Description, SubAgents, Run func(...), Before/AfterAgentCallbacks})`：自定义 Agent 只提供一个 Run 回调；
- **父指针不在 Agent 上**：Runner 构建时用 `parentmap.New` 一次性生成全树 name→parent 映射并校验（单父、树内名字唯一），放入 context；`RootAgent()` 沿 map 上溯——把"树结构"从 Agent 对象中剥离，Agent 保持可复用的纯配置体；
- base Run 统一做：telemetry span → BeforeAgentCallbacks（返回非 nil content 即短路 EndInvocation）→ 逐事件**为空 Author 盖当前 agent 名** → AfterAgentCallbacks。

### 4.3 LlmAgent：Mode 体系与自动委托工具（对基座最有启发的设计之一）

`agent/llmagent/llmagent.go`：

- **Mode 三态**（`llmagent.go:344`）：
  - `ModeChat`：对话同侪，经 `transfer_to_agent` 可达；
  - `ModeTask`：多轮完成一个任务，注入 `finish_task` 工具（`internal/workflowinternal/finish_task_tool.go`）显式交回控制权；
  - `ModeSingleTurn`：一次性完成任务（workflow 节点默认），以子 agent 名为工具名暴露给父；
- **installTaskTools**（`llmagent.go:139`）：按"自身 Mode + 各子 Agent 的 Mode"**自动安装委托工具**——子是 single_turn 装.SingleTurnTool，子是 task 装 TaskAgentTool（`DefersResponse()=true`），未声明 Mode 的子是 chat 同侪走 transfer。**声明式配置 → 自动工具装配**，用户不需要手写"把子 agent 包成工具"的胶水；
- Mode 解析是 placement-based：`ResolveMode(declared, byPlacement)`（`internal/llminternal/mode.go:92`），context key 用 `boundModeKey{agentName, State}` 双要素，避免同名 agent 串模式；Runner 为根绑 chat，AgentNode 为节点绑 single_turn；
- 其它关键字段：`Instruction` 支持 `{state.key}`/`{artifact.name}`/`{var?}` 模板占位（`instruction_processor.go:204`），`InstructionProvider` 动态指令，`GlobalInstruction` 仅根生效，`IncludeContents`（none/default，single_turn 节点默认只看当前轮），`InputSchema/OutputSchema`（agent 作为工具的契约），`OutputKey`（最终回复写入 state 的键）。

### 4.4 transfer_to_agent（AutoFlow，对齐 Python）

`internal/llminternal/agent_transfer.go`：

- 有 sub_agents 或未禁双向转移时，request processor 动态注入 `TransferToAgentTool`——`agent_name` 参数的 enum 即**合法目标白名单**（全部 sub_agents + 父[除非 DisallowTransferToParent] + 同侪[父也是 AutoFlow 且未 DisallowTransferToPeers]；task/single_turn 模式的子被剔除）；
- 工具 Run 只做一件事：`ctx.Actions().TransferToAgent = agent`（纯声明，不改控制流）；
- 指令注入：safehtml 模板生成"可用 agent 名单 + 描述 + 转移规则"的系统指令（注明源自 Python `_build_target_agents_instructions`）；
- 执行侧：`base_flow.go:718` FR 事件后若 `Actions.TransferToAgent != ""`，把目标 agent 的 Run 流**接在当前迭代器后转发**（一次 Run 同时返回转移事件与目标首个回复）；**下一轮路由**由 Runner.`findAgentToRun`（`runner.go:1151`）完成：从 session 倒序找最后一个非 user 事件的 author，沿 parent 链校验可达性后作为入口，否则回退 root。

### 4.5 编排三件套与 Workflow 图引擎

**编排 Agent**（`agent/workflowagents/`，均经 `agent.New` + 自定义 Run，禁止用户覆写）：

- **SequentialAgent**（`sequentialagent/agent.go:80`）：按序逐个 `subAgent.Run(ctx)` 转发事件——实现就是十行顺序循环；
- **ParallelAgent**（`parallelagent/agent.go:68`）：errgroup 并发；每个子分配隔离 branch `"<parent>.<child>"` + 新 InvocationContext；事件经 resultsChan 汇聚 + **ackChan 背压握手**（子 agent 每发一个事件，等 runner 消费完[含 session append]再继续）；消费者提前 break → cancelSubAgents 并 drain 到 Wait 完成；
- **LoopAgent**（`loopagent/agent.go:75`）：死循环「跑一遍 sub_agents → 检查任一子 `Actions.Escalate` 即整体退出 → MaxIterations 倒计数」；配套 `exit_loop` 工具（Run 内 `ctx.Actions().Escalate = true`）。

**Workflow 图引擎**（`workflow/`）：

- `Node` 接口（`workflow.go:35`）：Name/Description/Config/InputSchema/OutputSchema/ValidateInput/ValidateOutput/`Run(ctx, input) iter.Seq2`；`Edge{From, To, Route}`；**路由是数据驱动的**——节点通过 `Event.Routes []string` 发路由值，边用 `StringRoute/IntRoute/BoolRoute/MultiRoute[T]/Default` 匹配（含 LLM 路由示例）；
- 内置节点：AgentNode（包 agent，绑 single_turn）、FunctionNode（泛型 Go 函数，从 Go 类型自动推导 JSON Schema）、ToolNode、JoinNode（fan-in 屏障）、DynamicNode（体内可命令式 `RunNode` 派发子节点）、ParallelWorker（列表逐项并发）、WorkflowNode（子图嵌套）；
- **scheduler**（`scheduler.go:553`）：每节点一 goroutine 推事件进容量 16 的 eventQueue，单消费者 drain → 应用状态副作用 → yield → 调度后继；含 pendingActivation 并发上限、retryItem 定时重试、draining/cancelAll/consumerGone 取消状态机、finalize 校验"至多一个终端节点产出输出"；
- **NodeStatus 状态机**（`state.go:25`）：`Inactive→Pending→Running→Completed/Waiting/Failed/Cancelled`；NodeWaiting 覆盖 HITL 暂停与 fan-in 未齐两义；Completed 可被重新触发（图成环即循环）；
- **RunState 持久化**：每 invocation 的全图状态（每节点 Status/Input/Output/Branch/Interrupts/Attempt）经 `persistence.go` 序列化进 **session.State**，跨轮 pause/resume——与 eino 的独立 CheckPointStore 不同，复用 session 存储通道。

**双向桥接**：`workflowagent.New` 把 Workflow 适配成 Agent（可作 root/sub-agent）；`workflow.NewAgentNode` 把 agent 包成节点。即 **agent 与图互为容器**，任意嵌套。

### 4.6 Session / State / Event：可序列化的厚事件协议

- **State 三作用域前缀**（`session/session.go:416`）：`app:`（应用级共享）/ `user:`（跨会话用户级）/ `temp:`（本次 invocation 内，结束丢弃）；状态变更以 **StateDelta**（EventActions 上的 map）随事件声明，session 服务 AppendEvent 时合并、temp 键不入历史——**"事件即状态变更日志"**，回放天然一致；
- **Event**（`session.go:100`）：内嵌 LLMResponse + `InvocationID/Branch/IsolationScope/Author` + `Actions EventActions{StateDelta, ArtifactDelta, TransferToAgent, Escalate, SkipSummarization, RequestedToolConfirmations, Compaction}` + `LongRunningToolIDs` + `Routes` + `RequestedInput`（HITL 暂停信号）+ `Output any` + `NodeInfo{Path, MessageAsOutput}`；**Event JSON 与 adk-python 互操作**；
- **流式增量**：`LLMResponse.Partial` 标记流式分片，Runner 只对非 partial 事件 AppendEvent/触发插件；`stream_aggregator.go` 把流式文本与流式 function call（PartialArg JSON-path 重组）聚合为最终事件；
- **历史组装隔离**：LLM 输入历史按 **Branch 前缀匹配 + IsolationScope 精确匹配**过滤（`contents_processor.go:121`）——并行 agent 各自 branch、task agent 各自 isolation scope、chat coordinator 无 scope 看全部。**这是 adk-go 对 transfer 上下文串扰的修复方案**：不做 eino 式全账本改写，而是"事件全记、按 scope 过滤组装"。

### 4.7 Runner 双路径与 HITL 两套机制

- **Runner.Run**（`runner/runner.go:536`）：root 是 LlmAgent 走 node path（包成合成 workflow，`ReconstructRunState` 判断 resume vs run——把 msg 中命中 pending interrupt 的 FunctionResponse 变为 resume payload，复用暂停轮的 invocation id）；否则走 agent path（appendMessageToSession → 插件 BeforeRun 可早退 → range rootAgent.Run → 非Partial事件AppendEvent → yield）；收尾 compactOnce 滑动窗口摘要（Live 不做）；
- **HITL 之一（工具确认流）**：`ctx.RequestConfirmation(hint, payload)` → `adk_request_confirmation` 事件 → 客户端以 FunctionResponse 携带 ToolConfirmation 回传 → 工具恢复；未确认返回 `ErrConfirmationRequired`、拒答 `ErrConfirmationRejected`（`tool/tool.go:136` `WithConfirmation` 包装器）；
- **HITL 之二（工作流暂停/恢复）**：节点发 `Event.RequestedInput{InterruptID}` → scheduler 置 NodeWaiting → RunState 存 session.State → 用户下一轮 FunctionResponse 命中 InterruptID → `Workflow.Resume` 按 **RerunOnResume** 决定语义：重入该节点（re-entry，`ctx.ResumedInput(interruptID)` 取历史应答）或把 payload 交给后继（handoff）；chat coordinator 派发 task agent 遇暂停时**故意留 unresolved delegation**，下一轮续跑（`llm_agent_wrapper.go:570` 大段注释）；
- **AgentTool**（`tool/agenttool/agent_tool.go:105`）：独立 InMemory session/runner 运行子 agent，复制调用方 state（过滤 `_adk` 内部键）→ 取最后带 content 的事件 → OutputSchema 校验后返回——与会话内委托（task/single_turn）明确区分：完全隔离。

### 4.8 其它

- **插件**：Plugin 接口（OnUserMessage/BeforeRun/OnEvent），Compaction 记录有 fromPlugin 保护不重复处理；
- **Processor 流水线**：LLM 请求/响应两侧各一条有序 processor 链（request：basic→tools→auth→requestConfirmation→instructions→identity→compaction→contents→…→agentTransfer），每个关注点一个 processor，顺序显式声明（`base_flow.go:81`）；
- **测试设计**：`sessiontestsuite` 共享一致性测试套件（inmemory/GORM/vertexai 三实现跑同一套）；httprr 录制回放；`platform` 包注入时间/UUID 接缝（确定性测试）；
- **Parity 原则**：`AGENTS.md:353`"adk-python is the source of truth for feature behavior"，改动需引用 Python 行号；已知刻意分歧（Go 统一走图引擎、chat 模式 task 委托内联派发而非 Python 的"退出-重选"）均有注释说明。

---

## 5. 核心机制逐项对比

### 5.1 编排哲学：双轨 vs 单轨

| | eino | adk-go |
|---|---|---|
| 底层执行 | ChatModelAgent 的 ReAct 用 compose 图；多 Agent 转移用自研事件管道 | 一切（含顶层 agent）包成 workflow 图节点，统一 scheduler |
| checkpoint | 两套（compose `checkpoint` + adk `serialization`），需要衔接与字节级兼容修补 | 一套：RunState 序列化进 session.State |
| 代价/收益 | compose 提供通用 DAG/Pregel 能力（非 Agent 场景也能用），但双轨复杂度高 | 概念单一、持久化模型单一；但"agent 必须经节点包装"引入 RunNode/wrappedSession 等胶水（其 wrappedSession 种子事件插入处作者自注有架构缺陷待重构） |

**对基座的启示**：基座已有单 ReAct 主循环这一"单轨"，引入多 Agent 时**不要引入第二个执行引擎**（不建图引擎），在现有主循环上扩展——这天然避开了 eino 的双轨陷阱，接近 adk-go 的单轨收益。

### 5.2 Agent 抽象与语言风格

| | eino | adk-go |
|---|---|---|
| 接口 | 泛型 `TypedAgent[M]`（消息类型密封联合） | 最小 `Agent`（5 方法）+ `internal()` 锁实现 |
| 流 | 自研 `AsyncIterator`（框架早于 iter.Seq2 标准） | 标准库 `iter.Seq2`（Go 1.23+） |
| 树结构 | flowAgent 持 parent/subAgents 指针（Agent 与树绑定） | parentmap 剥离（Agent 是纯配置体，树在 Runner 构建期生成） |
| 泛型 | 大量泛型（Typed 全家桶） | 克制：仅 FunctionNode/schema 推导处用泛型 |

基座 Go 版本允许的话，事件流用 `iter.Seq2` 或沿用现有 WS 事件推送均可；**parentmap 式"配置体 + 建构期校验（单父、重名）"** 与基座"配置加载 → 校验 → 运行"的资源管理模式（skill/tool 注册）一致，值得直接采用。`internal()` 锁实现的取舍：牺牲第三方扩展性换安全，基座若只允许内部构造 Agent 同样适用。

### 5.3 协作模式：双方都在"收敛到 agent-as-tool"，但路径不同

| | eino | adk-go |
|---|---|---|
| transfer | 有，但官方标注 NOT RECOMMENDED（全上下文共享实证无效） | 主推之一（AutoFlow），但用 Branch/IsolationScope 隔离历史 + 纯声明式 Actions |
| agent-as-tool | **主推**（AgentTool + DeepAgent）；独立 checkpoint 域，中断跨工具边界可恢复 | 有（tool/agenttool），独立 session 完全隔离；另有会话内委托（task/single_turn Mode + finish_task/single_turn 工具自动装配） |
| Supervisor | prebuilt 有，NOT RECOMMENDED | 无专门类型（coordinator 模式由根 LlmAgent + task 子 agent 承担） |
| 编排容器 | Sequential/Parallel/Loop（无 Switch） | Sequential/Parallel/Loop + 完整图引擎（Route 数据驱动路由、Join/Dynamic/子图） |
| 并行事件账本 | laneEvents 泳道 fork/join（按时间戳合并） | branch + resultsChan/ackChan 背压 |

**演化结论的一致性值得强调**：eino 从"transfer 全共享"演进到"AgentTool 隔离委托"并官方否定前者；adk-go 保留了 transfer 但用历史过滤修复串扰、同时大力发展 Mode 委托与 AgentTool。**两家的终态都是"子 Agent 以受控边界（工具调用/委托工具）被主 Agent 调度，而非平级共享上下文"**。

### 5.4 状态与历史：共享账本 vs 作用域过滤

| | eino | adk-go |
|---|---|---|
| 存储 | runContext.Session（事件账本，内存态）+ checkpoint gob 落盘 | session（接口化存储：inmemory/GORM/vertexai + 一致性测试套件），Event 全量 Append |
| 跨 Agent 共享 | 默认全可见，他人消息改写注入（HistoryRewriter） | 默认不可见（Branch/IsolationScope 过滤），显式共享走 StateDelta（app:/user:/temp: 作用域）+ OutputKey |
| 状态变更 | runContext Values KV + checkpoint | **事件即日志**：StateDelta 随事件声明，AppendEvent 时合并，回放天然一致 |

adk-go 的"事件即状态变更日志 + 作用域前缀 + 历史组装时过滤"三件套，与基座"事件全量落 MySQL、历史回放与实时协议同形"的现有设计**高度兼容**——基座几乎可以增量式引入（给消息/事件加 branch 字段、给 run 加 delta 事件），而不需要 eino 式的整套 checkpoint 机制。

### 5.5 HITL：checkpoint 中断 vs session 内暂停

| | eino | adk-go |
|---|---|---|
| 机制 | Interrupt/Stateful/Composite + Address 寻址 + ResumeWithParams 定向恢复 | 工具确认流（FunctionResponse 回传）+ RequestedInput/InterruptID + RerunOnResume（重入/移交） |
| 持久化 | gob 序列化 RunCtx 整体（含事件账本），CheckPointStore 接口 | RunState 进 session.State，复用会话存储 |
| 嵌套语义 | CompositeInterrupt 聚合 + "Action 不越权、中断必上报"作用域规则 | chat coordinator 留 unresolved delegation 续跑；ErrParallelHITLUnsupported 显式不支持并行 HITL |
| 版本演进 | gob 线格式跨版本迁移工具（等长字节替换）——重包袱 | Event JSON 与 Python 互操作，schema 兼容相对轻 |

基座现状（`waiting_client_message` + 前端上行 + 断线补 tool_result）本质上是 adk-go "session 内暂停"路线的雏形：中断状态就在 run/session 记录里，恢复由新消息驱动。**建议沿用 adk-go 路线**，把"等待点"从单 Agent 工具调用泛化为「可寻址的中断（agent 路径 + toolUseId）」，而不是引入 gob checkpoint。

### 5.6 取消、容错、可观测

| | eino | adk-go |
|---|---|---|
| 取消 | CancelMode 安全点位掩码 + 递归取消 + 中断吸收语义（并发信号合并的明确规则） | iter break 逐层传播 + scheduler draining 状态机 + Parallel 显式 cancel+drain |
| 模型容错 | retry + failover 包装器链（ShouldRetry 决策对象、流式部分消息可见、attempt 上下文） | 无专门层（由 workflow NodeConfig.RetryConfig 提供节点级重试） |
| 可观测 | callbacks（handler 收独立事件流拷贝）+ DesignateAgent 过滤 + 中间件 SendEvent | telemetry span 每层 + 插件 OnEvent + partial/非partial 事件分级 |
| 上下文管理 | summarization/reduction/patchtoolcalls 中间件 | Compaction 插件 + sliding window + SkipSummarization 事件标志 |

eino 的**取消安全点**（只在模型调用后/工具批完后这些一致点取消，避免流中途撕裂）与**中断吸收语义**，对基座的 cancel.go/断线处理是直接可借鉴的精细化方向；其 **retry/failover 包装器链**比基座现有 model_failover 多了"流式部分输出可见 + 决策对象化 + 事件可观测"三点，值得对照升级。

---

## 6. 对基座的借鉴建议（分期路线）

### 设计总纲

结合两家终态共识与基座现状，多 Agent 能力的形态建议为：

> **主 Agent（现有 ReAct 引擎）+ 子 Agent 声明为受控资源（skill/工具组合/提示词配置）+ 以 Meta Tool 形式暴露给主 Agent 调度 + 子 Agent 以 scoped run 隔离执行 + 事件带路径冒泡 + 跨 Agent 状态走显式 delta 通道。**

不引入：图引擎（一期/二期均不需要；eino 的双轨教训 + adk-go 的胶水代价都说明其成本高）、平级 transfer 全共享（eino 官方否定）、独立 checkpoint 体系（与基座 MySQL 落库路线冲突）。

### 第一期：最小多 Agent（主 Agent + 委托工具 + 事件路径）

1. **事件协议扩展**（借鉴 adk-go Event.Author/Branch + eino RunPath）：现有事件加 `agentPath []string`（如 `["main","researcher"]`）与 `author` 字段；`step` 保持不变。历史回放协议同步扩展——基座"回放与实时同形"的原则在此延续。
2. **Agent 定义与注册**（借鉴 parentmap + 基座资源管理模式）：`agent` 成为与 skill/tool 同级的注册类资源（表 + conf），字段：name/description/instruction/tools/modelKey/子工具集/输出契约（InputSchema/OutputSchema）。运行期校验重名。parentmap 式"构建期校验、运行期只读"。
3. **子 Agent 即 Meta Tool**（借鉴 AgentTool + installTaskTools）：在 `meta_tools.go` 注册表为每个可用子 Agent 自动生成一个工具（工具名 = agent 名，描述 = agent description，参数 schema = agent InputSchema 或默认 `{request: string}`）——与现有 `get_tool/execute_tool` 两段式加载完全同构：`list_tools` 可发现、`get_tool` 可取 schema、`execute_tool` 触发执行。**这一步之后基座自然具备 DeepAgent 形态**（create_plan + todo_write + task 委托 = eino prebuilt/deep 的等价物）。
4. **scoped 子 run 执行**（借鉴 AgentTool 独立 runner + adk-go isolation scope）：`execute_tool` 命中 agent 工具时，起一个子 run（新 runID，parentRunID 指向父，branch = 父 branch + agent 名）；子 run 的 LLM 历史只含自己的消息 + 父传入的 request（不共享父全量历史——eino 官方实证教训）；子 run 结束取最终回复（OutputSchema 校验后）作为工具结果回填父循环。事件冒泡：子 run 事件带完整 agentPath 直接进父的 WS 事件流（eino EmitInternalEvents 模式），**但不进父的 LLM 历史**。
5. **取消与断线传播**：父 run 取消时级联取消子 run（参考 eino WithRecursive 的"各后代各自落终态、根包含全貌"）；复用基座现有 cancel/断线补 tool_result 机制。

一期验收形态：用户与主 Agent 对话，主 Agent 通过工具调用把子任务委托给配置好的子 Agent，前端能看到嵌套事件流（按 agentPath 分卡片渲染），子 Agent 的 HITL（ask_question）事件冒泡到顶层等待用户作答、作答后子 run 继续。

### 第二期：编排容器 + 跨 Agent 状态 + 嵌套 HITL 完善

6. **编排容器**（借鉴两家三件套，实现都很薄）：Sequential/Parallel/Loop 作为一种特殊的 agent 资源类型；Parallel 重点抄两个作业——**eino laneEvents 泳道**（子 run 事件免锁追加、合并时按时间戳排序）或 **adk-go ackChan 背压**（子发事件等消费者确认再继续，防止事件洪峰撑爆 WS），二选一即可；
7. **跨 Agent 状态通道**（借鉴 StateDelta + OutputKey + 三作用域）：事件增加 `stateDelta map[string]any`，落库时合并进 session/run 级 KV；作用域建议直接采纳 `app:/user:/temp:` 前缀命名（语义清晰、零成本）；`OutputKey` 让子 Agent 的产出可被后续节点按名引用（编排容器必需）；
8. **嵌套 HITL 寻址**（借鉴 eino Address + adk-go InterruptID）：waiting 点从「run 级 pending_tool_use_ids」升级为「(agentPath, toolUseId) 二元组」；前端上行 tool_use_answer 时按 agentPath 路由到对应子 run；RerunOnResume 的"重入/移交"二分语义值得在协议里预留。

### 第三期（视业务需要）：深化方向

9. **Mode 体系**（借鉴 adk-go installTaskTools）：子 agent 声明 `task`（需 finish_task 显式交还）或 `single_turn` 模式，引擎按 Mode 自动装委托工具——当"多轮任务型子 agent"成为需求时引入；
10. **中间件化改造**（借鉴 ChatModelAgentMiddleware）：把基座的上下文压缩（runtime_support）、token 估算、未来需要的 summarization 从引擎硬编码改为 Before/After 钩子链；
11. **模型容错升级**（借鉴 eino retry/failover 包装器链）：决策对象化（ShouldRetry 返回 Decision 可改输入/控延迟）、流式部分输出可见、重试作为事件可观测；
12. **取消安全点精细化**（借鉴 CancelMode）：把"取消只发生在模型调用后/工具批完成后"固化为安全点语义，明确取消与业务中断并发时的吸收规则；
13. **确定性测试基建**（借鉴 adk-go platform 接缝 + sessiontestsuite）：时间/UUID 注入接缝、存储实现一致性测试套件——基座多了一层嵌套 run 之后测试复杂度会跳涨，提前铺垫。

### 明确不建议引入的

| 不引入 | 理由 |
|---|---|
| 通用图引擎（compose/Pregel 或 workflow scheduler） | 基座是会话式 Agent 服务，无独立确定性流水线诉求；两家代码均证明其带来巨大的调度/持久化/测试复杂度；编排三件套用循环即可覆盖 90% 场景 |
| 平级 transfer 全上下文共享 | eino 官方实证无效并标注 NOT RECOMMENDED；adk-go 保留它付出了 Branch/IsolationScope/wrappedSession 整套修复成本 |
| gob checkpoint 整体序列化 | 与基座"事件全量落 MySQL、恢复靠重放"路线冲突；且 eino 的线格式迁移负担（等长字节替换 v0.8 旧格式）是前车之鉴 |
| 泛型 Typed 全家桶 | 基座消息类型单一，收益低 |

---

## 7. 附录：关键源码速查

### eino（`../eino`）

| 主题 | 位置 |
|---|---|
| Agent 接口 / AgentEvent / AgentAction | adk/interface.go:453 / :419 / :357（作用域规则 :346） |
| ReAct = compose 图 | adk/react.go:326,354 |
| flowAgent（子 Agent 树/转移/历史改写） | adk/flow.go:42,75,188,275,481 |
| transfer_to_agent 工具 / 合成消息 | adk/chatmodel.go:583-678 / adk/utils.go:96 |
| AgentTool | adk/agent_tool.go:93,154（bridgeStore: interrupt.go:369） |
| Sequential/Parallel/Loop | adk/workflow.go:694-714（泳道: runctx.go:423,478） |
| Supervisor / DeepAgent / Plan-Execute | adk/prebuilt/{supervisor,deep,planexecute}/ |
| Runner / 定向恢复 | adk/runner.go:55,102-167 |
| 中断体系 / checkpoint 序列化 | adk/interrupt.go:88-189,210-249 |
| 取消（安全点/递归/吸收） | adk/cancel.go:43,149,162；react.go:402,476 |
| TurnLoop（抢占/停止） | adk/turn_loop.go:879,1056,1225 |
| 中间件接口 | adk/handler.go:139 |
| retry / failover | adk/retry_chatmodel.go:222 / adk/failover_chatmodel.go:128 |
| compose checkpoint / Pregel | compose/checkpoint.go:108 / compose/graph_run.go:108 |
| 官方"NOT RECOMMENDED"立场 | adk 各文件注释（如 flow.go、prebuilt/supervisor） |

### adk-go（`../adk-go`）

| 主题 | 位置 |
|---|---|
| Agent 接口 / 构造 / Run 包装 | agent/agent.go:44,56,163,218 |
| 三层运行模型注释（必读） | agent/context.go:28-62 |
| InvocationContext / Context | agent/context.go:63-119,142-240 |
| LlmAgent / installTaskTools / Mode | agent/llmagent/llmagent.go:39,139,344；internal/llminternal/mode.go:92 |
| LlmAgent 节点三形态（chat/task/single_turn） | agent/llmagent/llm_agent_wrapper.go:76,537,565,680 |
| Sequential / Parallel / Loop | agent/workflowagents/{sequentialagent,parallelagent,loopagent}/agent.go |
| transfer_to_agent（AutoFlow） | internal/llminternal/agent_transfer.go:69,102,188,312 |
| instruction/state 注入 | internal/llminternal/instruction_processor.go:41,121,204 |
| 图引擎（Node/Edge/Route） | workflow/workflow.go:35,63,126,150,238 |
| scheduler / NodeStatus / RunState | workflow/scheduler.go:553 / workflow/state.go:25,86,158 |
| Runner 双路径 / findAgentToRun | runner/runner.go:536,1151；runner/run_node.go:66,326 |
| Event/EventActions/State 作用域 | session/session.go:100,184,249,416 |
| HITL（确认流/暂停恢复） | tool/tool.go:136-237；workflow/hitl_test.go；workflow/persistence.go |
| AgentTool | tool/agenttool/agent_tool.go:58,105 |
| parentmap | internal/agent/parentmap/map.go |
| 仓库自带阅读指南 | docs/reading-guide.md（中文） |
