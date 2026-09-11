# 系统架构

## 1. 总体分层

```
                        ┌─────────────────────────────────────────────────┐
                        │              接入层（客户端）                    │
                        │  Web 前端(SDK) · playground · 服务间调用方 Caller │
                        └───────────────┬─────────────────────────────────┘
                                        │ WebSocket / HTTP（免鉴权，X-User-Name 可选）
                        ┌───────────────▼─────────────────────────────────┐
                        │  router + middleware（免鉴权 / 静态资源）        │
                        └───────────────┬─────────────────────────────────┘
        ┌───────────────────────────────┼───────────────────────────────┐
        ▼                               ▼                               ▼
┌───────────────┐              ┌────────────────┐              ┌────────────────┐
│ controllers/  │              │ controllers/   │              │  controllers/  │
│ http/react    │              │ http/{caller,  │              │ http/attachment│
│ (WS + 会话API)│              │ skill,tool,...} │              │ (附件上传)     │
└───────┬───────┘              └───────┬────────┘              └───────┬────────┘
        │                              │                               │
        ▼                              ▼                               ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                            service 层                                        │
│                                                                              │
│  service/react —— ReAct 运行时核心                                           │
│  ├─ runtime.go        Run 入口：鉴权/模型/提示词/工具/Skill 装配 → 建 session/run │
│  ├─ engine.go         executeReactLoop 主循环：模型轮 → 解析 tool call → 执行 → 回填 │
│  ├─ meta_tools.go     内置 Meta Tool 注册表 + 两段式业务工具加载(get_tool/execute_tool)│
│  ├─ model_failover.go 模型互备（当前模型优先，失败切换列表内其他模型）           │
│  ├─ runtime_support.go工具结果标准化 / resultRef 落库 / 上下文压缩             │
│  ├─ history.go        会话列表 + 历史事件回放（与实时协议同形）                │
│  ├─ async_task.go     异步任务提交快照 / 提醒注入 / resolve 闭环              │
│  ├─ ws.go             手写 WebSocket（RFC6455 握手、帧编解码、ping/pong）      │
│  └─ python_exec / todo / ask_question / client_tool / attachments / ...      │
│                                                                              │
│  service/{tool,skill,systemprompt,apikey,caller}  注册类资源管理               │
│  service/llmmodel + credits                       用户模型与积分              │
│  service/asynctask                                异步任务 Provider 同步框架  │
│  service/skillchatfile                            附件存储（COS）与解码       │
└──────────────┬───────────────────────────────┬───────────────────────────────┘
               ▼                               ▼
    ┌─────────────────────┐          ┌──────────────────────┐
    │ api/llm             │          │ MySQL（17 张表）      │
    │ claude / gpt / mini │          │ Redis（附件元数据缓存）│
    │ 流式 + 工具调用      │          │ COS（附件/产物存储）  │
    └─────────┬───────────┘          └──────────────────────┘
              ▼
    ┌─────────────────────┐
    │ api/pythonexec      │  Python 沙箱（python_exec 工具的执行端）
    └─────────────────────┘
```

## 2. ReAct 运行时核心链路

一次 run 的完整生命周期：

