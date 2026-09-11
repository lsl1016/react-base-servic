/**
 * Session 管理器
 *
 * 封装 react-base-service 的 HTTP 接口，提供会话管理能力：
 * - listSessions: 查询会话列表（支持分页、关键词搜索）
 * - getEvents: 获取指定会话的全部历史事件（用于会话切换时还原状态）
 *
 * 统一处理后端响应格式 { errNo, errMsg, data }：
 * - errNo=0 → 返回 data
 * - errNo!=0 → 抛出 Error
 * - HTTP 非 2xx → 抛出 Error
 *
 * 构造函数接受可选的 fetch 函数注入，方便测试时 mock。
 */
import type { ApiResponse, PlanExecutionDetailResp, PlanStepEventsResp } from '../protocol/types';
import type {
  AsyncTaskListParams,
  AsyncTaskListResp,
  RunFeedbackParams,
  RunFeedbackResp,
  SessionEventsResp,
  SessionFeedbackResp,
  SessionListParams,
  SessionListResp,
} from './types';

export class SessionManager {
  private baseUrl: string;
  private fetchFn: typeof fetch;

  constructor(baseUrl: string, fetchFn?: typeof fetch) {
    const resolvedFetch = fetchFn ?? globalThis.fetch;

    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.fetchFn = resolvedFetch === globalThis.fetch ? resolvedFetch.bind(globalThis) : resolvedFetch;
  }

  /** 查询会话列表 → POST /session/list */
  async listSessions(params: SessionListParams): Promise<SessionListResp> {
    return this.post<SessionListResp>('/session/list', params);
  }

  /** 获取指定会话的全部历史事件 → POST /session/events */
  async getEvents(sessionId: string): Promise<SessionEventsResp> {
    return this.post<SessionEventsResp>('/session/events', { sessionId });
  }

  /** 分页获取指定会话第三方已终态但本地尚未处理的异步任务。 */
  async listAsyncTasks(params: AsyncTaskListParams): Promise<AsyncTaskListResp> {
    return this.post<AsyncTaskListResp>('/async_task/list', params);
  }

  /** 提交单轮反馈 → POST /run/feedback */
  async submitRunFeedback(params: RunFeedbackParams): Promise<RunFeedbackResp> {
    return this.post<RunFeedbackResp>('/run/feedback', params);
  }

  /** 获取指定会话的全部反馈 → POST /session/feedback */
  async getSessionFeedback(sessionId: string): Promise<SessionFeedbackResp> {
    return this.post<SessionFeedbackResp>('/session/feedback', { sessionId });
  }

  /** 查询 Plan 最新公开视图和 Attempt 索引。 */
  async getPlanExecutionDetail(planExecutionId: string, sessionId: string, callerKey: string): Promise<PlanExecutionDetailResp> {
    return this.post<PlanExecutionDetailResp>('/plan_execution/detail', { planExecutionId, sessionId, callerKey });
  }

  /** 查询一个 Plan StepAttempt 的持久化详细事件。 */
  async getPlanStepEvents(planExecutionId: string, stepAttemptId: string, sessionId: string, callerKey: string): Promise<PlanStepEventsResp> {
    return this.post<PlanStepEventsResp>('/plan_execution/events', { planExecutionId, stepAttemptId, sessionId, callerKey });
  }

  private async post<T>(path: string, body: Record<string, unknown> | object): Promise<T> {
    const res = await this.fetchFn(`${this.baseUrl}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });

    if (!res.ok) {
      throw new Error(`HTTP ${res.status}: ${res.statusText}`);
    }

    const json = (await res.json()) as ApiResponse<T>;
    if (json.errNo !== 0) {
      throw new Error(`API Error [${json.errNo}]: ${json.errMsg}`);
    }

    return json.data;
  }
}