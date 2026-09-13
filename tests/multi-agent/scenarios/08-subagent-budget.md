# 场景 08：子代理预算

- **测试函数**：`TestSubAgentBudgetE2E`（service/react/budget_e2e_test.go）
- **维度**：agent 级递归 token 预算、超限终止、原因回填父循环

## 目的

`max_tokens_per_run: 1` 的预算探针 agent（任意一轮真实模型调用必超限）被委派后，
首轮工具轮次触发预算检查：子 run 终止、父循环收到含「预算超限」的软错误工具结果。

## 触发

```
请立即调用 delegate_agent 工具，把任务「回答 ok」委派给子代理 e2e-budget-probe
（agent_key: e2e-budget-probe），拿到结论或失败原因后向用户转述。
```

（探针 agent 的系统提示词要求先调 get_tool——预算检查点在「还有后续工具轮次」时触发；
已产出最终回答的轮次保留完成态，OH max_budget_per_run 同款语义。）

## 预期

1. 子 run state=error，error_message 含「预算超限（累计 N tokens > 上限 1...）」
2. 父 run 的 tool_result 消息含「预算超限」（软错误回填）
3. 父 run 正常收尾并转述失败原因

## 结果

见 [results/](../results/) 最新归档。
