# 多智能体执行正确性：测试场景与验证结果

本目录是多智能体（委派/并行/嵌套/HITL/预算）执行正确性的**场景库与验证档案**：
`scenarios/` 描述每个场景的目的、触发方式、预期与对应测试；`results/` 存放按日期归档的实跑验证记录。

## 场景总览

| # | 场景 | 验证维度 | 自动化 | 场景文档 |
|---|---|---|---|---|
| 1 | 单层委派 | 子 run 落库/agentPath 冒泡/历史隔离/结果回填/回放还原 | e2e | [01-single-delegation.md](scenarios/01-single-delegation.md) |
| 2 | 并行重叠 | 同轮多委派真并发（事件到达交错）+ 结果对位回填 | e2e | [02-parallel-overlap.md](scenarios/02-parallel-overlap.md) |
| 3 | 并行 HITL | 两个子代理同时提问，答案按 toolUseId 路由不交叉 | e2e | [03-parallel-hitl.md](scenarios/03-parallel-hitl.md) |
| 4 | 失败隔离 | 同轮一败一成：软错误回填、父不中断、成功侧照常转述 | e2e | [04-parallel-failure-isolation.md](scenarios/04-parallel-failure-isolation.md) |
| 5 | 嵌套链 | main→A→B 两层嵌套：三段 agentPath + 递归 token 计量 | e2e | [05-nested-chain.md](scenarios/05-nested-chain.md) |
| 6 | 深度限制 | max_depth 达限后子 run 不装配委派工具，自动降级自行完成 | e2e | [06-depth-limit.md](scenarios/06-depth-limit.md) |
| 7 | 断连级联 | 并行执行中用户断连：父与在途子 run 全部收敛，无悬挂 | e2e | [07-disconnect-cascade.md](scenarios/07-disconnect-cascade.md) |
| 8 | 子代理预算 | max_tokens_per_run 超限：子 run 终止 + 原因回填父循环 | e2e | [08-subagent-budget.md](scenarios/08-subagent-budget.md) |
| 9 | 工具确认 | 委派子代理内危险工具确认卡片（嵌套 HITL） | e2e | [09-tool-confirm.md](scenarios/09-tool-confirm.md) |
| 10 | 泳道视图 | 并行执行的前端观测：四道并排、各自思考流实时滚动、恢复还原 | 浏览器 | [10-lane-view.md](scenarios/10-lane-view.md) |

## 运行方式

e2e 需要真实环境（react-base-mysql 容器 + glm 模型 + mcp-server 网关）：

```bash
# 全部多智能体 e2e（约 3~5 分钟）
REACT_DELEGATE_E2E=1 go test ./service/react/ -run 'TestDelegate|TestToolConfirm|TestSkillTrigger|TestSubAgentBudget|TestParallelOverlap|TestParallelFailureIsolation|TestNestedDelegationChain|TestParallelDisconnectCascade' -v -timeout 1800s

# 单场景示例（并行重叠）
REACT_DELEGATE_E2E=1 go test ./service/react/ -run TestParallelOverlapE2E -v -timeout 600s
```

前置条件：
- `conf/mount/custom.yaml` 的 `llm.react.subagent.enabled: true`（max_parallel ≥ 2 才能验证并行，测试内会临时覆写）
- `react-base-mysql` 容器运行中（默认 3317）；`demo-app` caller 与 apikey 已注册
- mcp-server 网关（127.0.0.1:18080）用于带真实工具的场景（场景 1/9）
- 模型走 `claude`（glm-4.6 Anthropic 协议）

## 结果归档约定

每次整批验证后在 `results/` 新建 `YYYY-MM-DD-<主题>.md`，记录：运行的提交号、每个场景的
结论（PASS/FAIL）、关键证据（日志行 / DB 断言值 / 截图 artifact 路径）、环境说明与偏差。
