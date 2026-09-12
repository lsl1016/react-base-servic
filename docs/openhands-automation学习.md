# OpenHands 无人值守执行体系学习文档（定时触发工作流）

> 调研对象：本地 `../software-agent-sdk`（源码级深挖其"无人值守执行"基础）+ 独立仓库 `OpenHands/automation`（网络调研，Beta）+ Agno AgentOS Scheduler / LangGraph Cron（次要参考）。
> 调研目的：为 react-base-service 设计「定时触发工作流」（定时智能巡检）能力提供参考。
> 与 [openhands-sdk学习.md](./openhands-sdk学习.md) 互补：前文讲 Agent 运行时本体，本文讲"**没人盯着时 Agent 怎么被触发、跑完、回报**"。
> 撰写日期：2026-09-12。

---

## 1. 总览：三层职责拆分

OpenHands 把"定时做一件事"拆成三层，每层职责边界在官方文档里写得很明确：

```text
┌────────────────────────────────────────────────┐
│ Automation Service（OpenHands/automation, Beta）│  「何时跑」
│  automation 定义 / cron+webhook 触发 / dispatch  │
│  run history / sandbox 生命周期编排 / 完成回调   │
└───────────────┬────────────────────────────────┘
                │ 创建 sandbox、注入回调凭据（HTTP）
                ▼
┌────────────────────────────────────────────────┐
│ Agent Server（容器内进程）                       │  「怎么跑」
│  Agent Runtime / Conversation / Tool / MCP      │
│  Workspace / SubAgent / 后台 run / goal 循环     │
└───────────────┬────────────────────────────────┘
                │ SDK（openhands-sdk）
                ▼
┌────────────────────────────────────────────────┐
│ 会话执行完成 → POST /v1/runs/{id}/complete 回报  │  「跑完了说一声」
└────────────────────────────────────────────────┘
```

关键认知：**SDK/Agent Server 里没有任何 cron 框架**——全仓库搜 cron/schedule 只有租约续期、空闲回收、flush 延迟这类内部 asyncio 循环。定时是 Automation 层的职责，运行时只提供"后台执行 + 完成回报"的契约。这个分层本身就是最重要的学习点。

---

## 2. 运行时侧：无人值守执行的基础（源码级）

### 2.1 后台 run：没有客户端时事件去哪里

`openhands-agent-server` 的 `POST /api/conversations/{id}/run`（`event_service.py::EventService.run()`）：

- 持 run 锁原子检查——已在 RUNNING 则 409（`conversation_already_running`）；
- `asyncio.create_task(_run_and_publish())` **立即返回**，run 在后台驱动（原生 arun 优先，同步 agent 落线程池）；
- **事件去向与 WS 客户端无关**：所有事件先落盘到会话目录的 EventLog（append-only 文件），事后 `GET /conversations/{id}/events` 分页拉取；若配置了 webhooks，`WebhookSubscriber` 作为**内部订阅者**在会话激活时即注册，后台 run 的事件照样批量 POST 给编排方；
- 最终结论 `GET …/agent_final_response`（取最后 FinishAction 文本）；
- 迭代上限 `max_iteration_per_run`（默认 500）+ goal 循环另有审计轮上限；**无 run 级 wall-clock 超时**（靠客户端 wait 超时或 automation 层两段式超时兜底，见 §3.4）。

### 2.2 goal 循环：裁判 LLM 驱动的自治外环

`openhands-sdk/openhands/sdk/conversation/gooal/`（controller.py / judge.py / runner.py）+ agent-server 的 `_run_goal_loop`。这是 OpenHands 对"巡检式任务"最直接的回答：

```text
objective（目标陈述）
   ↓ 发为首条消息，await run()
一轮结束 → 裁判 LLM 只读 LLMConvertibleEvent 渲染的 transcript
   → verdict(score/complete/missing)
       ├─ complete → GoalStatus=complete，结束
       └─ 未达成 → 自动构造 followup 消息再 run 一轮
                   （默认上限 10 轮 → status=capped）
状态经 ConversationStateUpdateEvent(key="goal") 持久化+广播
   → resume_goal_loop 可跨服务器重启恢复
```

三个可借鉴的细节：① 裁判是**独立 LLM 调用**，与执行模型解耦（`judge_goal()` 纯函数：objective+transcript→verdict）；② 状态机的每次变迁都是事件（可回放、可恢复）；③ STUCK 不终止循环（继续交给裁判判断），PAUSED/ERROR 才终止。

### 2.3 RemoteConversation：宿主进程驱动远程会话

`sdk/conversation/impl/remote_conversation.py`——编排方（如 automation 的 local 后端）程序化驱动一个跑在 agent-server 里的会话的标准姿势：

