# 场景 04：并行失败隔离

- **测试函数**：`TestParallelFailureIsolationE2E`（service/react/multi_agent_e2e_test.go）
- **维度**：同轮多委派中单个失败的隔离性、软错误回填、父 run 不中断

## 目的

同轮并行委派两个任务：一个必失败（`max_tokens_per_run: 1` 的预算探针，任意一轮真实
模型调用都超限）、一个成功（echo 计算 7*8）。验证「一败一成」时失败被隔离在子 run、
父 run 正常收尾、两条工具结果（isError 文本 + finalResponse JSON）都回到父循环。

## 触发

```
请在同一次回复中同时发起两次 delegate_agent 调用（互不依赖）：
①把「回答 ok」委派给 e2e-budget-probe；②把「计算 7*8 并只回答数字」委派给 echo-agent。
无论第一个成败，请把第二个的结果转述，并说明第一个的状态。
```

## 预期

1. 预算探针子 run：state=error，error_message 含「预算超限」
2. echo 子 run：finished，最终回复含 56
3. 父 run 正常 finished（不因单个子失败中断）
4. 父的 tool_result 消息：一条含「预算超限」（isError 软错误）、一条含 56（成功）
5. 父最终回复转述 56 并说明失败侧状态

## 结果

见 [results/](../results/) 最新归档。
