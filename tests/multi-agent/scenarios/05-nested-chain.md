# 场景 05：嵌套委派链与递归计量

- **测试函数**：`TestNestedDelegationChainE2E`（service/react/multi_agent_e2e_test.go）
- **维度**：两层嵌套（max_depth=2 内合法链）、三段 agentPath、递归 token 计量

## 目的

main → planner-agent → echo-agent 的两层委派链（13*13=169）：验证孙 run 的路径拼接、
隔离子 run 内仍可再委派（工具与清单继承）、以及 OH delegate:{id} 口径的**递归计量**
（echo 的消耗累计进 planner 的 delegated 列，planner 的递归口径再累计进父）。

## 触发

```
请调用 delegate_agent 把任务「计算 13*13 的值并报告」委派给子代理 planner-agent
（agent_key: planner-agent），拿到结论后转述。
```

（planner-agent 的系统提示词要求它优先把算术部分再委派给 echo-agent。）

## 预期

1. planner 子 run：agentPath=main/planner-agent，finished
2. 孙 run：agentPath=**main/planner-agent/echo-agent**（三段），finished，回复含 169
3. 递归计量（output 口径稳定；input 受前缀缓存影响可为 0 仅记录）：
   - planner.delegated_output > 0（echo 累计进来）
   - 父.delegated_output ≥ planner.TotalOutput + planner.DelegatedOutput（递归口径）
4. 父最终回复转述 169

## 结果

见 [results/](../results/) 最新归档。
