import { afterEach, describe, expect, it, vi } from 'vitest';
import { SessionManager } from '../session/session-manager';

function createMockFetch(responses: Record<string, any>) {
  return vi.fn(async (url: string, init?: RequestInit) => {
    const path = new URL(url, 'http://test').pathname;
    const body = responses[path];
    if (!body) {
      return { ok: false, status: 404, statusText: 'Not Found', json: async () => ({}) };
    }
    return {
      ok: true,
      status: 200,
      json: async () => body,
    };
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('SessionManager', () => {
  it('should listSessions correctly', async () => {
    const mockFetch = createMockFetch({
      '/react/session/list': {
        errNo: 0,
        errMsg: 'succ',
        data: {
          sessions: [
            { sessionId: 's1', callerKey: 'test', type: 'chat', title: 'Session 1', state: 'active', createdAt: '', updatedAt: '' },
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        },
      },
    });

    const manager = new SessionManager('/react', mockFetch as any);
    const result = await manager.listSessions({
      callerKey: 'test',
      routeValues: ['route_1'],
      type: 'chat',
      keyword: 'Session',
      page: 1,
      pageSize: 20,
    });

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(JSON.parse((mockFetch as any).mock.calls[0][1].body)).toEqual({
      callerKey: 'test',
      routeValues: ['route_1'],
      type: 'chat',
      keyword: 'Session',
      page: 1,
      pageSize: 20,
    });
    expect(result.sessions).toHaveLength(1);
    expect(result.sessions[0].sessionId).toBe('s1');
    expect(result.total).toBe(1);
  });

  it('should getEvents correctly', async () => {
    const mockFetch = createMockFetch({
      '/react/session/events': {
        errNo: 0,
        errMsg: 'succ',
        data: {
          sessionId: 's1',
          title: 'Test Session',
          events: [
            { type: 'thought_start', seq: 1, runId: 'r1', sessionId: 's1', stepIndex: 0 },
          ],
        },
      },
    });

    const manager = new SessionManager('/react', mockFetch as any);
    const result = await manager.getEvents('s1');

    expect(result.sessionId).toBe('s1');
    expect(result.events).toHaveLength(1);
  });

  it('should list async tasks with cursor pagination params', async () => {
    const mockFetch = createMockFetch({
      '/react/async_task/list': {
        errNo: 0,
        errMsg: 'succ',
        data: { tasks: [], nextCursor: 'next', hasMore: true, hasProcessingTasks: true },
      },
    });
    const manager = new SessionManager('/react', mockFetch as any);
    const result = await manager.listAsyncTasks({ sessionId: 's1', cursor: 'cursor', pageSize: 100 });

    expect(result.hasMore).toBe(true);
    expect(result.hasProcessingTasks).toBe(true);
    expect(JSON.parse((mockFetch as any).mock.calls[0][1].body)).toEqual({
      sessionId: 's1',
      cursor: 'cursor',
      pageSize: 100,
    });
  });

  it('should throw on HTTP error', async () => {
    const mockFetch = vi.fn(async () => ({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: async () => ({}),
    }));

    const manager = new SessionManager('/react', mockFetch as any);

    await expect(manager.listSessions({ callerKey: 'test' })).rejects.toThrow('HTTP 500');
  });

  it('should throw on API error', async () => {
    const mockFetch = createMockFetch({
      '/react/session/list': {
        errNo: 1001,
        errMsg: 'Invalid callerKey',
        data: null,
      },
    });

    const manager = new SessionManager('/react', mockFetch as any);

    await expect(manager.listSessions({ callerKey: 'bad' })).rejects.toThrow('API Error [1001]');
  });

  it('should strip trailing slash from baseUrl', async () => {
    const mockFetch = createMockFetch({
      '/react/session/list': {
        errNo: 0,
        errMsg: 'succ',
        data: { sessions: [], total: 0, page: 1, pageSize: 20 },
      },
    });

    const manager = new SessionManager('/react/', mockFetch as any);
    await manager.listSessions({ callerKey: 'test' });

    const calledUrl = (mockFetch as any).mock.calls[0][0];
    expect(calledUrl).toBe('/react/session/list');
  });

  it('should bind the default global fetch to avoid illegal invocation in browsers', async () => {
    const nativeLikeFetch = vi.fn(async function (this: typeof globalThis) {
      if (this !== globalThis) {
        throw new TypeError("Failed to execute 'fetch' on 'Window': Illegal invocation");
      }

      return {
        ok: true,
        status: 200,
        json: async () => ({
          errNo: 0,
          errMsg: 'succ',
          data: { sessions: [], total: 0, page: 1, pageSize: 20 },
        }),
      };
    });
    vi.stubGlobal('fetch', nativeLikeFetch);

    const manager = new SessionManager('/react');
    await manager.listSessions({ callerKey: 'test' });

    expect(nativeLikeFetch).toHaveBeenCalledTimes(1);
  });
});
