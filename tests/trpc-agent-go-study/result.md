# 测试结果：tRPC-Agent-Go 多智能体代码学习

> 场景见同目录 `scenario.md`；完整回放见 `replay-session.md`。

## 结论

测试成功。父智能体同轮并行委派三个子智能体，全程仅用只读检索 MCP 工具 + 代码检索策略 Skill，在约 22 分钟内完成对 tRPC-Agent-Go（3155 个 Go 文件）三大主题的学习并产出三份有代码佐证的报告与一份汇总。

## 执行统计

| 指标 | 值 |
|---|---|
| 事件总数（原始流） | 38810 |
| 工具调用总数 | 157 |
| delegate_agent 委派 | 3（同轮并行发出） |
| run 数 | 4（1 父 + 3 子） |
| done 事件 | 4 |

### 工具调用分布

```
{
 "get_skill": 4,
 "todo_write": 23,
 "delegate_agent": 3,
 "get_tool": 23,
 "repo_list_repositories": 2,
 "repo_get_repo_map": 3,
 "repo_search_code": 7,
 "repo_list_files": 35,
 "repo_get_file_symbols": 35,
 "repo_read_file": 18,
 "repo_find_symbol": 4
}
```

### Token 消耗（4 个 run 的 done 事件）

| run | outputTokens | cacheRead | contextUsed |
|---|---|---|---|
| run_f3dc6c59f9dd4e | 10595 | 845952 | 1338 |
| run_e0c96446280b4b | 13504 | 565120 | 3402 |
| run_c5fbb5c3c9e645 | 14810 | 743808 | 1424 |
| run_738d2a7b6b9e44 | 8760 | 21888 | 1492 |

## Skill 与工具链路验证

- **Skill 触发**：父 run 首步即 `get_skill(name=代码检索策略)`，三个子代理各自 run 内也独立加载了该 Skill（get_skill ×4），说明子 run 正确继承了 caller 级 Skill 注册表；
- **两段式工具**：`get_tool` ×23 后接 `execute_tool` 执行，符合注册表工具的加载协议；
- **Skill 工作流遵从度**：子代理的 todo 轨迹显示「加载 Skill → repo_map 总览 → 符号/文本定位 → read_file 精读」的推荐顺序被实际执行；
- **并行委派**：三个 delegate_agent 的 tool_use_start 事件序号连续（同轮），证实 max_parallel=3 生效。

## 效果评估

**做得好的**：
1. 报告结论全部带文件路径佐证（如 `memory/memory.go`、`agent/transfer_controller.go`、`team/team.go`、`runner/runner.go`），可复核；
2. 检索行为符合 Skill 引导——没有出现「递归列全仓库/大段读文件」反模式，read_file 全部为小范围调用；
3. `get_file_symbols` 成为最高频定位工具（35 次），说明声明级符号检索比文本撒网更被模型偏好，验证了 AST 增强的价值；
4. 记忆系统子代理正确识别了「Memory/Entry/Kind/Metadata/Service/Reader 接口」核心抽象。

**可改进的**：
1. `find_symbol` 使用率偏低（4 次）而 `list_files` 偏多（35 次）——子代理倾向逐目录浏览，可在 Skill 中更强调「先 find_symbol 再定向」；
2. `search_pattern`（ast-grep 风格）未被使用——新工具知名度不足，说明 **Skill 是推广新工具的正确位置**；
3. 父汇总压缩比较狠（三份报告 → 2952 字），如需保留细节应让子代理报告全文另行归档（本文件已归档）。

## 子智能体报告全文

---

### code-orchestration-learner 报告

# trpc-agent-go 多智能体设计分析报告

## 1. Agent 抽象与生命周期
- **核心接口**：`Agent`（`trpc-agent-go/agent/agent.go:62`）仅含 5 个方法：`Run(ctx, *Invocation) (<-chan *event.Event, error)`、`Tools()`、`Info()`、`SubAgents()`、`FindSubAgent(name)`——把“树形子代理关系”（`SubAgents/FindSubAgent`）直接内建进接口。`Info`（agent.go:23）携带 Name/Description/InputSchema/OutputSchema；`SubAgentSetter`（agent.go:91）支持运行期动态刷新子代理。
- **执行上下文**：`Invocation`（`agent/invocation.go:123`）是一次运行的全部状态：Agent、`InvocationID`、`Branch`（执行链轨迹）、`Session/SessionService`、`Model`、`TransferInfo`、`parent *Invocation`、`state map`、`MaxLLMCalls/MaxToolIterations` 计数限流。
- **生命周期**：Runner 构造根 Invocation（`NewInvocation`，invocation.go:1528）→ Agent `Run` 时 `setupInvocation` 绑定身份/模型/限额（`llmagent/llm_agent.go:1741`）→ 内部 flow 产出事件流 → 遥测包装与 AfterAgent 回调（llm_agent.go:1540 `Run`）。子代理通过 `Invocation.Clone`（invocation.go:1559）派生：新 ID、`parent` 回链、Branch 追加子代理名、计数器清零。