- 创建：`POST /api/conversations`（Agent 配置序列化随请求传入）；
- 事件订阅：`WebSocketCallbackClient` 守护线程连 `/sockets/events/{id}`，断线指数退避重连 + REST 补齐对账；构造时阻塞等 WS ready（30s 超时）；
- 驱动：`send_message()`（run=False）→ `run(blocking=True, timeout=3600)`：POST /run 触发后台 run，然后**以 WS 全量终态快照为唯一权威完成信号**（因为 stop hook 可能回滚 FINISHED），REST 轮询作兜底；
- 与 LocalConversation 的接口差异：`execute_tool`/`stuck_detector` 不支持（服务端职责）；pause/interrupt/fork/确认答复全部转 REST。

### 2.4 webhook 外推：两类订阅者

`conversation_service.py`：

- **事件流 webhook**：攒够 buffer（5 条）/字节阈值/30s flush_delay 后 `POST {base_url}/events/{conversation_id}`，body 是 Event JSON 数组；带背压（队列超限丢最旧）与 3 次重试；
- **生命周期 webhook**：会话 start/pause/interrupt/delete/update 即时 `POST {base_url}/conversations`。

### 2.5 与 automation 的集成契约（运行时侧）

SDK 不"知道"automation，只认环境变量（`workspace/remote/base.py`）：

- `AUTOMATION_CALLBACK_URL` / `AUTOMATION_RUN_ID` / `AUTOMATION_EVENT_PAYLOAD`（trigger 可为 cron/webhook/manual）——dispatcher 注入沙箱；
- workspace `__exit__` 时 `POST` 回调：`{status: COMPLETED|FAILED, run_id, conversation_id, cost, error}`；
- 回调凭据 `AUTOMATION_CALLBACK_API_KEY`（较新版本才补上，早期无鉴权是 Beta 局限）。

**设计精髓：执行侧只暴露"完成回报"端点，调度正确性（丢回调怎么办）由调度侧 watchdog 负责。**

### 2.6 无人值守的授权/确认语义

- **默认自动放行**：`StoredConversation.confirmation_policy` 默认 `NeverConfirm`，security_analyzer 默认 None——headless run 不等人；
- 若配了 AlwaysConfirm/ConfirmRisky：run 停在 `WAITING_FOR_CONFIRMATION`，**支持事后异步答复** `POST …/events/respond_to_confirmation {accept, reason}`（accept 则二次 run 隐式确认执行 pending actions；reject 则给每个 pending action 补拒绝观察）；后续 send_message 时遗留 pending 确认会被自动拒绝，防挂死。

---

## 3. 调度侧：OpenHands Automation Service（网络调研，Beta）

### 3.1 组件与执行模型

Python 3.12 + FastAPI + SQLAlchemy + **PostgreSQL**（Alembic 迁移）。四个核心部件：

```text
scheduler.py   自研 async 轮询（croniter 解析，默认 60s 一轮，批量 50 条）
               PG FOR UPDATE SKIP LOCKED —— 天然支持多 worker 横扩
dispatcher.py  独立进程：领取 PENDING run（同样 SKIP LOCKED）→ 置 RUNNING
               → 经 ExecutionBackend 建沙箱、注入回调凭据 → 不阻塞
backends/      cloud.py（sandbox API 建云端沙箱）/ local.py（直连常驻 agent-server，
               每 run 独立 workspace ~/.openhands/workspaces/automation-runs/{run_id}/）
watchdog.py    回调丢失时到 agent-server 读 BashOutput 兜底核验 run 真实状态
```

状态机：`PENDING → RUNNING → COMPLETED | FAILED | CANCELLED | SKIPPED`（SKIPPED = org 并发超限的瞬时跳过，不算失败）。

完成链路是**异步回调制**：沙箱内 SDK 退出时 `POST /v1/runs/{id}/complete`（RUNNING→COMPLETED/FAILED），run 关联 conversation_id，用户可在正常会话列表里**查看/继续/调试**那次巡检会话。

### 3.2 策略取舍（有明确的设计立场）

| 关注点 | OpenHands Automation 的选择 |
|---|---|
| misfire（错过的触发） | **不补跑**：无着火点的 cron 视为 not due |
| 自动重试 | **无重试循环**：瞬时错误留待下次调度；永久错误直接失败并 `disable=True` |
| 连环失败 | `maybe_disable_unhealthy_automation_after_run()` **自动禁用**不健康的 automation |
| 并发 | org 级并发闸门由 sandbox API 强制，超限记 SKIPPED |
| 超时 | 两段式：provisioning 截止 = sandbox ready timeout + run timeout + margin；执行段 timeout_at 对齐沙箱内 bash 服务的 kill（保证 watchdog 能读到退出码） |
| 删除 | **软删除**（置 deleted_at + 禁用 + 跳过 pending runs）；官方最佳实践"优先 disable 而非 delete" |
| 非法 cron | 直接永久禁用 |
| 验证 | 最佳实践："新建先手动触发验证一次" |

### 3.3 定义格式与 API

真实 create schema（比社区传说的 prompt/repos 字段更工程化）：`name / model / trigger / tarball_path（s3://…https://…）/ setup_script_path / entrypoint / timeout / keep_alive / template`。trigger 是判别联合：

