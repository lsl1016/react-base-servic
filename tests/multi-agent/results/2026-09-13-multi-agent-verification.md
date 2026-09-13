# 2026-09-13 多智能体执行正确性验证

- **代码基线**：本归档所属提交（HEAD 之前为 `0fcf4a8` 泳道视图 + 并行开启）
- **环境**：react-base-mysql（3317）+ glm-4.6（claude 协议）+ mcp-server 网关（18080）；
  `subagent.max_parallel=3`（e2e 内按需覆写为 2）
- **命令**：`REACT_DELEGATE_E2E=1 go test ./service/react/ -run 'TestDelegateAgentE2E$|TestDelegateParallelHitlE2E|TestDelegateDepthLimitE2E|TestParallelOverlapE2E|TestParallelFailureIsolationE2E|TestNestedDelegationChainE2E|TestParallelDisconnectCascadeE2E|TestSubAgentBudgetE2E' -v -timeout 1500s`

## 结果：8/8 PASS（223.4s）

| 场景 | 结果 | 耗时 | 关键证据 |
|---|---|---|---|
| 01 单层委派 | ✅ | 16.0s | 子 run 落库带 parent_run_id/agent_path=main/echo-agent，子事件 163 条回放还原 8 条会话级事件，最终转述 391 |
| 02 并行重叠 | ✅ | 24.5s | 两子 run 存在期相交 a=[18:27:10~11] b=[18:27:10~18]（同秒创建），父回复含 42/144 |
| 03 并行 HITL | ✅ | 51.2s | 先等待=main/hitl-a 后等待=main/hitl-b；hitl-a 收到「苹果」hitl-b 收到「蓝色」，无交叉 |
| 04 失败隔离 | ✅ | 21.7s | 失败=[main/e2e-budget-probe]（预算超限 error）成功=[main/echo-agent]（56），父两结果齐备并转述 56 |
| 05 嵌套链 | ✅ | 34.2s | 孙=[main/planner-agent/echo-agent] 169；父 delegated(out)=369 = planner 递归口径 288+81 **精确相等** |
| 06 深度限制 | ✅ | 26.3s | max_depth=1 下无孙 run，planner 降级自行算出 144 |
| 07 断连级联 | ✅ | 16.4s | main=error, hitl-a=error, hitl-b=error——并行等待中 断连后全部收敛，无悬挂 |
| 08 子代理预算 | ✅ | 32.2s | 子 run error 含「预算超限」，父 tool_result 回填原因并转述 |

## 本轮发现并修复的引擎缺陷

**clientHub 迟到等待者挂死**（场景 07 首跑抓出，10 分钟超时挂死）：

- 现象：并行委派中断连，pump 广播错误后永久退出；「断连时还在模型调用、稍后才进入
  ask_question 等待」的子 run 注册等待者后**永远阻塞**在 w.ch，父 run 的 wg.Wait 挂死，
  run 永不终态（生产上表现为断连后 run 悬挂，只能靠陈旧 run 清理兜底）。
- 修复：hub 增加 deadErr 终结标记，迟到等待者注册时立即返回断连错误。
- 防回归单测：`TestClientHubLateWaiterAfterDisconnect`（client_hub_test.go，7 个 hub 单测全过）。

## 相关验证（同日早些时候）

- **场景 09 工具确认（浏览器）**：exchange_rate confirm 模式下，委派 finance-agent 内
  确认卡片渲染/拒绝/允许全链路通过（拒绝回填「执行被拒绝」、允许返回 0.8153 CHF）。
- **场景 10 泳道视图（浏览器）**：同一轮三次委派（geo/finance/echo）三子 run 同秒并发，
  四道并排实时思考流（echo 逐步验算 391 / finance 汇率推理 / geo 上游重试思考可见）；
  刷新恢复同样还原四道。截图 artifact 存于当次会话记录。
- 同日还发现并修复：EventLedger 本地缓存漏存 agentPath（恢复态徽章/泳道丢失）。

## 环境偏差说明

- 前缀缓存开启时模型 usage 的 input_tokens 常为 0（计入 cache_read_tokens），
  因此递归计量断言采用 output 口径（本轮实测父=369 与子递归 369 精确相等），
  input 数值仅记录不断言。
- 事件到达序在两个并行子 run 间可能整块相邻（一方先流完），不构成串行判据；
  并行证据以 DB 存在期相交为准（场景 02）。
