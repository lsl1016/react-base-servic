# 长期记忆模块设计（Long-Term Memory）

> 状态：设计稿（feature/memory 分支）
> 目标：为 ReAct 基座补上**跨会话长期记忆**能力——会话结束时沉淀事实与偏好，新会话开始时按需注入，并由后台整理任务持续维护。
>
> 设计蓝本：Letta Code 的 MemFS 记忆系统（`C:\Users\keke\Desktop\xm\letta-code\docs\memory-system-design.md`，
> Apache-2.0，取其**设计**而非代码）与 `letta-code\docs\改造建议.md` 的结论：
> **ChatGPT 的双层记忆思想 + Mem0 的事实化写入/检索 + Letta 的常驻核心记忆**。

---

## 1. 背景与问题

当前基座的上下文生命周期止于会话内部：

| 现有机制 | 覆盖范围 | 缺口 |
| --- | --- | --- |
| 上下文自动压缩（`compact_start` / `compact_end`） | 单会话内，超水位时把早期消息摘要化 | 会话结束，摘要随之失效 |
| 会话历史回放（`/react/session/events`） | 事后查看，不参与新会话推理 | 新会话无法"想起"旧会话 |
| 系统提示词 / Skill 摘要注入 | 静态知识，人工维护 | 无法从对话中自动积累 |

带来的实际问题：同一个用户的偏好（称呼、常用格式、业务背景）每次都要重新说明；跨会话的事实（"上次的方案 A 已经被否了"）丢失；caller 想让 agent"越用越懂业务"没有抓手。

## 2. 设计原则

1. **双层记忆**（借鉴 ChatGPT Memory / Letta MemFS 的 system-detached 分层）：
   - **常驻层（resident）**：少量高价值、必须每次都在场的记忆（用户画像、长期偏好、身份约定），有严格的字符预算，随 system prompt 前缀注入；
   - **按需层（detached）**：大量历史事实，只注入目录索引，模型通过 `memory_list` / `memory_read` 工具按需读取。
2. **记忆即事实条目**（借鉴 Mem0）：每条记忆是一个原子条目（一句话可表述的事实/偏好/约定），带 `description` 供检索、带 `reason` 供审计，而非整段自由文本。
3. **写入有痕**（借鉴 MemFS "每写必 commit"）：任何写入（模型工具写、后台整理写、管理面改）都产生一条不可变的修订记录，可回滚、可审计。
4. **作用域与现有体系对齐**：记忆的可见范围沿用 caller（+ 可选 user 维度）模型，权限校验复用会话归属校验思路，工具进现有注册表，注入复用 Skill 摘要的注入通道。
5. **先机制、后智能**：第一阶段只有模型自管写入 + 常驻注入；reflection（后台整理）与向量检索放后面，各自可独立开关。

## 3. 总体架构

```text
                          ┌──────────────────────────────────────────┐
                          │              ReAct 运行时                 │
                          │                                          │
   run 初始化 ────────────►  system prompt 前缀注入                    │
   （与 Skill 摘要同通道）    │   <memory> 常驻层全文 + 按需层目录 </memory> │
                          │                                          │
                          │   工具循环                                │
                          │   ├─ memory_list  （按需层目录/过滤）       │
                          │   ├─ memory_read  （读单条/多条全文）        │
                          │   └─ memory_write（新建/更新/删除，必填 reason）│
                          └───────┬──────────────────────┬───────────┘
                                  │                      │ compact_end 事件
                                  ▼                      ▼
                        ┌──────────────┐        ┌──────────────────┐
                        │ tblLlmMemory │        │ reflection 整理 run │
                        │     Item     │        │ （异步子 run，受限  │
                        │ 条目 + 版本号  │        │  工具集，五阶段）   │
                        └──────┬───────┘        └────────┬─────────┘
                               │ 每次写入                  │ 也走 memory_write
                               ▼                         │
                        ┌──────────────────┐             │
                        │ tblLlmMemory     │◄────────────┘
                        │    Revision      │
                        │ （不可变审计流水） │
                        └──────────────────┘
```

