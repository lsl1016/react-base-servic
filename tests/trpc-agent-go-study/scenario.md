# 测试场景：tRPC-Agent-Go 多智能体代码学习（检索 MCP + Skill 全链路）

> 日期：2026-09-13 ｜ 会话：`session_a7c6d4ff13a54ff1a1618e93e1f192ae` ｜ 父 run：`run_738d2a7b6b9e445b9721cf47523bd13f`

## 目的

端到端验证「代码检索 MCP 工具族 + 代码检索策略 Skill」在真实多智能体场景下的可用性与效果：由父智能体并行委派三个子智能体，仅凭只读检索工具学习一个陌生 Go 仓库（tRPC-Agent-Go，腾讯开源，3155 个 .go 文件）的三大主题并产出报告。

## 前置条件

| 项 | 值 | 说明 |
|---|---|---|
| 检索镜像 | `data/repo-mirrors/trpc-agent-go` | `git clone https://github.com/trpc-group/trpc-agent-go`（Go 项目，AST 工具完全适用） |
| MCP 工具 | `mcp_repo_*` 共 9 个 | 挂 demo-app caller，含本轮新增的 `search_pattern`/`get_file_symbols`/`find_references`/`list_repositories` |
| Skill | `代码检索策略`（skill_e46606ac11a445efbc3c4f53a8190248） | 含 11 个 triggers；本测试同时验证 triggers 命中与 get_skill 按需加载 |
| 子代理 | `code-arch-learner` / `code-orchestration-learner` / `code-memory-learner` | 各挂 9 个 repo_* 工具 + Skill，systemPrompt 强制"第一步 get_skill" |
| 轮次上限 | 父 500 / 子 500 | `custom.yaml` 的 `react.max_steps` 与 `subagent.default_max_steps`（测试临时调大） |
| 模型 | claude 协议 / glm-4.6 | — |

## 执行步骤

1. 克隆 tRPC-Agent-Go 到镜像目录（普通工作副本形态）；
2. 调大父/子最大运行轮次至 500 并重启服务（配置热加载验证：MCP 工具重新同步，`tblLlmTool` updated_at 刷新）；
3. 经 `/agent/create` 注册三个学习型子代理（任务自包含、限定单主题、强制走 Skill）；
4. 经 WebSocket（`GET /react/ws`）发起 `type=run` 的 chat 会话，父任务提示词要求**同轮并行**三次 `delegate_agent` 委派；
5. 事件流全量落盘 `data/study-events.jsonl`（38,806 事件），`/react/session/events` 拉取官方回放（690 合并事件）；
6. 结果写入本文件夹（`result.md` 结果与效果、`replay-session.md` 回放历史）。

## 验证点

- [x] 三个 delegate_agent 是否真正并行（max_parallel=3）
- [x] 子代理是否按 Skill 工作流执行（先 get_skill、地图→定位→精读）
- [x] 检索工具族分布是否合理（find_symbol/get_file_symbols/search_code/read_file）
- [x] 三份报告 + 父汇总的产出质量（结论是否有代码路径佐证）
- [x] 回放历史可完整拉取并归档