```json
{"type": "cron", "schedule": "0 9 * * *", "timezone": "Asia/Shanghai"}
{"type": "event", "source": "github", "on": "pull_request.*",
 "filter": "<JMESPath 表达式>", "destination": "dispatch_run | continue_conversation",
 "subject_key_expr": "<按 subject 归并同一会话>", "wake_agent": true}
```

API 面（`/v1`）：automation CRUD（创建对同 template 幂等）、`PATCH …/{id}`（`enabled` 字段即启停；仅创建者可改定义）、`POST …/{id}/dispatch`（手动触发）、`GET …/{id}/runs`（分页 + status_counts 统计）、`POST /v1/runs/{id}/complete|phase|cancel`（沙箱侧回报）、`/v1/webhooks` CRUD + rotate-secret、`POST /v1/events/{org}/{source}`（HMAC 验签 + 时间窗防重放 + 按 delivery id 去重）。鉴权为 per-user API key + 权限点（view/manage_automations）。

### 3.4 Beta 局限（我方的增强机会）

无 misfire 补跑、无自动重试、webhook 入口未做限流、hex HMAC 无时间窗可重放、自托管 REST 契约文档缺失（要读源码）、仓库年轻（22 stars）。

---

## 4. 次要参考

### 4.1 Agno AgentOS Scheduler（学"产品化字段清单"）

`AgentOS(scheduler=True)` 开启；调度的目标就是 AgentOS 自己的 run 端点。`POST /schedules`：`name / cron_expr / endpoint / payload / timezone / max_retries / retry_delay_seconds`；启停 `/enable|disable`、手动 `/trigger`、历史 `/runs`（status/timing/input/output/errors 全持久化）。**最值得抄的语义**：PAUSED（等待人类输入）的 run "recorded as paused rather than retried as a failure"——不把等待当失败重试。

### 4.2 LangGraph Cron（学"会话模型二分"）

cron → assistant → thread → run。两种模式：**有状态**（固定 thread 周期运行，跨执行累积上下文）/ **无状态**（每次触发新 thread，`on_run_completed: delete|keep` 控制保留）。字段：schedule（**默认按 UTC 解释**——时区坑）、timezone、end_time（到点停跑）、webhook。文档特别提醒"遗忘的 cron 会持续产生 LLM 费用"。

---

## 5. 可迁移设计要点（十条）

1. **调度与运行时必须分层**：运行时不需要知道 cron 的存在，只需要提供「可编程触发的一次 run + 完成回报」。
2. **触发定义与运行历史分表**，run 记录关联会话 id——巡检结果永远是"可点开的一场对话"，不是一行日志。
3. **轮询 + SKIP LOCKED** 是多 worker 调度的最简正确解；单实例时退化为一张表一个 goroutine。
4. **misfire 不补跑、失败不盲目重试、连环失败自动熔断**——对"定时巡检"这是对的默认值（补跑风暴比漏跑一次更危险）。
5. **完成回调 + watchdog 对账**：异步系统里"没收到回调"≠"没完成"，必须有兜底核验路径。
6. **并发超限用 SKIPPED 而非 FAILED**：语义区分让告警不误报。
7. **软删 + 启停分离 + 手动触发端点**：运维三件套（disable 优先于 delete；新建先手动触发验证）。
8. **无人值守 = 确认策略显式化**：要么工具集天然只读（不需要确认），要么默认自动放行 + 危险工具直接不可用；等待人工输入的状态要能被调度器识别并"记录而非重试"（Agno PAUSED 语义）。
9. **goal/裁判外环**：用独立的轻量 LLM 调用审计"目标是否达成"，未达成自动追问——巡检类任务的完成度保障机制，比加大 maxSteps 优雅。
10. **时区显式化 + cron 非法即禁用**：定时系统两类最常见事故的预防针。

## 6. 关键源码/资料索引

| 主题 | 位置 |
|---|---|
| 后台 run / 409 并发保护 | `software-agent-sdk/openhands-agent-server/openhands/agent_server/event_service.py`（EventService.run L1233） |
| goal 循环 | `openhands-sdk/openhands/sdk/conversation/goal/{controller,judge,runner}.py` + `event_service.py`（start/stop/resume_goal_loop） |
| RemoteConversation | `openhands-sdk/openhands/sdk/conversation/impl/remote_conversation.py` |
| webhook 订阅者 | `openhands-agent-server/.../conversation_service.py`（WebhookSubscriber L2637 / ConversationWebhookSubscriber L2851） |
| automation 集成契约 | `openhands-sdk/openhands/sdk/workspace/{base,remote/base}.py`（AUTOMATION_* 环境变量与完成回报） |
| 确认异步答复 | `openhands-agent-server/.../event_router.py`（respond_to_confirmation L218） |
| Automation Service | github.com/OpenHands/automation（scheduler/dispatcher/backends/watchdog/schemas.py）；docs.openhands.dev/openhands/usage/automations/* |
| Agno Scheduler | docs.agno.com/agent-os/scheduler/overview、/examples/agent-os/scheduler/rest-api-schedules |
| LangGraph Cron | docs.langchain.com/langsmith/cron-jobs |