## 2. 编排调度模型
四种正交编排器，全部自身实现 `Agent` 接口（可任意嵌套组合）：
- **顺序**：`ChainAgent.executeSubAgents`（`agent/chainagent/chain_agent.go:211`）按序运行子代理，逐个消费完事件流再切下一个，链内转发最终响应与 token 统计。
- **并行**：`ParallelAgent.startSubAgents`（`agent/parallelagent/parallel_agent.go:143`）每个子代理一个 goroutine + `WaitGroup`，带 panic recover；`mergeEventStreams`（:313）多路归并输出。
- **循环**：`CycleAgent.runSubAgentsLoop`（`agent/cycleagent/cycle_agent.go:266`），`maxIterations` 与 `escalationFunc` 控制退出。
- **图**：`GraphAgent`（`agent/graphagent/graph_agent.go:40`）包装 `graph.Graph + graph.Executor`，支持 checkpoint/interrupt/time-travel。
上层另有 `Team`（`team/team.go:32`）：`ModeCoordinator`（协调者+成员工具）与 `ModeSwarm`（`runSwarm`，:257，经 session 状态跨请求记住活跃成员）。顶层 `Runner`（`runner/runner.go:233`）负责会话管理、事件循环与持久化。

## 3. 委派与子任务机制
双模式委派（`agent/invocation.go:102-108` 显式定义 TriggerType）：
- **子任务式**：`tool/agent/agent_tool.go:372` 把任意 Agent 包装成工具；调用时 `callWithParentInvocation`（:482）克隆子 Invocation 运行，`collectResponse`（:1151）回收最终响应，作为 tool result 返回给父 LLM 合并。
- **交接式**：`transfer_to_agent` 触发 `TransferInfo`，由 `internal/flow/processor/transfer.go` 消费切换控制权；`TransferController`（`agent/transfer_controller.go:27`）提供 OnTransfer 钩子。

## 4. 通信与上下文
- **通信=事件流**：Agent 间无直接消息传递，统一通过 `<-chan *event.Event` 向上转发（ChainAgent:258 逐事件 `EmitEvent`）；`ParentMetadata`（invocation.go:133）注入事件，使下游能关联“哪个父动作产生了这个子事件”（并行分支用 TriggerID 消歧）。
- **共享与隔离并存**：`Clone` 共享 Session/SessionService/artifact 等服务与 `noticeChannels`（invocation.go:1576-1577，跨代理同步信号），但 `state` 经 `cloneState` 深拷贝隔离，`Branch` 区分轨迹；AgentTool 还提供 `HistoryScopeIsolated/ParentBranch` 会话历史隔离粒度（agent_tool.go:264）。

## 设计评价
1. **接口极简 + 组合式编排**：5 方法 Agent 接口让顺序/并行/循环/图编排器都是普通 Agent，可自由嵌套成任意树/图，扩展成本低。
2. **Clone 派生协议是精髓**：一次性解决父子上下文继承（共享 Session、隔离 state、Branch 追踪、计数清零），是所有委派/编排的统一底座，但也使 Clone 语义偏重、易被误用。
3. **委派语义显式化**：ToolCall（子任务）与 Transfer（交接）用 TriggerType/ParentMetadata 显式建模并可追溯，比隐式路由更可观测，代价是 runner/flow 侧需大量事件去重与补全逻辑（runner.go 事件循环复杂度高）。

---

### code-arch-learner 报告

# tRPC-Agent-Go 架构分析报告

> 仓库：`trpc-agent-go`（trpc.group/trpc-go/trpc-agent-go，纯 Go）。以下结论均来自镜像代码实际阅读，附文件路径佐证。

## 一、模块地图

