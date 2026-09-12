# coze-studio 学习文档：设计思想与可抽取能力

> 调研对象：`../coze-studio`（github.com/coze-dev/coze-studio 开源版，Go 后端 hertz 单体 + React 前端，四个并行探索代理完成源码调查）。
> 调研视角：react-base-service 是同语言（Go）的 ReAct Agent 基座，本文只回答两个问题——**什么设计思想值得学、什么能力可以抽取改造**。
> 与既有调研的关系：[openhands-sdk学习.md](./openhands-sdk学习.md) 学"引擎"（运行时/子代理/插件/SKILL），本文学"**产品化**"（多租户/发布/插件市场/RAG/管理后台）；且 coze 的 ReAct 循环底层正是 eino（见 [multi-agent-orchestration.md](./multi-agent-orchestration.md)）——`agentflow/agent_flow_builder.go` 用 `react.NewAgent`，工具循环本身不是它的重点。
> 撰写日期：2026-09-13。

---

## 1. 一页结论

coze-studio 的价值坐标：**它是"AI 应用平台"的产品化样板，不是"Agent 运行时"的样板**。对照基座最直接的判断：

| 维度 | coze-studio | 基座现状 | 结论 |
|---|---|---|---|
| Agent 循环/工具调用 | eino ReAct（单 Agent） | 自研 ReAct + 两段式工具加载 | 基座领先，不学 |
| 模型互备/上下文压缩/事件回放 | 无（按轮数截断历史） | 有 | 基座领先，不学 |
| MCP | **占位未实现**（`invocation_mcp.go` 返回 not implemented） | 三种传输已实现 | 基座领先，不学 |
| 多租户/空间/权限/PAT | 完整骨架 | 免鉴权 + X-User-Name | **重点学**（对应多端接入 P0） |
| 草稿→发布→渠道 分发 | draft/version/publish 三层 + connector 渠道 | caller 静态配置 | **重点学** |
| 插件市场/工具契约 | manifest + OpenAPI3 双文档、OAuth、AES 凭据 | 工具注册表 + P3 Bundle 规划 | **重点学**（直接喂 P3） |
| 知识库 RAG | 三表 + 事件驱动管线 + 可插拔检索存储 | 无 | **重点学**（未来 RAG 的最小闭环） |
| 版本可复现（commit_id 冻结） | draft/snapshot/version + 执行记录回写 | 无 | **重点学**（喂定时工作流审计） |
| 定时触发器 | **开源版没有**（商业版特性） | 已有方案 | 无可抄，自建（方案已就绪） |

**一句话：从 coze 抽"平台壳"（身份/空间/发布/插件/RAG），不从 coze 抽"引擎"。**

## 2. 八个值得抽取的设计（按对基座的价值排序）

### 2.1 双通道鉴权：请求分类器 + Session/PAT 分流（→ 多端接入 P0）

coze 的鉴权入口是一个 **RequestInspector 前置中间件**：按路由把请求分类为 WebAPI / OpenAPI / 静态资源，再分流到两条链——控制面走 Cookie session（HMAC 自签 token + DB 反查），数据面走 `Authorization: Bearer <PAT>`（个人访问令牌）；UID 统一注入请求上下文，业务层无感。

- **对基座的裁剪**：这正是 [多端服务形态接入方案.md](./多端服务形态接入方案.md) P0 缺口的现成骨架——网页/管理面板走 session，App/小程序/QQ adapter 走 PAT；`tblLlmCaller` 挂 caller 级 PAT（或直接用户级），校验后映射 userName，替换裸信任的 X-User-Name。
- **必须加固的三处**（coze 为开源简化，勿照抄）：PAT 只存 **MD5** → 改 SHA-256/bcrypt；session HMAC 密钥**硬编码** → 配置注入；token 明文仅创建时返回一次的做法**保留**。

### 2.2 Space + SpaceUser 空间模型（→ 多租户资源归属）

