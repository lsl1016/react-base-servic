# 场景 01：单层委派

- **测试函数**：`TestDelegateAgentE2E`（service/react/delegate_e2e_test.go）
- **维度**：委派执行、子 run 落库、事件冒泡、历史隔离、结果回填、回放还原

## 目的

主 Agent 调用 `delegate_agent` 把任务委派给子代理（echo-agent 计算 17*23），
验证委派链路的端到端正确性。

## 触发

```
请立即调用 delegate_agent 工具，把任务「计算 17*23 的值」委派给子代理 echo-agent
（agent_key: echo-agent），拿到结论后向用户转述子代理的回复。不要自己计算。
```

## 预期

1. 子 run 落库：`parent_run_id` 指向父 run，`agent_path = main/echo-agent`，模型继承父 run（claude），终态 finished
2. 子 run 最终回复包含 391（17×23）
3. 事件冒泡：存在 `agentPath=main/echo-agent` 的事件，子 done 早于父 done
4. 历史隔离：外层历史装配排除子 run 消息（子消息只经事件通道给前端）
5. 父最终回复转述 391

## 结果

见 [results/](../results/) 最新归档。