| 模块 | 职责 |
|---|---|
| `runner/` | 运行入口：会话装配、Agent 选择、事件循环与持久化（`runner.go`） |
| `agent/` | Agent 抽象与实现：`llmagent/`（核心 LLM Agent）、`chainagent|parallelagent|cycleagent|graphagent/`（多 Agent 编排）、`a2aagent/`、`taskrun/`、`dify|n8n|claudecode|codex/`（外部 Agent 适配）、`invocation.go`（运行上下文）、`callbacks.go`、`run_with_plugins.go` |
| `internal/flow/` | LLM 执行引擎：`flow.go`（Request/ResponseProcessor 接口）、`llmflow/`（主循环）、`processor/`（30+ 处理器切片）、`calllimit/` |
| `model/` | 模型层：`model.go` 接口 + openai/gemini/anthropic/bedrock/hunyuan/ollama 等供应商适配，`registry.go`（上下文窗口注册）、`failover/`、`hedge/` |
| `tool/` | 工具层：`tool.go` 接口 + `function/`、`mcp/`、`agent/`（Agent 即工具）、`transfer/`、`codeexec/`、`webfetch/`、`skill/`、`permission.go`、`toolset.go` |
| 状态与记忆 | `session/`（Service + inmemory/sqlite/postgres/mysql/mongodb/redis/clickhouse/pgvector 后端 + summary）、`memory/`、`artifact/`、`knowledge/`（RAG：chunking/embedder/vectorstore/retriever/reranker） |
| 扩展与生态 | `team/`（swarm 协作）、`skill/`、`evolution/`、`evaluation/`、`planner/`、`codeexecutor/`、`graph/`（类型安全图工作流：state_graph/executor/checkpoint/interrupt） |
| 服务与横切 | `server/`（a2a/agui/openai/trpcagent 对外协议）、`telemetry/`、`plugin/`、`event/`、`internal/`（约 45 个内部支撑包） |

## 二、核心抽象清单

- **Agent**（`agent/agent.go:62`）：`Run(ctx, *Invocation) (<-chan *event.Event, error)` + `Tools/Info/SubAgents/FindSubAgent`——一切 Agent（LLM、工作流、远程）的统一契约。
- **Invocation**（`agent/invocation.go:123`）：单次运行上下文，携带 Agent/Session/SessionService/Model/Message/RunOptions/Plugins/Memory/Artifact 服务及 `MaxLLMCalls/MaxToolIterations` 限额。
- **Model**（`model/model.go:45`）：`GenerateContent(ctx, *Request) (<-chan *Response, error)`；`IterModel`（:66）提供 `Seq[*Response]` 迭代器形态。
- **Tool**（`tool/tool.go:17-41`）：`Tool{Declaration()}` → `CallableTool{Call(ctx, jsonArgs)}` / `StreamableTool{StreamableCall}` 三级拆分，声明与执行分离。
- **Flow/Processor**（`internal/flow/flow.go:22-38`）：`Flow{Run}`、`RequestProcessor{ProcessRequest}`、`ResponseProcessor{ProcessResponse}`。
- **Runner**（`runner/runner.go:233`）：`Run(ctx, userID, sessionID, message)` 返回事件流；`ManagedRunner`（:254，Cancel/RunStatus）、`SteerableRunner`（:271，运行中注入消息）逐级增强。
- **Session/Service**（`session/session.go:63, 1111`）：StateMap + Events + Tracks + Summaries；Service 含 `AppendEvent/CreateSessionSummary/EnqueueSummaryJob` 等。
- **Event**（`event/event.go:99`）：内嵌 `*model.Response`，附加 `InvocationID/Author/Branch/StateDelta/LongRunningToolIDs`，是全链路唯一消息载体。

## 三、分层与数据流

分层自上而下：**服务层**（server/ 协议适配）→ **运行层**（runner：装配+持久化+事件出口）→ **编排层**（chain/parallel/cycle/graph/team 组合 Agent）→ **Agent 层**（llmagent + callbacks/plugins）→ **Flow 引擎**（processor 流水线）→ **能力层**（model/tool/knowledge/memory）→ **状态层**（session/artifact）。

