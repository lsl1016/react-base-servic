# 场景 02：并行重叠（并行的直接证据）

- **测试函数**：`TestParallelOverlapE2E`（service/react/multi_agent_e2e_test.go）
- **维度**：同轮多委派真并发、事件到达交错、结果对位回填

## 目的

证明「max_parallel ≥ 2 时，同一轮发出的多个 delegate_agent 是真并发而非串行」。
这是泳道视图、委派计量等一切并行语义的前提。

## 触发

```
请在同一次回复中同时发起两次 delegate_agent 调用（互不依赖，不要等第一个完成再发第二个）：
①把「计算 21+21 并只回答数字」委派给 echo-agent；
②把「计算 12*12 并只回答数字」委派给 echo-agent。两个结果都回来后分别转述。
```

（两个委派目标同为 echo-agent——事件按 runId 区分，不按 agentPath。）

## 预期

1. 两个子 run 都 finished，创建时间差 < 10s
2. **事件到达交错**（核心断言）：按收集侧到达序统计两个子 run 的首/末到达位，
   满足 `A.first < B.last && B.first < A.last`。串行执行下 B 的首事件必然晚于 A 的
   末事件，该断言必不成立——这是并行的直接证据（不用 per-run 的 event.Seq，跨 run 不可比）
3. 结果对位回填：父最终回复同时包含 42 与 144

## 结果

见 [results/](../results/) 最新归档。