两类参与者都只通过 `memory_write` 等工具落库，保证审计流水完整：

- **在线写入**：主对话 run 中模型主动调用（用户说"记住我用的是香港主体"这类显式信号，或模型判断值得沉淀）。
- **离线整理（reflection）**：压缩事件后异步触发的子 run，审阅近期对话，提炼/合并/淘汰记忆（第三阶段上线）。

## 4. 存储模型

### 4.1 表结构

沿用 `tblLlm` 前缀与 GORM 建模规范（见 `models/llm/`、`sql/init.sql`），新增两张表：

```sql
-- 记忆条目：一条原子事实。逻辑主键 (owner_type, owner_key, item_key)，item_key 由内容语义哈希生成，
-- 同一事实重复写入收敛为更新而非新增。
CREATE TABLE IF NOT EXISTS `tblLlmMemoryItem` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `owner_type`    VARCHAR(32)  NOT NULL COMMENT '记忆归属维度：caller / caller_user',
  `owner_key`     VARCHAR(255) NOT NULL COMMENT 'caller:key 或 caller:key|userName 拼接',
  `layer`         VARCHAR(16)  NOT NULL DEFAULT 'detached' COMMENT 'resident=常驻层 / detached=按需层',
  `title`         VARCHAR(128) NOT NULL DEFAULT '' COMMENT '短标题，目录索引展示用',
  `content`       TEXT         NOT NULL COMMENT '记忆正文（一到三句原子事实）',
  `description`   VARCHAR(512) NOT NULL DEFAULT '' COMMENT '检索描述：什么时候需要这条记忆',
  `tags`          VARCHAR(512) NOT NULL DEFAULT '' COMMENT '逗号分隔标签，memory_list 过滤用',
  `source`        VARCHAR(32)  NOT NULL DEFAULT 'model' COMMENT '写入来源：model / reflection / admin',
  `item_key`      VARCHAR(64)  NOT NULL COMMENT '内容语义指纹（规范化后 SHA-256 前 16 位），幂等去重',
  `version`       INT          NOT NULL DEFAULT '1' COMMENT '乐观锁版本号，每次修订 +1',
  `state`         VARCHAR(16)  NOT NULL DEFAULT 'active' COMMENT 'active / deleted（软删）',
  `last_reason`   VARCHAR(512) NOT NULL DEFAULT '' COMMENT '最近一次修订原因（冗余展示用）',
  `created_by`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '触发写入的 runID 或操作人',
  `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_owner_item` (`owner_type`, `owner_key`, `item_key`),
  KEY `idx_owner_layer` (`owner_type`, `owner_key`, `layer`, `state`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 修订流水：不可变，只插不改。回滚 = 用旧快照反向插入一条新修订。
CREATE TABLE IF NOT EXISTS `tblLlmMemoryRevision` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `item_id`      BIGINT UNSIGNED NOT NULL,
  `action`       VARCHAR(16)  NOT NULL COMMENT 'create / update / delete / rollback',
  `before_json`  TEXT         NULL COMMENT '变更前快照（create 时为空）',
  `after_json`   TEXT         NULL COMMENT '变更后快照（delete 时为空）',
  `reason`       VARCHAR(512) NOT NULL COMMENT '必填：为什么改这条记忆',
  `source`       VARCHAR(32)  NOT NULL COMMENT 'model / reflection / admin',
  `created_by`   VARCHAR(64)  NOT NULL COMMENT 'runID 或操作人',
  `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_item` (`item_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 4.2 作用域（owner）