`User 1—N SpaceUser N—1 Space`，角色三值（owner/admin/member），注册自动建个人空间；**所有资源表冗余 space_id + creator_id**，隔离 = 查询过滤 + 归属校验。

- **对基座的裁剪**：基座的 caller 是"接入方"不是"人的组织"。若演进为多团队产品，可在 caller 之上加 space 层（资源表加 space_id 一列即可平滑迁移）；初期只保留 personal space 语义。**不建议现在做**——但建表时给未来留列的意识值得学。
- coze 的权限判定（`CheckAuthz` + 按 ResourceType 注册 `ResourceQueryer` 返回 {CreatorID, SpaceID}）是很好的**骨架**，但其实现退化为 creator-only（角色未接入），学结构不学现状。

### 2.3 草稿 → 版本快照 → 发布 → 渠道（→ 配置生命周期）

三层模型：draft（JSON 可变区，bot 全量配置一列存）→ version（发布时事务内 INSERT 不可变快照，版本号唯一索引）→ publish（渠道维度的发布记录，渠道=connector：WebSDK/API 两个白名单 ID）。运行时按 `{AgentID, IsDraft, ConnectorID}` 取 draft 或对应渠道的版本快照；对外 `/v3/chat` SSE API + `/v3/chat/retrieve` + message/list。

- **对基座的裁剪**：基座 caller 的提示词/工具/Skill 配置目前是"活的"（改了立即生效）。当出现"巡检 agent 要稳定运行，不被配置变更打断"的需求时（尤其定时工作流上线后），抄这个模式：**定时触发时冻结 caller 配置快照（commit_id），run 记录回写快照 ID**——headless run 永远可复现、可审计。这是本篇性价比最高的一条：给 `tblLlmWorkflowRun` 加一列 `config_snapshot_id` 就够。
- 渠道分发（connector）对基座暂无需求，跳过。

### 2.4 插件双文档模型：manifest + OpenAPI3（→ P3 Bundle 直接可用）

coze 插件 = `PluginManifest`（schema_version / name_for_model / description_for_model / auth / api.type{cloud,custom,mcp} / common_params，内置 `Validate()`）+ **OpenAPI3 operation 作为工具契约**（输入输出 schema 直接取 parameters/requestBody/responses，递归转树状参数，支持 `x-*` 扩展）。配套：

- **AuthV2 多态凭据**：none / service_api_token / oauth_authorization_code / oauth_client_credentials，payload 统一 AES 加密落库（`COZE_PLUGIN_AUTH_SECRET`）；
- **OAuth 授权挂起**：缺 token 时抛 `InterruptAndRerunErr` 中断事件 → 用户授权 → 重放工具调用（HITL 与工具执行的优雅结合）；
- draft/version 双表让发布后的插件不可变；
- 市场安装 = 资源拷贝（duplicate 到用户空间），官方插件以 YAML 预置。

- **对基座的裁剪**：[服务端AI工作台改造方案.md](./服务端AI工作台改造方案.md) P3 的 Bundle 可直接采用这个格式——**bundle.yaml（元信息 + 鉴权 + 公共参数）+ 每工具一份 OpenAPI operation**，`Validate()` 逻辑近乎照搬；现有 http 工具的 inputSchema 与 OpenAPI schema 的转换器可双向写。AES 凭据加密方案顺手解决 P2 里 MCP/HTTP 工具的密钥存储问题。

### 2.5 RAG 最小闭环：三表 + 事件驱动管线 + 可插拔检索存储（→ 未来知识库）

数据模型只有三张表：`knowledge`（含 format_type 文本/表格/图片）→ `knowledge_document`（parse_rule/table_info JSON 列）→ `knowledge_document_slice`（content/sequence/hit，**向量不落 MySQL**）。写入是 MQ 事件驱动（解析→切片→双写 向量库+ES）；检索链 = queryRewrite（LLM 改写）→ **并行三路召回（向量/全文/可选 NL2SQL）** → rerank（默认 **RRF 融合，免模型**）→ MinScore/TopK 截断。检索存储抽象为 searchstore Manager（milvus/es/oceanbase 可换），embedding 内聚在 store 内。

