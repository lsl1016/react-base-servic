import { createRoot } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { AgentClient } from '../runtime/agent-client';
import type { AgentState, Step } from '../runtime/types';
import { createAgentStore } from '../ui/store';

function createStep(content: string): Step {
  return {
    index: 0,
    runId: 'run-1',
    role: 'assistant',
    thoughts: '',
    content,
    toolCalls: [{
      toolUseId: 'tool-1',
      toolName: 'search',
      input: { query: 'test' },
      status: 'running',
      executedBy: 'server',
    }],
    thoughtComplete: true,
    contentStarted: true,
    contentComplete: false,
  };
}

function createState(content: string, sessionId = 'session-1'): AgentState {
  return {
    status: 'running',
    connected: true,
    sessionId,
    currentRunId: 'run-1',
    steps: [createStep(content)],
    todos: [],
    usage: {
      totalInputTokens: 0,
      totalOutputTokens: 0,
      totalCacheReadTokens: 0,
      totalCacheCreateTokens: 0,
      runCount: 0,
    },
    lastRunStats: null,
    compactState: null,
  };
}

describe('createAgentStore', () => {
  it('preserves step and tool identities across streamed snapshots', () => {
    let listener: ((state: AgentState) => void) | undefined;
    const initialState = createState('开始');
    const client = {
      getState: () => initialState,
      subscribe: (fn: (state: AgentState) => void) => {
        listener = fn;
        fn(initialState);
        return () => undefined;
      },
    } as unknown as AgentClient;

    createRoot((dispose) => {
      const store = createAgentStore(client);
      const steps = store.state.steps;
      const step = steps[0];
      const toolCall = step.toolCalls[0];

      listener?.(createState('开始生成更多内容'));

      expect(store.state.steps).toBe(steps);
      expect(store.state.steps[0]).toBe(step);
      expect(store.state.steps[0].toolCalls[0]).toBe(toolCall);
      expect(store.state.steps[0].content).toBe('开始生成更多内容');

      listener?.(createState('另一个会话', 'session-2'));
      expect(store.state.steps[0]).not.toBe(step);
      expect(store.state.steps[0].content).toBe('另一个会话');
      dispose();
    });
  });
});
