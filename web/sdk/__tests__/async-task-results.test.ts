import { createComponent } from 'solid-js';
import { render } from 'solid-js/web';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { AgentClient } from '../runtime/agent-client';
import type { AgentState } from '../runtime/types';
import type { AsyncTaskItem } from '../session/types';
import { AgentPanel } from '../ui/components/AgentPanel';

vi.mock('../ui/components/SmartScroll', () => ({
  SmartScroll: (props: { children: unknown }) => props.children,
}));
vi.mock('../ui/components/ScrollArea', () => ({
  ScrollArea: (props: { children: unknown }) => props.children,
}));

const state: AgentState = {
  status: 'idle',
  connected: true,
  sessionId: 'session-async-task',
  currentRunId: null,
  steps: [],
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

function terminalTask(overrides: Partial<AsyncTaskItem> = {}): AsyncTaskItem {
  return {
    toolUseId: 'toolu-async-1',
    toolName: '数据查询',
    scope: 'outer',
    state: 'pending',
    executionStatus: 'succeeded',
    createdAt: '2026-09-02 22:00:00',
    updatedAt: '2026-09-02 22:05:00',
    completedAt: '2026-09-02 22:05:00',
    ...overrides,
  };
}

afterEach(() => {
  document.body.innerHTML = '';
  sessionStorage.clear();
});

describe('AgentPanel async task results', () => {
  it('只在终态任务出现后展示入口、通知和任务详情', async () => {
    let taskListener: ((tasks: readonly AsyncTaskItem[]) => void) | undefined;
    const unsubscribeAsyncTasks = vi.fn();
    const client = {
      getState: () => state,
      subscribe: (listener: (next: AgentState) => void) => {
        listener(state);
        return () => undefined;
      },
      subscribeAsyncTasks: (listener: (tasks: readonly AsyncTaskItem[]) => void) => {
        taskListener = listener;
        listener([]);
        return unsubscribeAsyncTasks;
      },
      getRegisteredTool: vi.fn(),
      listCachedSessions: vi.fn(async () => []),
      syncSessions: vi.fn(async () => []),
      uploadAttachment: vi.fn(),
      run: vi.fn(),
      cancel: vi.fn(),
      sendAskQuestionAnswer: vi.fn(),
    } as unknown as AgentClient;
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(() => createComponent(AgentPanel, { client }), host);

    expect(host.querySelector('button[aria-label^="任务结果"]')).toBeNull();

    taskListener?.([terminalTask()]);
    await vi.waitFor(() => expect(host.textContent).toContain('数据查询已完成'));
    const trigger = host.querySelector<HTMLButtonElement>('button[aria-label="任务结果，共 1 条"]');
    expect(trigger).not.toBeNull();

    trigger?.click();
    expect(host.querySelector('[role="dialog"][aria-label="任务结果"]')).not.toBeNull();
    expect(host.textContent).toContain('已完成');
    expect(host.textContent).toContain('09-02 22:05');

    host.querySelector<HTMLButtonElement>('button[title="关闭通知"]')?.click();
    expect(host.querySelector('.agent-ui-async-task-toast')).toBeNull();
    taskListener?.([terminalTask()]);
    await Promise.resolve();
    expect(host.querySelector('.agent-ui-async-task-toast')).toBeNull();

    dispose();
    expect(unsubscribeAsyncTasks).toHaveBeenCalledTimes(1);
  });

  it('失败任务展示错误原因和计划任务标签', async () => {
    let taskListener: ((tasks: readonly AsyncTaskItem[]) => void) | undefined;
    const client = {
      getState: () => state,
      subscribe: (listener: (next: AgentState) => void) => {
        listener(state);
        return () => undefined;
      },
      subscribeAsyncTasks: (listener: (tasks: readonly AsyncTaskItem[]) => void) => {
        taskListener = listener;
        listener([]);
        return () => undefined;
      },
      getRegisteredTool: vi.fn(),
      listCachedSessions: vi.fn(async () => []),
      syncSessions: vi.fn(async () => []),
      uploadAttachment: vi.fn(),
      run: vi.fn(),
      cancel: vi.fn(),
      sendAskQuestionAnswer: vi.fn(),
    } as unknown as AgentClient;
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(() => createComponent(AgentPanel, { client }), host);

    taskListener?.([terminalTask({
      executionStatus: 'failed',
      scope: 'plan_step',
      errorMessage: 'SQL 语法错误，请检查字段名称',
    })]);
    await vi.waitFor(() => expect(host.textContent).toContain('数据查询执行失败'));
    host.querySelector<HTMLButtonElement>('button[aria-label="任务结果，共 1 条"]')?.click();

    expect(host.textContent).toContain('执行失败');
    expect(host.textContent).toContain('SQL 语法错误，请检查字段名称');
    expect(host.textContent).toContain('计划任务');
    expect(host.querySelector('.agent-ui-async-task-item-failed')).not.toBeNull();

    dispose();
  });
});
