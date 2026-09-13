# 场景 06：深度限制与自动降级

- **测试函数**：`TestDelegateDepthLimitE2E`（service/react/delegate_parallel_e2e_test.go）
- **维度**：max_depth 嵌套上限、达限子 run 不装配委派工具、自动降级自行完成

## 目的

`max_depth=1` 时二级委派被禁止：planner-agent 的子 run 根本看不到 delegate_agent 工具，
按其系统提示词的降级指示自行完成计算——验证深度防线是「工具不装配」而非「调用时报错」。

## 触发

```
请调用 delegate_agent 把任务「计算 12*12 的值并报告」委派给子代理 planner-agent
（agent_key: planner-agent），拿到结论后转述。
```

（planner-agent 被要求优先委派 echo-agent，但 depth=1 下无委派工具可用。）

## 预期

1. 只有 1 个子 run（main/planner-agent），finished
2. 无孙 run（planner 没有任何子 run）
3. planner 自行算出 144（降级完成，不是失败）

## 结果

见 [results/](../results/) 最新归档。
