/**
 * Agent Store - AgentClient 的 SolidJS 状态桥接
 *
 * 包装 AgentClient，为 SolidJS 组件提供响应式状态。
 * 内部订阅 AgentClient 的状态变化，同步更新到 SolidJS store。
 */

import { onCleanup } from "solid-js";
import { createStore, reconcile } from "solid-js/store";
import type { AgentClient } from "../runtime/agent-client";
import type { AgentState } from "../runtime/types";

export interface AgentStore {
  state: AgentState;
  client: AgentClient;
}

export function createAgentStore(client: AgentClient): AgentStore {
  const [state, setState] = createStore<AgentState>(client.getState());

  // Step/ToolCall 没有 `id` 字段，按位置合并才能保留流式快照中的 Solid 代理。
  const unsubscribe = client.subscribe((newState) => {
    if (state.sessionId && newState.sessionId && state.sessionId !== newState.sessionId) {
      setState('steps', []);
    }
    setState(reconcile(newState, { merge: true }));
  });
  onCleanup(unsubscribe);

  return { state, client };
}
