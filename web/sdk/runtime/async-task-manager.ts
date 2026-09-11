import { SessionManager } from '../session/session-manager';
import type { AsyncTaskItem } from '../session/types';

export type AsyncTaskListener = (tasks: readonly AsyncTaskItem[]) => void;

const POLL_DELAYS_MS = [5_000, 10_000, 30_000];
const PAGE_SIZE = 200;
const MAX_PAGES = 1_000;

function taskFingerprint(tasks: readonly AsyncTaskItem[]): string {
  return JSON.stringify(tasks.map((task) => [
    task.toolUseId,
    task.state,
    task.executionStatus,
    task.providerStatus ?? '',
    task.progress ?? null,
    task.errorMessage ?? '',
    task.updatedAt,
  ]));
}

/**
 * AsyncTaskManager 只轮询本服务的任务表接口，不调用第三方接口，也不感知 Provider 内部实现。
 */
export class AsyncTaskManager {
  private readonly sessionManager: SessionManager;
  private readonly listeners = new Set<AsyncTaskListener>();
  private sessionId: string | null = null;
  private tasks: AsyncTaskItem[] = [];
  private timer: ReturnType<typeof setTimeout> | null = null;
  private generation = 0;
  private pollIndex = 0;
  private hasProcessingTasks = false;
  private disposed = false;

  constructor(sessionManager: SessionManager) {
    this.sessionManager = sessionManager;
    if (typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', this.handleVisibilityChange);
    }
  }

  /** 绑定当前会话；会话切换后立即获取该会话全部待处理终态任务。 */
  setSession(sessionId: string | null): void {
    const normalized = sessionId?.trim() || null;
    if (this.sessionId === normalized) return;
    this.generation += 1;
    this.stopTimer();
    this.sessionId = normalized;
    this.tasks = [];
    this.hasProcessingTasks = false;
    this.pollIndex = 0;
    this.notify();
    if (normalized) {
      void this.refresh().catch((error) => {
        console.warn('[AsyncTaskManager] initial refresh failed:', error);
        this.schedule();
      });
    }
  }

  /** 主动刷新当前会话，并自动取完所有游标分页。 */
  refresh(): Promise<readonly AsyncTaskItem[]> {
    const sessionId = this.sessionId;
    if (!sessionId || this.disposed) return Promise.resolve(this.tasks);
    const generation = this.generation;
    return this.loadAllPages(sessionId).then(({ tasks, hasProcessingTasks }) => {
      if (generation !== this.generation || sessionId !== this.sessionId || this.disposed) {
        return this.tasks;
      }
      const changed = taskFingerprint(this.tasks) !== taskFingerprint(tasks) || this.hasProcessingTasks !== hasProcessingTasks;
      this.tasks = tasks;
      this.hasProcessingTasks = hasProcessingTasks;
      this.pollIndex = changed ? 0 : Math.min(this.pollIndex + 1, POLL_DELAYS_MS.length - 1);
      if (changed) this.notify();
      this.schedule();
      return this.tasks;
    });
  }

  getTasks(): readonly AsyncTaskItem[] {
    return this.tasks;
  }

  /** 当前会话是否仍有 Provider 正在跟踪的任务。 */
  hasProcessing(): boolean {
    return this.hasProcessingTasks;
  }

  subscribe(listener: AsyncTaskListener): () => void {
    this.listeners.add(listener);
    listener(this.tasks);
    return () => this.listeners.delete(listener);
  }

  dispose(): void {
    this.disposed = true;
    this.generation += 1;
    this.stopTimer();
    this.listeners.clear();
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', this.handleVisibilityChange);
    }
  }

  private async loadAllPages(sessionId: string): Promise<{ tasks: AsyncTaskItem[]; hasProcessingTasks: boolean }> {
    const result: AsyncTaskItem[] = [];
    let cursor: string | undefined;
    let hasProcessingTasks = false;
    for (let page = 0; page < MAX_PAGES; page += 1) {
      const response = await this.sessionManager.listAsyncTasks({ sessionId, cursor, pageSize: PAGE_SIZE });
      result.push(...(response.tasks ?? []));
      hasProcessingTasks ||= response.hasProcessingTasks;
      if (!response.hasMore) return { tasks: result, hasProcessingTasks };
      if (!response.nextCursor || response.nextCursor === cursor) {
        throw new Error('异步任务分页游标无效');
      }
      cursor = response.nextCursor;
    }
    throw new Error('异步任务分页数量超过安全上限');
  }

  private schedule(): void {
    this.stopTimer();
    if (this.disposed || !this.sessionId || !this.hasProcessingTasks) return;
    if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return;
    const delay = POLL_DELAYS_MS[this.pollIndex] ?? POLL_DELAYS_MS[POLL_DELAYS_MS.length - 1];
    this.timer = setTimeout(() => {
      this.timer = null;
      void this.refresh().catch((error) => {
        console.warn('[AsyncTaskManager] poll failed:', error);
        this.pollIndex = Math.min(this.pollIndex + 1, POLL_DELAYS_MS.length - 1);
        this.schedule();
      });
    }, delay);
  }

  private stopTimer(): void {
    if (this.timer != null) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }

  private notify(): void {
    for (const listener of this.listeners) listener(this.tasks);
  }

  private handleVisibilityChange = (): void => {
    if (document.visibilityState === 'visible' && this.sessionId) {
      this.pollIndex = 0;
      void this.refresh().catch((error) => {
        console.warn('[AsyncTaskManager] visibility refresh failed:', error);
        this.schedule();
      });
    } else {
      this.stopTimer();
    }
  };
}