- **对基座的裁剪**：基座做 RAG 时表结构照抄三表（够用且轻）；检索链用不到 eino 也能 120 行内移植（并行召回 + RRF）；**默认 RRF 免 rerank 模型**是低成本起步的正确姿势。知识库的按需加载同样是工具化做法——`recallKnowledge` 工具把知识库清单写进参数 schema 的 description，与基座 Skill 摘要注入完全同构，可统一为"索引注入 + 工具深检索"双通道。
- 表格知识库的 NL2SQL 安全链（逻辑表名重写 + 语句类型白名单 + 强制行过滤 + 行转 Document）是已验证的路径，未来做结构化数据问答时整链可抄。

### 2.6 消息分型与 run 状态（→ 对照基座消息模型）

coze 没有"事件表"——**事件即消息**：`message` 表按语义分型（question/answer/function_call/tool_response/verbose/follow_up），流式输出先建 `Streaming` 状态行、流结束 Edit 成完整内容（含 broken_position 断点标记）；上下文组装时 `historyPairs` 剔除孤立的 function_call。`run_record` 七状态（含 **required_action**）+ **chat_request 原始请求快照** + usage 内嵌。

- **对基座的取舍**：基座"消息即上下文 + toolMeta 旁路 + 回放同形"已经更优（事件粒度更细、断线续传更完整），**不建议改**。值得吸收两点：① run 表补 **原始请求快照**（审计/重放/复现一次 run 的完整依据）；② `required_action` 这种"等待外部输入"的显式状态命名，比 waiting_client_message 语义更通用（未来 HITL 场景可复用）。

### 2.7 轻量 checkpoint + 中断事件队列（→ HITL/断线恢复参考）

coze 的恢复体系三层分离：① 只有 schema 含可中断节点才启用 checkpoint（**按需启用**），checkpoint 本体是 KV 存储里的**不透明字节**（Redis 7 天 TTL，三方法接口）；② 业务层独立维护 `interrupt_event` DB 队列（GetFirst/Pop/TryLock 幂等恢复）；③ 恢复信息从历史消息 ext 反解。

- **对基座的启示**：基座的恢复模型（事件全量落库 + waiting 状态 + 上行回填）更简单可靠，不动；但"**按需启用 + 不透明快照 + 恢复队列幂等锁**"的分层思想，在 P2 危险操作确认（多个 pending 确认的排队与恢复）时用得上。

### 2.8 工程模式四件套（低成本高回报）

- **ResourceQueryer 注册模式**：新资源类型注册一个"查归属"的 Queryer 即获得统一鉴权判定——基座未来加 agent/bundle/workflow 资源时可复用这个模式；
- **双 schema 表里分离**：前端画布 JSON（vo.Canvas）与后端执行 schema（WorkflowSchema）独立演进、adaptor 转换——基座做编排容器 DSL 时同样适用（UI 结构 ≠ 执行结构）；
- **InputSources 变量系统**：节点间传值只有"字面量 Val 或 Ref{FromNodeKey}"两种、依赖即 Ref 列表——P3 编排容器不需要图引擎，照这个结构就能获得可校验的变量系统（与既有调研"不引入图引擎"结论一致）；
- **infra 抽象层**：storage（minio/s3/tos）/searchstore/eventbus（kafka/nats/nsq/pulsar/rmq）/embedding 全部接口化 + 可插拔 impl——与基座 asynctask Provider 的"可插拔+空转降级"是同一种工程审美，可延续。

## 3. 明确不值得学 / 需要避坑的

