import { describe, expect, it } from 'vitest';
import type { ReactEvent } from '../protocol/types';
import { SessionEventAssembler } from '../runtime/session-event-assembler';

describe('SessionEventAssembler', () => {
  it('should keep content chunks in memory until content_end', () => {
    const assembler = new SessionEventAssembler();

    expect(assembler.consume({
      type: 'content_start',
      seq: 1,
      sessionId: 'session_1',
      runId: 'run_1',
      stepIndex: 0,
    })).toEqual([]);

    expect(assembler.consume({
      type: 'content_delta',
      seq: 2,
      sessionId: 'session_1',
      runId: 'run_1',
      stepIndex: 0,
      payload: { contentDelta: '你' },
    } as ReactEvent)).toEqual([]);

    expect(assembler.consume({
      type: 'content_delta',
      seq: 3,
      sessionId: 'session_1',
      runId: 'run_1',
      stepIndex: 0,
      payload: { contentDelta: '好' },
    } as ReactEvent)).toEqual([]);

    const completed = assembler.consume({
      type: 'content_end',
      seq: 4,
      sessionId: 'session_1',
      runId: 'run_1',
      stepIndex: 0,
      payload: { content: '你好' },
    } as ReactEvent);

    expect(completed.map((event) => event.type)).toEqual(['content_start', 'content_delta', 'content_end']);
    expect(completed[1].payload).toEqual({ contentDelta: '你好' });
    expect(completed[2].payload).toEqual({ content: '你好' });
  });

  it('should keep thought chunks in memory until thought_end', () => {
    const assembler = new SessionEventAssembler();

    assembler.consume({ type: 'thought_start', seq: 1, sessionId: 'session_1', runId: 'run_1', stepIndex: 0 });
    assembler.consume({
      type: 'thought_delta',
      seq: 2,
      sessionId: 'session_1',
      runId: 'run_1',
      stepIndex: 0,
      payload: { contentDelta: '思考' },
    } as ReactEvent);

    const completed = assembler.consume({
      type: 'thought_end',
      seq: 3,
      sessionId: 'session_1',
      runId: 'run_1',
      stepIndex: 0,
      payload: { content: '完整思考' },
    } as ReactEvent);

    expect(completed.map((event) => event.type)).toEqual(['thought_start', 'thought_delta', 'thought_end']);
    expect(completed[1].payload).toEqual({ contentDelta: '完整思考' });
    expect(completed[2].payload).toEqual({ content: '完整思考' });
  });

  it('should pass through complete tool and terminal events', () => {
    const assembler = new SessionEventAssembler();
    const event: ReactEvent = {
      type: 'done',
      seq: 10,
      sessionId: 'session_1',
      runId: 'run_1',
      payload: { inputTokens: 1, outputTokens: 2 },
    };

    expect(assembler.consume(event)).toEqual([event]);
  });

  it('should ignore heartbeat and compact_start for persistence', () => {
    const assembler = new SessionEventAssembler();
    expect(assembler.consume({ type: 'heartbeat', seq: 1 })).toEqual([]);
    expect(assembler.consume({ type: 'compact_start', seq: 2, sessionId: 'session_1', runId: 'run_1' } as ReactEvent)).toEqual([]);
  });
});