一次请求执行路径（均已在代码中核实）：
1. `Runner.Run`（`runner/runner.go:544`）：getOrCreateSession(:620) → selectAgentForRun(:653) → 构造 Invocation(:689) → 注册运行(:719) → 持久化本轮消息(:744)；
2. `agent.RunWithPlugins`（`agent/run_with_plugins.go:66`）：BeforeAgent 回调 → `ag.Run`(:94) → AfterAgent 包装(:98)；
3. `LLMAgent.Run`（`agent/llmagent/llm_agent.go:1540`）→ `executeAgentFlow`(:1640) 驱动 `Flow.Run`（`llmflow/llmflow.go:200`）；
4. `Flow.Run` 起 goroutine 循环 `runOneStep`(:677)：请求处理器链（instruction 注入、会话历史、工具装配）→ `model.GenerateContent` 流式调用(:2489) → 响应处理器（流聚合、**functioncall 处理器执行工具并回填结果**）→ 直到 `IsFinalResponse/EndInvocation`(:308)；
5. 事件回流 `runner.processAgentEvents`(:1461)：事件按 `StateDelta` 落库（sessionService.AppendEvent）、插件过滤、最终补发 runner completion 事件。

## 四、架构评价

1. **单流事件总线是全框架的脊柱**：从 Flow 到 Runner 全部用 `<-chan *event.Event` 传递，流式输出、工具调用、状态写入共管道，Runner 侧统一去重/持久化/收尾，契约极简但可组合性强；代价是 `runner.go`（4500+ 行）事件循环复杂度集中。
2. **处理器流水线是核心扩展点**：ReAct 循环被拆成可插拔的 Request/ResponseProcessor（`internal/flow/processor/`），按 Agent 特性在 `buildRequestProcessorsWithAgent`（`llm_agent.go:300`）动态装配，符合开闭原则。
3. **接口先行 + Invocation 上下文贯通**：Session/Model/Tool/Memory/Artifact 均为小接口多后端可替换；多 Agent 编排（链/并/环/图/swarm）全部收敛到同一个 `Agent` 接口，工作流 Agent 与 LLM Agent 可无差别嵌套组合。

---

### code-memory-learner 报告

# trpc-agent-go 记忆系统设计分析报告

## 一、记忆数据模型

核心定义在 `memory/memory.go`：

- **Entry**（L244-252）：`ID / AppName / Memory *Memory / UserID / CreatedAt / UpdatedAt / Score`，以 `(AppName, UserID, MemoryID)` 三元组 `Key`（L255）寻址，天然支持多应用/多用户隔离；
- **Memory**（L231-241）：`Memory string`（正文）+ `Topics []string` + `LastUpdated` + 可选元数据 `Kind`（`fact`/`episode` 二分类，L219-227）、`EventTime`、`Participants`、`Location`（`Metadata` L55-60）——比 ADK 原版多出情景记忆维度；
- **接口抽象**（L162-212）：`Reader`（`ReadMemories`/`SearchMemories`）+ `Service`（增删改 + `Tools()` + `EnqueueAutoMemoryJob`），读写分离；
- **SearchOptions**（L283-325）：含 `SimilarityThreshold`、`KindFallback`、`Deduplicate`、`HybridSearch`、`HybridRRFK`，暗示混合检索能力。

## 二、写入 → 检索 → 注入链路

**写入（双通道）**：
1. **Agentic 工具**：6 个工具 `memory_add/update/delete/clear/search/load`（L24-29，实现在 `memory/tool/tool.go` NewAddTool 等），经 `GetMemoryServiceFromContext`（L477）取服务写入；auto 模式下默认隐藏写工具、仅暴露 `memory_search`（`memory/internal/memory/memory.go` BuildToolsList）。
2. **自动提取**：Runner 在本轮结束后调用 `EnqueueAutoMemoryJob` → `AutoMemoryWorker`（`memory/internal/memory/auto.go` L262，多 channel worker 池）→ `searchRelevantMemories` 取旧条目 + `MemoryExtractor.Extract`（LLM 驱动，`memory/extractor/extractor.go` L29-56，产出 Add/Update/Delete/Clear Operation）→ `reconcileOps` 去重门控后落库。增量游标存于 session state `memory:last_extract_at`，`scanDeltaSince` 只提取新消息；消费过外部上下文的会话标记 `polluted` 被跳过（memory.go L33-42）。

**检索（关键词为基座 + 可选向量混合）**：共享引擎在 `memory/internal/memory/memory.go`——gse 中文分词 + BM25（L69-70）+ content/topics 字段加权 + 短语加分；向量后端（sqlitevec/pgvector/mysqlvec/chromadb，embedding 由 embedder 生成）的稠密结果经 `MergeHybridResults`（L1504）做 RRF 融合，`DeduplicateResults`（L1566）按 token Jaccard 去重。