```
WS 连接 /react/ws
  └─ handleWSMessage 分发（run / cancel / tool_use_answer / client_tool_use_end）
      └─ RunWithClientReaderContext → run()
          ├─ prepareRuntimeRequest
          │    ├─ caller 校验（tblLlmCaller，需 status=1）
          │    ├─ 用户名解析（中间件注入：X-User-Name 头或默认 anonymous）
          │    ├─ 模型解析：modelHash（用户自定义模型） 或 modelKey + caller 级 ApiKey
          │    ├─ ResolveSystemPrompt（callerKey + routeValues 前缀匹配，多条拼接）
          │    ├─ FindSkillsByCallerAndRoutes → Skill 摘要索引快照
          │    └─ FindVisibleToolsByCallerAndRoutes → 工具索引快照（含白名单过滤）
          ├─ createReactRunContext（单事务）
          │    ├─ getOrCreateReactSession（FOR UPDATE 锁 + 归属校验：用户/caller/路由/类型一致才能复用）
          │    ├─ HasActiveReactRun（同一会话并发 run 互斥）
          │    ├─ 恢复上一 run 的 todo / 已加载工具快照
          │    ├─ 建 tblLlmReactRun + 持久化用户输入消息（seq=1）
          │    └─ 更新会话 last_run_id / last_message
          └─ executeReactLoop（每轮 step）
               ├─ UpdateReactRun step_index
               ├─ maybeCompactContext（token 超高水位 → LLM 语义压缩，失败回退本地摘要）
               ├─ buildToolDefinitions（只暴露稳定 Meta Tool 集合）
               ├─ callModelRound（模型互备；ChatStreamWithTools 流式）
               ├─ collectLLMStream（边收边发 thought/content 事件；空闲超时检测）
               ├─ 无 tool call → finish()：收敛 run 状态 + done 事件
               └─ 有 tool call → executeToolCalls（串行执行）
                        ├─ Meta Tool → executeInternalTool（get_tool/execute_tool/todo_write/...）
                        ├─ client 工具 → 通知前端执行 + 阻塞等待 WS 回填（run 转 waiting_client_message）
                        └─ http 工具 → 后端代理执行 → normalizeToolResult（大结果落 resultRef）
```

### 系统提示词装配

```
system = systemPrompt（注册表解析，多条按路由短→长拼接）
       + "\n\n" + 工具索引摘要（## 当前 run 可用 Business Tool 摘要索引 …）
       + "\n\n" + Skill 索引摘要（## 当前 run 可用 Skill 摘要索引 …）
```

每轮还会在上下文末尾**临时**追加（不持久化）：`<async_tasks>` 未完结任务提醒、todo 提醒。

### 两段式工具加载

1. 模型只看到工具**摘要索引**（system 前缀），不含 parameters；
2. `get_tool(toolId|name)` → 激活并返回完整 parameters/outputSchema（定义指纹落 run）；
3. `execute_tool(toolId|name|callName, input)` → 输入 JSON Schema 校验 + 指纹复核 + 可见性复核 → 执行；
4. 定义变更可自愈：execute_tool 未命中时现查最新工具，定义与上次一致则自动激活。

## 3. 周边能力

### 3.1 会话管理

- **创建/复用**：客户端传 `sessionId` 则优先锁定复用（校验 userName/callerKey/routeValues/type 完全一致，否则拒绝）；不传则新建（`session_` + UUID）。
- **并发互斥**：同一会话同时只允许一个活跃 run（事务内 session 行锁 + active run 检查）。
- **列表**：`POST /react/session/list`（callerKey + routeValues + userName 维度分页，支持标题关键词）。
- **状态**：active / archived / deleted（软删）。

### 3.2 历史回放

`POST /react/session/events`（sessionId）→ `historyEventBuilder` 把 `tblLlmReactMessage` 按时间线还原成**与实时 WS 协议同形**的事件流（run/thought/content/tool_use_start/tool_use_end/todo_update/compact_end/done/error/cancelled），前端直播与回放共用一套解析逻辑。权限：会话本人，或 playground 白名单用户。

内置回放页：`GET /react/replay?sessionId=...`。

### 3.3 异步任务

```
工具 config.async=true（异步提交型）
  → 执行成功后 recordAsyncSubmit 落 tblLlmReactAsyncTask（pending，TTL 7 天，入参/响应全量快照）
  → 后续 run 启动时 loadPendingReactAsyncTasks → <async_tasks> 提醒注入模型上下文
  → 模型用查询工具确认结果后 resolve_async_task(done|failed) 唯一清除入口
  → get_async_task 回读完整提交记录（提醒被截断时补救）
  → POST /react/async_task/list 供前端拉取任务状态（游标分页）
```

