# 场景 09：委派子代理内的工具确认（嵌套 HITL）

- **测试函数**：`TestToolConfirm*`（service/react/tool_confirm_e2e_test.go）
- **维度**：子 run 内危险工具的确认门、确认事件带 agentPath 冒泡、并行等待按 toolUseId 路由

## 目的

permission_mode=confirm 的工具（如汇率查询）在**委派子代理内**被调用时：
确认卡片渲染在子代理泳道/嵌套卡片中，允许/拒绝的答案经 clientHub 路由回子 run，
拒绝后子 run 收到 isError 工具结果并向父汇报，父向用户转述。

## 触发（浏览器侧）

playground 中让主 Agent 委派 finance-agent 查汇率（exchange_rate 置 confirm 模式），
子代理调用工具时出现确认卡片，点击「拒绝」/「允许执行」。

## 预期

1. tool_confirm_request 事件带 agentPath（前端归入正确泳道）
2. 拒绝：卡片消失，子 run 工具结果为「用户拒绝」（isError），父转述「执行被拒绝」
3. 允许：工具执行，父转述汇率结果

## 结果

见 [results/](../results/) 最新归档（2026-09-13 浏览器全链路验证含嵌套场景）。
