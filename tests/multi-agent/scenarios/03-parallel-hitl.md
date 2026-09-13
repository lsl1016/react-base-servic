# 场景 03：并行 HITL（答案按 toolUseId 路由）

- **测试函数**：`TestDelegateParallelHitlE2E`（service/react/delegate_parallel_e2e_test.go）
- **维度**：并行等待中的上行消息路由（clientHub）、答案不交叉不丢失

## 目的

两个子代理（hitl-a 水果 / hitl-b 颜色）同轮并行委派且**同时** ask_question 等待用户作答，
模拟前端按 toolUseId 分别提交答案，验证 clientHub 把答案路由到各自等待者。

## 触发

主 Agent 提示「同一次回复中同时发起两次委派」；writer 捕获两个 waiting 的 ask_question
事件后，readClient 按各自 toolUseId 提交「苹果」与「蓝色」。

## 预期

1. 两个子 run 均为 main/hitl-a 与 main/hitl-b，终态 finished
2. **路由正确性（错配即交叉）**：hitl-a 最终回复含「苹果」且不含「蓝色」；hitl-b 反之
3. 父 run 的 delegated_output_tokens > 0（两个子 run 均有模型消耗）

## 结果

见 [results/](../results/) 最新归档。