Provider 状态同步框架（`service/asynctask`）：实现 `SchedulerProvider` 接口并 `Register` 即可让第三方调度系统状态自动回写任务表；未注册任何 Provider 时自动降级为纯模型提醒模式。

### 3.4 工具大结果与产物

- 大结果（超过 `inline_limit_bytes`）只回填预览，全文落 `tblLlmReactToolResult`（resultRef），模型经 `read_tool_result` 分片读取。
- python_exec 产物（图片/CSV 等）上传 COS，元数据落 `tblLlmReactArtifact`；前端经 `GET /react/artifact/:artifactId`（IPS 鉴权）下载。

### 3.5 附件

`POST /api/chat/files/upload`（csv/md/txt，≤50MB，支持 UTF-8/UTF-16 BOM/GB18030）→ 返回 fileId；run payload 的 `attachments` 引用 fileId。模型可 `read_attachment`（读入上下文）/ `inspect_attachment`（探查表结构），python_exec 可将附件作为输入源交沙箱处理。

### 3.6 上下文压缩

`lastInputTokens`（上一轮真实 input_tokens）超过 `token_trigger` → 保留 system 前缀 + 最近若干轮，早期消息由当前模型生成语义摘要（超时/失败回退本地拼接摘要），落 `react_compact_summary` 消息并记录 coveredThrough（回放与后续上下文据此过滤已覆盖消息）。

### 3.7 模型互备

`models.mutual_failover=true` 时：当前模型失败（请求错误/流空闲超时/流中断）自动切换 `available` 列表中的其他模型，发送 `model_fallback` 事件（带 resetCurrentOutput 提示前端丢弃半截输出）；成功后该模型成为后续轮次首选。

## 4. 数据模型（17 张表）

| 域 | 表 | 说明 |
|---|---|---|
| 注册资源 | tblLlmCaller | 调用方（鉴权与资源归属主体） |
| | tblLlmSystemPrompt | 系统提示词（caller+route 前缀匹配） |
| | tblLlmTool / tblLlmToolUserPolicy | 工具及白名单 |
| | tblLlmSkill | Skill |
| | tblLlmApiKey | caller 级模型 API Key |
| 模型积分 | tblLlmUserModel | 用户自定义模型（modelHash 直引） |
| | tblLlmUserBaseCredits / tblLlmUserBonusCredits | 月度基础/赠送积分 |
| ReAct 核心 | tblLlmReactSession | 会话 |
| | tblLlmReactRun | 运行实例（状态机 + token 统计 + 快照） |
| | tblLlmReactMessage | 上下文消息（回放数据源） |
| 周边能力 | tblLlmReactToolResult | 工具大结果 |
| | tblLlmReactArtifact | python 产物元数据 |
| | tblLlmReactAsyncTask | 异步任务待办 |
| | tblLlmReactRunFeedback | 轮次反馈 |
| | tblLlmChatFileRecord | 附件记录 |

完整 DDL 见 `sql/init.sql`。

## 5. 关键设计约束

- **消息即上下文**：`tblLlmReactMessage.content_json` 保存真实进入模型的 `modelMessage`（含 tool_use parts 与 reasoning），UI 旁路数据（toolMeta）单独存放，保证回放与模型上下文一致。
- **Run 状态机**：running → waiting_client_message（等前端回填）→ finished / error / cancelled / expired；取消经 `Cancel` 置 cancelling + context 取消传播，最终 cancelled 事件由 run 收敛后发送；进程退出时活跃 run 统一 expire。
- **串行工具执行**：同一步多个工具调用按模型返回顺序串行执行，保证副作用与回填顺序稳定。
- **事件信封**：所有事件统一补 runId/sessionId/seq，经 `EventWriter` 串行写出，避免并发工具事件乱序。
- **WS 保活**：协议层 ping 每 5s + 应用层 heartbeat 事件每 20s；读侧靠 pong 刷新读超时实现断连检测。