**注入（推拉结合）**：拉取——`memory_search/load` 工具由模型主动调用；推送——`internal/flow/processor/content.go` 的 `ContentRequestProcessor`（L157-240）携带 `PreloadMemory`/`PreloadMemoryPlaybook`，`loadPreloadMemoryMessage`（L4200）按 limit `ReadMemories` 或 `getAdaptivePreloadMemoryMessage`（L4150）预算感知地按 query 搜索，再按 `System`（`injectSystemContextMessage` L1054）或 `User`（`prependUserContextMessage` L1366）模式注入 prompt，读取面收敛为只读 `inv.MemoryReader`（`agent/invocation.go` L174）。

## 三、持久化机制

11 个后端：`inmemory / redis / sqlite / sqlitevec / postgres / pgvector / mysql / mysqlvec / chromadb / mem0 / tencentdb`，各为独立 Go module（如 `memory/mysql/go.mod`），依赖隔离干净。inmemory 为嵌套 map + RWMutex（`inmemory/service.go` L30-33）；sqlitevec 存 embedding blob + topics JSON，支持软删除（`deleted_at`）与更新轮转 `rotateMemory`（L488）；统一由 `memory.Service` 接口抽象，自动提取链路对后端无感。

## 四、压缩与淘汰策略

- **无压缩/摘要**：条目本身不做摘要压缩（session 包的 compaction 属会话上下文管理，与长期记忆分离）；**无 TTL**（memory 包内未检索到 TTL 逻辑）。
- **去重门控代替淘汰**：`reconcileOps` 用双信号阈值（BM25 Score≥0.90 或 Jaccard≥0.70 → skip；≥0.60/0.40 → 改写为 update；probe 下限 0.30，auto.go L72-127）抑制重复条目增长；
- **硬容量上限**：sqlitevec `enforceMemoryLimitTx`（L321-350）超限时**直接拒绝写入报错**，并非 LRU/TTL 淘汰；另有显式 `ClearMemories`/`memory_clear` 兜底。

## 五、设计评价

1. **接口契约驱动的高可插拔性**是最大亮点：单一 `Service` 接口 + 11 后端分模块，向量/关系/托管服务随意切换，且关键词基线保证无嵌入器也能工作；代价是接口偏胖（读写/工具/异步全塞一起），mem0 等外部平台难以完整实现契约。
2. **写入质量控制务实**：自动提取链路的"LLM 生成 + 双信号 reconcile 门控"设计使记忆增长可控且不依赖向量可用性；但"超限即拒绝写入"的策略偏保守，缺乏衰减/淘汰，长期运行依赖 LLM 提取质量与人工清理。
3. **检索与注入的中文化适配到位**（gse 分词、CJK trigram、RRF 混合），preload 提供 System/User 双注入位 + playbook 模板 + 预算自适应，工程完成度高；不足是 preload 与 agentic 检索未打通统一的相关性排序口径。

---

## 父智能体最终汇总

三个子智能体的学习报告已全部返回，以下是汇总后的最终学习总结。

# trpc-agent-go 学习总结（协调者汇总）

## 一、架构设计（code-arch-learner）

**模块划分**：约十大模块分层清晰——`runner/`（运行入口：会话装配、Agent 选择、事件循环与持久化）、`agent/`（Agent 抽象 + llmagent 核心实现 + 编排型 Agent + 外部 Agent 适配）、`internal/flow/`（LLM 执行引擎与 30+ 可插拔处理器）、`model/`（OpenAI/Gemini/Anthropic 等多供应商适配）、`tool/`（function/mcp/agent-as-tool）、状态与记忆（session/memory/artifact/knowledge-RAG）、`server/`（a2a/agui/openai 等对外协议）及 telemetry/plugin 等横切能力。

**核心抽象**：五个小接口构成骨架——`Agent`（`agent/agent.go:62`，Run + Tools/Info/SubAgents）、`Invocation`（单次运行上下文，携带全部服务引用与限额）、`Model`（流式 GenerateContent）、`Tool`（声明/执行/流式三级拆分）、`Event`（内嵌 model.Response 的全链路唯一消息载体）。

**分层与数据流**：服务层 → 运行层→ 编排层（chain/parallel/cycle/graph）→ Agent 层→ Flow 处理器流水线 → 能力层→ 状态层。一次请求路径：`Runner.Run` 装配会话与 Invocation → `LLMAgent.Run` 驱动 `Flow.Run` → 请求处理器链（指令注入/历史/工具装配）→ 流式调模型 → 响应处理器执行工具并回填 → 事件流经 Runner 按 StateDelta 落库输出。

