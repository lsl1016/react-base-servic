import { afterEach, describe, expect, it, vi } from 'vitest';
import { AsyncTaskManager } from '../runtime/async-task-manager';
import type { AsyncTaskItem, AsyncTaskListResp } from '../session/types';

function task(status: AsyncTaskItem['executionStatus']): AsyncTaskItem {
  return {
    toolUseId: 'toolu_1',
    toolName: 'run_query',
    scope: 'outer',
    state: 'pending',
    executionStatus: status,
    createdAt: '2026-09-02 10:00:00',
    updatedAt: `2026-09-02 10:00:0${status === 'succeeded' ? '1' : '2'}`,
  };
}

afterEach(() => {
  vi.useRealTimers();
});

describe('AsyncTaskManager', () => {
  it('会自动取完当前会话全部待处理终态任务的游标分页', async () => {
    const listAsyncTasks = vi.fn()
      .mockResolvedValueOnce({ tasks: [task('succeeded')], hasMore: true, nextCursor: 'next', hasProcessingTasks: false } satisfies AsyncTaskListResp)
      .mockResolvedValueOnce({ tasks: [{ ...task('failed'), toolUseId: 'toolu_2' }], hasMore: false, hasProcessingTasks: false } satisfies AsyncTaskListResp);
    const manager = new AsyncTaskManager({ listAsyncTasks } as any);

    manager.setSession('session_1');
    await vi.waitFor(() => expect(listAsyncTasks).toHaveBeenCalledTimes(2));

    expect(manager.getTasks().map((item) => item.toolUseId)).toEqual(['toolu_1', 'toolu_2']);
    expect(listAsyncTasks).toHaveBeenNthCalledWith(2, { sessionId: 'session_1', cursor: 'next', pageSize: 200 });
    manager.dispose();
  });

  it('空列表但存在 processing 任务时继续轮询，终态出现后停止', async () => {
    vi.useFakeTimers();
    const listAsyncTasks = vi.fn()
      .mockResolvedValueOnce({ tasks: [], hasMore: false, hasProcessingTasks: true } satisfies AsyncTaskListResp)
      .mockResolvedValueOnce({ tasks: [task('succeeded')], hasMore: false, hasProcessingTasks: false } satisfies AsyncTaskListResp);
    const manager = new AsyncTaskManager({ listAsyncTasks } as any);
    const snapshots: string[][] = [];
    manager.subscribe((items) => snapshots.push(items.map((item) => item.executionStatus)));

    manager.setSession('session_1');
    await vi.advanceTimersByTimeAsync(0);
    expect(listAsyncTasks).toHaveBeenCalledTimes(1);
    expect(manager.hasProcessing()).toBe(true);

    await vi.advanceTimersByTimeAsync(5_000);
    expect(listAsyncTasks).toHaveBeenCalledTimes(2);
    expect(manager.getTasks()[0]?.executionStatus).toBe('succeeded');
    expect(manager.hasProcessing()).toBe(false);

    await vi.advanceTimersByTimeAsync(60_000);
    expect(listAsyncTasks).toHaveBeenCalledTimes(2);
    expect(snapshots).toContainEqual(['succeeded']);
    manager.dispose();
  });

  it('切换会话时忽略旧会话的迟到响应', async () => {
    let resolveOld!: (value: AsyncTaskListResp) => void;
    const oldResponse = new Promise<AsyncTaskListResp>((resolve) => { resolveOld = resolve; });
    const listAsyncTasks = vi.fn()
      .mockReturnValueOnce(oldResponse)
      .mockResolvedValueOnce({ tasks: [{ ...task('succeeded'), toolUseId: 'new' }], hasMore: false, hasProcessingTasks: false } satisfies AsyncTaskListResp);
    const manager = new AsyncTaskManager({ listAsyncTasks } as any);

    manager.setSession('old');
    manager.setSession('new');
    await vi.waitFor(() => expect(manager.getTasks()[0]?.toolUseId).toBe('new'));
    resolveOld({ tasks: [{ ...task('succeeded'), toolUseId: 'old' }], hasMore: false, hasProcessingTasks: false });
    await Promise.resolve();

    expect(manager.getTasks()[0]?.toolUseId).toBe('new');
    manager.dispose();
  });
});