| 项 | 问题 | 基座对策 |
|---|---|---|
| PAT 存 MD5、session HMAC 密钥硬编码 | 开源简化实现 | SHA-256/bcrypt + 配置注入密钥 |
| 权限判定退化为 creator-only | 骨架有、实现未完成 | 学 ResourceQueryer 结构，判定规则自己补全 |
| HTTP 工具执行用 resty 默认 client | 无显式超时/重试 | 基座现有工具代理链路已更严谨，不动 |
| 定时触发器 | 开源版无（商业版特性） | 按[定时触发工作流实现方案](./定时触发工作流实现方案.md)自建 |
| MCP 集成 | 占位未实现 | 基座已实现三种传输 |
| coderunner 本地 python 子进程 | 非容器沙箱（策略白名单勉强兜底） | 基座已有独立 python 沙箱服务，不动 |
| 上下文按轮数截断、无压缩 | 简陋 | 基座 token 级压缩已领先 |
| 单体 hertz + IDL 生成 | 与基座 gin 手写风格差异大 | 只学模块划分，不迁移框架 |

## 4. 抽取优先级（结合基座 roadmap）

| 优先级 | 抽取什么 | 喂给哪个规划 | 成本 |
|---|---|---|---|
| ★★★ | 双通道鉴权骨架（Inspector + session/PAT 分流 + 哈希存 PAT） | [多端接入](./多端服务形态接入方案.md) P0 | 中 |
| ★★★ | 配置快照冻结（commit_id 列 + run 回写） | [定时触发工作流](./定时触发工作流实现方案.md) 可审计性 | 低 |
| ★★★ | 插件双文档模型 + AuthV2/AES 凭据 | [工作台改造方案](./服务端AI工作台改造方案.md) P3 Bundle | 中 |
| ★★ | RAG 三表 + 并行召回 + RRF 检索链 | 未来知识库能力 | 中高 |
| ★★ | run 原始请求快照 + required_action 状态命名 | run 模型小增强 | 低 |
| ★ | Space/space_user 空间模型 | 多租户演进预留 | 暂不做 |
| ★ | ResourceQueryer / InputSources / 双 schema 工程模式 | P2/P3 实现时顺手用 | 低 |

## 5. 关键源码索引

| 主题 | 位置（相对 `coze-studio/backend/`） |
|---|---|
| 请求分类/双通道鉴权 | `api/middleware/request_inspector.go`、`session.go`、`openapi_auth.go`；PAT `domain/openauth/openapiauth/` |
| 空间与权限 | `domain/user/`（user/space/space_user 模型）、`domain/permission/`（CheckAuthz、resource_queryiers.go） |
| 草稿/发布/渠道 | `domain/agent/singleagent/internal/dal/model/`（draft/version/publish）、`domain/app/service/publish_app.go`、`crossdomain/connector/` |
| 对话运行时 | `application/conversation/agent_run.go`、`domain/conversation/agentrun/internal/run.go`、`singleagent_run.go`；eino 图 `crossdomain/agent/impl/single_agent.go`、`agentflow/agent_flow_builder.go` |
| 消息/会话表 | `domain/conversation/*/internal/dal/model/`（conversation/run_record/message） |
| 插件体系 | `crossdomain/plugin/model/`（PluginManifest/AuthV2/ToolInfo）、`domain/plugin/service/exec_tool.go`、`service/tool/invocation_*.go`、加密 `domain/plugin/encrypt/aes.go` |
| 知识库 RAG | `domain/knowledge/`（三表 + `service/event_handle.go` 管线 + `service/retrieve.go` 检索链）、`infra/document/{parser,searchstore,embedding,rerank,nl2sql}/`、`infra/sqlparser/` |
| 工作流 | `domain/workflow/`（draft/snapshot/version/execution/node_execution、`internal/compose/workflow.go`、`entity/node_meta.go` ~40 节点、`internal/repo/interrupt_event_store.go`）、`infra/checkpoint/redis.go` |
| 记忆 | `domain/memory/variables/`（KV + setKeywordMemory 工具）、`domain/memory/database/`（SQL 记忆表） |
| 存储抽象 | `infra/storage/`、`infra/eventbus/` |