## 二、多智能体设计（code-orchestration-learner）

**Agent 抽象与生命周期**：接口仅 5 个方法，却把树形子代理关系（`SubAgents/FindSubAgent`）内建其中；生命周期以 `Invocation` 为载体——Runner 构造根 Invocation → `setupInvocation` 绑定身份/模型/限额 → 产出事件流；子代理经 `Invocation.Clone` 派生（新 ID、parent 回链、Branch 追加、计数清零）。

**编排调度模型**：四种正交编排器（Chain 顺序逐个消费事件流、Parallel goroutine+WaitGroup 多路归并、Cycle 迭代上限+escalation 退出、Graph 支持 checkpoint/interrupt），全部自身实现 `Agent` 接口，可任意嵌套组合；上层另有 `Team`（协调者/Swarm 模式）。

**委派与通信隔离**：双模式委派——子任务式（`tool/agent` 把 Agent 包装成工具，克隆子 Invocation 运行、回收结果作为 tool result）与交接式（`transfer_to_agent` 切换控制权）；Agent 间无直接消息传递，统一通过事件流向上转发，`ParentMetadata` 保证事件可追溯到父动作；上下文"共享与隔离并存"——共享 Session 服务、深拷贝 state、Branch 区分轨迹，且支持历史隔离粒度配置。

## 三、记忆系统设计（code-memory-learner）

**数据模型**：`Entry`（ID/AppName/Memory/UserID/时间戳/Score）以三元组寻址，天然多应用多用户隔离；`Memory` 含正文 + Topics + 情景元数据（Kind：fact/episode、EventTime、Participants），比 ADK 原版多了情景记忆维度；读写接口分离（Reader/Service）。

**写入→检索→注入链路**：写入双通道——agentic 工具（add/update/delete/search/load 六件套）+ 自动提取（Runner 结束后 EnqueueAutoMemoryJob → worker 池 → LLM Extractor 产出增删改操作 → 双信号阈值 reconcile 去重门控落库，增量游标只扫新消息）；检索以 gse 中文分词 + BM25 为基座，可选向量后端经 RRF 混合融合、Jaccard 去重；注入推拉结合——模型主动调 search/load 工具（拉），ContentRequestProcessor 按 System/User 模式 preload 记忆且预算自适应（推）。

**持久化与淘汰**：11 个后端（inmemory/redis/sqlite/pgvector/mysql/chromadb/mem0/tencentdb 等），各为独立 Go module 依赖隔离干净；**无压缩摘要、无 TTL**，以"去重门控 + 硬容量超限拒绝写入"代替淘汰策略，依赖显式 clear 兜底——这是相对保守的设计选择。

## 四、整体架构评价

1. **"单流事件总线 + 极简接口 + 组合式编排"是全框架的脊柱**：从 Flow 到 Runner 全部用 `<-chan *event.Event` 传递，五种编排器与 LLM Agent 收敛到同一个 5 方法 `Agent` 接口，可无差别嵌套组合，可组合性和可扩展性（处理器流水线动态装配）是其最大亮点；代价是 `runner.go`（4500+ 行）事件循环复杂度集中，去重/补全逻辑偏重。
2. **`Invocation.Clone` 派生协议是多智能体设计的精髓**：一次性解决父子上下文继承（共享服务、隔离 state、Branch 追踪、计数清零），是所有委派/编排的统一底座，但语义偏重、对使用者的心智负担较高；委派语义（ToolCall vs Transfer）显式建模，可观测性好。
3. **记忆系统"接口契约驱动 + 质量门控务实"**：11 后端可插拔、关键词基线保证无嵌入器也能工作、中文化适配（gse/RRF）到位；但缺乏衰减/TTL 淘汰与统一相关性排序口径，长期运行依赖 LLM 提取质量与人工清理——与整体"工程完成度高、策略偏保守"的风格一致。

总体而言，trpc-agent-go 是一个接口先行、组合优先、流式事件驱动的 Go 智能体框架：架构分层清晰、扩展点明确，多智能体与记忆两大子系统都构建在统一的事件流与 Invocation 上下文底座之上，工程成熟度较高，复杂度主要集中在 Runner/Flow 的事件处理层。

任务已全部完成：三个子智能体并行学习并各自产出报告，汇总如上，无需再做代码检索。