| owner_type | owner_key 构成 | 适用场景 | 谁可见 |
| --- | --- | --- | --- |
| `caller` | `{callerKey}` | 业务方全局约定（产品口径、通用 SOP） | 该 caller 下所有会话 |
| `caller_user` | `{callerKey}\|{userName}` | 终端用户个人偏好与事实 | 该 caller 下该 userName 的会话 |

run 初始化时按 **caller_user → caller** 两级合并解析（caller 级做公共底座，user 级覆盖同名 `item_key`），与系统提示词 `default` 作用域"由通用到具体"的合并语义一致。不引入 route 维度：记忆的语义（用户偏好/业务事实）天然跨路由，按路由切分会把同一用户割裂成多个失忆副本；确有路由隔离诉求时，用 `tags` 表达。

注入总量控制：常驻层条数上限（默认 16 条）+ 字符预算（默认 2000 字符），超出按 `updated_at` 降序截断并在索引尾部提示"另有 N 条常驻记忆未注入，可用 memory_list 查看"。防止单 caller 无限膨胀撑爆 system prompt。

## 5. 上下文注入

挂载点复用 Skill 摘要注入的同一通道（run 初始化阶段拼 system 前缀，参照 `service/react/meta_tools.go` 的 skill index snapshot 机制）：

```text
<memory>
## 关于当前用户的长期记忆（自动维护，可信）

### 常驻
- [称呼] 用户希望被称为"陈总"（2026-09-01 起）
- [主体] 公司主体在香港，报表口径用 HKD
...

### 记忆目录（需要时用 memory_read 按 itemId 读取全文）
- #12 [偏好] 数据分析结论先给摘要再给图表
- #15 [背景] 上季度预算方案 A 已被管理层否决
...
</memory>
```

要点：

1. **常驻层全文注入**，每条一行；**按需层只注入目录**（id + title + description 首段），正文必须走 `memory_read`。对应 MemFS 的 `system/` 常驻与 detached 按需检索。
2. 目录条目上限（默认 64 条），超出部分靠 `memory_list` 分页/过滤。
3. 记忆为空时整个 `<memory>` 块不出现，不浪费 token。
4. 注入发生在每次 run 初始化（而非会话创建），保证跨 run 的写入对同会话后续轮次立即可见。

## 6. 工具协议

注册进现有工具体系（`tblLlmTool` `tool_type=client` 的内置工具形态，与 `get_skill` / `create_plan` 同类），受 caller 白名单管控，可通过配置整体关闭。

### memory_list —— 列出/检索记忆

```json
{ "name": "memory_list", "description": "列出当前作用域的长期记忆（默认按需层全部）",
  "parameters": {
    "layer":  "resident | detached | all，默认 detached",
    "tag":    "按标签过滤，可选",
    "keyword": "标题/描述关键词过滤，可选（第一阶段为 LIKE 匹配）",
    "limit":  "默认 20，最大 50" } }
```

返回：条目数组（id / title / description / tags / layer / updatedAt），不含正文。

### memory_read —— 读取记忆全文

```json
{ "name": "memory_read",
  "parameters": { "itemIds": "数组，一次最多 10 条" } }
```

返回：正文全文 + 版本号 + 来源与最近修订原因。**作用域校验**：owner 不在当前 run 可见范围（caller_user/caller 两级）内的 id 一律拒绝，防止跨用户/跨 caller 编造读取（与会话归属校验同思路）。

### memory_write —— 写入/更新/删除记忆

```json
{ "name": "memory_write",
  "parameters": {
    "action":  "create | update | delete",
    "itemId":  "update/delete 必填",
    "layer":   "resident | detached，create 默认 detached；常驻层写入加倍审慎",
    "title":   "create/update 必填，≤32 字",
    "content": "create/update 必填，一到三句原子事实",
    "description": "create/update 必填：什么场景需要想起这条记忆",
    "tags":    "可选，逗号分隔",
    "reason":  "必填：为什么写入/修改/删除" } }
```

行为：

