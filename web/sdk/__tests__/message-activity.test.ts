import { describe, expect, it } from 'vitest';
import type { Step } from '../runtime/types';
import { getActivityGapSignature, activeTurnHasLiveWork, shouldShowDelayedActivityIndicator } from '../ui/components/message-activity';

function createAssistantStep(overrides: Partial<Step> = {}): Step {
  return {
    index: 0,
    runId: 'run_1',
    role: 'assistant',
    thoughts: '',
    content: '',
    toolCalls: [],
    thoughtComplete: false,
    contentStarted: false,
    contentComplete: false,
    ...overrides,
  };
}

function createUserStep(overrides: Partial<Step> = {}): Step {
  return {
    index: 0,
    runId: 'run_1',
    role: 'user',
    thoughts: '',
    content: '用户问题',
    toolCalls: [],
    thoughtComplete: false,
    contentStarted: false,
    contentComplete: true,
    ...overrides,
  };
}

describe('message activity gap logic', () => {
  it('does not show a gap while WorkBlock is live on the active turn', () => {
    const steps = [
      createAssistantStep({ thoughts: 'thinking', thoughtComplete: true, contentComplete: false }),
    ];

    expect(activeTurnHasLiveWork(steps, true)).toBe(true);
    expect(shouldShowDelayedActivityIndicator(steps, true, false)).toBe(false);
  });

  it('shows a gap after thought completed when work is no longer live', () => {
    const steps = [
      createAssistantStep({
        thoughts: 'thinking',
        thoughtComplete: true,
        contentStarted: true,
        content: 'partial',
        contentComplete: false,
      }),
    ];

    expect(activeTurnHasLiveWork(steps, true)).toBe(false);
    expect(shouldShowDelayedActivityIndicator(steps, true, false)).toBe(false);
  });

  it('does not show a gap after tool completed while WorkBlock is still live', () => {
    const steps = [
      createAssistantStep({
        toolCalls: [{
          toolUseId: 'tool_1',
          toolName: 'Read',
          input: {},
          status: 'done',
          executedBy: 'server',
        }],
      }),
    ];

    expect(shouldShowDelayedActivityIndicator(steps, true, false)).toBe(false);
  });

  it('shows a gap after content completed and the run is still active', () => {
    const steps = [
      createUserStep({ index: 0, content: '第一问' }),
      createAssistantStep({ index: 1, content: '第一答', contentComplete: true }),
    ];

    expect(shouldShowDelayedActivityIndicator(steps, true, false)).toBe(true);
    expect(getActivityGapSignature(steps, true, false)).toContain('content');
  });

  it('does not show a gap after compact while trailing Work is still live', () => {
    const steps = [
      createUserStep({ index: 0, content: '第一问' }),
      {
        index: 1,
        runId: 'run_1',
        role: 'compact' as const,
        thoughts: '',
        content: '',
        toolCalls: [],
        thoughtComplete: true,
        contentStarted: false,
        contentComplete: true,
        compact: {
          beforeMessageCount: 10,
          afterMessageCount: 3,
          summary: '摘要',
        },
      },
    ];

    expect(shouldShowDelayedActivityIndicator(steps, true, false)).toBe(false);
  });

  it('shows a gap when the first assistant event has not arrived yet after the user message', () => {
    const steps = [createUserStep()];

    expect(shouldShowDelayedActivityIndicator(steps, true, false)).toBe(true);
    expect(getActivityGapSignature(steps, true, false)).toBe('first-event-gap');
  });

  it('shows a gap for a new user turn even when the session already has history', () => {
    const steps = [
      createUserStep({ index: 0, content: '历史问题' }),
      createAssistantStep({ index: 1, content: '历史回答', contentComplete: true }),
      createUserStep({ index: 2, content: '新问题' }),
    ];

    expect(shouldShowDelayedActivityIndicator(steps, true, false)).toBe(true);
    expect(getActivityGapSignature(steps, true, false)).toBe('first-event-gap');
  });

  it('does not show a gap while content is actively streaming', () => {
    const steps = [
      createAssistantStep({ content: 'partial', contentComplete: false }),
    ];

    expect(shouldShowDelayedActivityIndicator(steps, true, false)).toBe(false);
  });

  it('does not show a gap while compacting has its own indicator', () => {
    const steps = [createAssistantStep({ thoughts: 'thinking', thoughtComplete: true })];
    expect(shouldShowDelayedActivityIndicator(steps, true, true)).toBe(false);
  });

  it('does not show a gap when the run is not active', () => {
    const steps = [createAssistantStep({ thoughts: 'thinking', thoughtComplete: true })];
    expect(shouldShowDelayedActivityIndicator(steps, false, false)).toBe(false);
  });

  it('detects live WorkBlock on the active turn', () => {
    const steps = [
      createUserStep(),
      createAssistantStep({ thoughts: 'streaming', thoughtComplete: false }),
    ];
    expect(activeTurnHasLiveWork(steps, true)).toBe(true);
    expect(activeTurnHasLiveWork(steps, false)).toBe(false);
  });

  it('does not treat ended work before content as live work', () => {
    const steps = [
      createUserStep(),
      createAssistantStep({
        thoughts: 'done',
        thoughtComplete: true,
        contentStarted: true,
        content: 'answer',
        contentComplete: false,
      }),
    ];
    expect(activeTurnHasLiveWork(steps, true)).toBe(false);
  });

});
