# 场景 07：并行中断连级联

- **测试函数**：`TestParallelDisconnectCascadeE2E`（service/react/multi_agent_e2e_test.go）
- **维度**：并行执行中客户端断连的取消级联、无悬挂 run

## 目的

两个子代理并行且同时 ask_question 等待中，readClient 返回断连（模拟用户关闭页面）：
父 run 与全部在途子 run 必须收敛到终态，不遗留悬挂 run。

> 本场景在 2026-09-13 首次运行时**抓出引擎真 bug**：clientHub 的 pump 在断连后永久退出，
> 「断连时还在模型调用、稍后才进入等待」的子 run 注册等待者后永远阻塞，父 run 的
> wg.Wait 挂死。已修复（deadErr 标记 + 迟到等待者立即返回断连错误，含单测
> `TestClientHubLateWaiterAfterDisconnect` 防回归）。

## 触发

主 Agent 提示「同一次回复中同时发起两次委派 hitl-a/hitl-b」；writer 捕获 waiting 的
ask_question 后 readClient 返回 `ErrReactClientDisconnected`（60s 兜底：模型偶发只发
一个委派时同样推进）。

## 预期

1. RunWithClientReaderContext 返回错误（断连），带出 runID
2. 轮询 30s 内：会话内全部 run（父 + ≥1 个子）终态非 running/waiting_client_message

## 结果

见 [results/](../results/) 最新归档。