- **幂等收敛**：create 时按 `item_key`（规范化 content 的语义指纹）查重，命中未删条目自动转为 update 并合并 layer/tags；
- **乐观锁**：update 携带读取时的 version，冲突则失败并提示重读（模型重试即可）；
- **reason 必填**：空则直接校验失败，这是审计链的根；
- **写入即修订**：每次成功操作插入一条 `tblLlmMemoryRevision`；
- **常驻层守门**：单 owner 常驻层超上限时写入失败并提示"需先降级一条常驻记忆"（不自动挤占，逼模型显式决策）。

### 工具说明文案中的写入纪律（system 提示词约束）

在 `memory_write` 的 description 与注入块尾部写明纪律，对应 Letta 的记忆写入协议：

> 只记稳定事实与明确偏好，不记一次性的任务上下文；宁可少写不写错；用户明确说"忘记"时执行 delete 并给 reason。

## 7. Reflection：后台整理（第三阶段）

借鉴 Letta 的 reflection（sleeptime）机制：**触发器挂在压缩事件上**（Letta 默认 trigger 即 compaction-event，与现有 `compact_end` 事件天然对齐）。

### 7.1 触发链路

```text
engine 发出 compact_end
  → memory reflection 判定（配置开关 + 冷却期，如同一 session 1 小时内至多 1 次）
  → 异步派生一个 reflection run：
      session_type = 'reflection'（复用 tblLlmReactSession，历史列表默认隐藏）
      工具集白名单 = memory_list / memory_read / memory_write（仅此三个）
      上下文 = <memory> 注入块 + 压缩摘要 + 被压缩覆盖的近期消息（CoveredThrough 引用区间）
  → 整理产物全部经 memory_write 落库（source=reflection，reason 必填）
  → 失败静默重试一次，再失败仅记日志，绝不影响主会话
```

复用要点：reflection run 本身就是普通 run，事件流照常持久化，回放页天然可看（排障友好）；`compactSummaryContent.CoveredThrough`（`service/react/engine.go`）已记录被摘要覆盖的消息区间，reflection 可精确取回"被压缩掉的原文"做审阅，这正是压缩后原文不丢失的第二个用途。

### 7.2 五阶段提示词骨架（取自 Letta reflection 的阶段划分，措辞自行重写）

Investigate（读近期对话与现有记忆）→ Extract（列出候选事实/变化/过期项）→ Update（逐条 create/update/delete，每条给 reason）→ Review（重读修改后记忆，检查矛盾与预算）→ Commit（输出本次整理摘要，作为 run 结束语）。

### 7.3 整理职责边界

- 合并重复事实（同义条目收敛为一条，保留更准确的表述）；
- 淘汰过期项（时间敏感事实加"截至 YYYY-MM-DD"前缀，过期的 delete）；
- 分层调整（高频被 memory_read 命中的 detached 条目可提名升 resident；反之降级）；
- **不做**跨 owner 迁移、不做向量嵌入（后续阶段）、不修改用户显式锁定条目（`tags` 含 `locked` 的条目 reflection 只读）。

## 8. 配置项（conf/mount/custom.yaml）

挂在现有 `llm.react` 段下，风格对齐 `allow_plan`：

```yaml
llm:
  react:
    memory:
      enabled: true                # 总开关；false 时不注入、不注册工具
      residentMaxItems: 16         # 常驻层条数上限（单 owner）
      residentBudgetChars: 2000    # 常驻层字符预算
      indexMaxItems: 64            # 按需层目录注入条数上限
      allowUserScope: true         # 是否启用 caller_user 维度（false 则全员共享 caller 级记忆）
      reflection:
        enabled: false             # 第三阶段默认关
        cooldownMinutes: 60        # 同一 session 触发冷却
        maxWritesPerRun: 20        # 单次整理写入上限，防失控
```

## 9. 管理面与可观测

- **HTTP 管理接口**（挂管理路由组，风格对齐 MCP 连接管理）：
  - `POST /react/memory/list`——按 owner/layer/tag/keyword 查询；
  - `POST /react/memory/update`、`/react/memory/delete`——人工修订（source=admin，同样落修订流水）；
  - `POST /react/memory/revisions`——条目修订历史（回滚取 before_json 反向提交）；
  - `POST /react/memory/rollback`——指定 revision 回滚。
- **回放**：在线写入走工具卡片（现有渲染）；reflection run 是普通 session，`session_type='reflection'` 在历史列表默认折叠，排障时可展开回放。
- **指标**（`/metrics` 增补）：memory_write 调用量与失败率、按 source 分布、常驻层字符占用水位、reflection 触发/失败次数、记忆条目数按 owner TopN。

## 10. 安全与边界

1. **跨用户隔离**：`caller_user` 作用域的记忆在 run 初始化时按 `userName` 解析，工具执行时校验条目 owner ∈ {当前 caller_user, 当前 caller}，杜绝跨用户读取（参照 `display_files.go` 对产物归属的铸造式校验思路）。
2. **敏感信息**：记忆正文可能含 PII/密钥。第一阶段的纪律靠提示词（"不记录凭证、证件号、密钥"）+ 管理面可删；第二阶段加写入前正则拦截（复用现有敏感词/凭证 pattern）。
3. **预算防膨胀**：常驻层双重上限（条数+字符）；按需层单 owner 软上限（默认 500 条，超出拒绝 create 并提示先整理）；reflection 有单次写入上限。
4. **并发**：条目级乐观锁（version），修订流水只插不改天然无并发问题。
5. **信任模型**：注入块标注"自动维护"，但记忆可能过时甚至被误导写入——`memory_write` 纪律 + reflection 淘汰 + 用户口头纠正（模型应 update 而非新增）三层兜底；管理面保留最终删除权。

## 11. 实施计划

| 阶段 | 内容 | 交付物 | 依赖 |
| --- | --- | --- | --- |
| P1 基础闭环 | 建表、模型层、常驻+目录注入、memory_list/read/write 三工具、配置开关、单测 | 记忆自管可用，playground 可验证 | 无 |
| P2 审计与管理面 | 修订流水查询/回滚接口、管理页（可先并入现有管理面板）、写入敏感词拦截、指标 | 运营可治理 | P1 |
| P3 Reflection | compact_end 触发链路、五阶段提示词、受限工具集 run、冷却与限额 | 自动整理上线（默认关，灰度开） | P1/P2 |
| P4 检索增强 | keyword → 向量检索（`description` 嵌入），配合 feature/RAG 分支的向量基础设施 | 大规模按需层可用 | P1，RAG 基础设施 |

P1 验收标准：同一 caller+user 的新会话能复现上一会话沉淀的偏好（端到端手测脚本）；关闭开关后行为与现状完全一致（零回归）；记忆为空时 token 消耗与现状一致。

## 12. 与蓝本的取舍说明

| MemFS（Letta）做法 | 本设计取舍 | 理由 |
| --- | --- | --- |
| 记忆存本地 git 仓库 | MySQL 条目 + 修订流水表 | 基座状态已在 MySQL，git 目录引入第二存储体系，运维与事务一致性成本高；"每写必 commit"的审计语义用 Revision 表等价实现 |
| `system/` 目录 + frontmatter description | layer 字段（resident/detached） | 关系模型里目录结构是冗余，两层语义用字段表达更直接 |
| reflection 子 agent 跑在 OS 级沙箱 | 受限工具集的普通 run | 复用现有运行时与事件流，隔离目标（只能动记忆）用工具白名单达成 |
| git 远端镜像备份 | 管理面导出（后续可加 JSON 导出/导入） | 备份诉求降级为可运营操作 |
| 云端 agent 状态（Letta API） | 不引入，全本地 | 集成评估结论：不引运行时/进程/服务，仅取设计 |
