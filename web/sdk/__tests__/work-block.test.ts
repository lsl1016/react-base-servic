import { createComponent, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Step } from '../runtime/types';
import { WorkBlock } from '../ui/components/WorkBlock';
import type { WorkPart } from '../ui/components/turn-segments';

vi.mock('../ui/components/AutoScrollPanel', () => ({
  AutoScrollPanel: (props: { children: unknown }) => props.children,
}));

function createToolStep(toolUseIds: string[]): Step {
  return {
    index: 0,
    runId: 'run-1',
    role: 'assistant',
    thoughts: '',
    content: '',
    toolCalls: toolUseIds.map((toolUseId) => ({
      toolUseId,
      toolName: 'search',
      input: {},
      status: 'running',
      executedBy: 'server',
    })),
    thoughtComplete: true,
    contentStarted: false,
    contentComplete: false,
  };
}

afterEach(() => {
  document.body.innerHTML = '';
});

describe('WorkBlock streaming updates', () => {
  it('reacts when an existing work part receives another tool call', () => {
    const host = document.createElement('div');
    const firstStep = createToolStep(['tool-1']);
    const [parts, setParts] = createSignal<WorkPart[]>([{
      kind: 'tools',
      step: firstStep,
      toolUseIds: ['tool-1'],
    }]);
    const dispose = render(
      () => createComponent(WorkBlock, {
        workId: 'work-1',
        get parts() {
          return parts();
        },
        collapsed: false,
        liveTimer: true,
      }),
      host,
    );

    expect(host.querySelectorAll('.agent-ui-tool-card')).toHaveLength(1);
    const nextStep = createToolStep(['tool-1', 'tool-2']);
    setParts([{
      kind: 'tools',
      step: nextStep,
      toolUseIds: ['tool-1', 'tool-2'],
    }]);

    expect(host.querySelectorAll('.agent-ui-tool-card')).toHaveLength(2);
    dispose();
  });

  it('preserves the user collapsed state across live dependency updates', async () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const step = createToolStep(['tool-1']);
    const [liveUpdate, setLiveUpdate] = createSignal(0);
    const dispose = render(
      () => createComponent(WorkBlock, {
        workId: 'work-1',
        parts: [{ kind: 'tools', step, toolUseIds: ['tool-1'] }],
        collapsed: false,
        get liveTimer() {
          liveUpdate();
          return true;
        },
      }),
      host,
    );
    await Promise.resolve();

    const header = host.querySelector<HTMLButtonElement>('.agent-ui-work-block-header');
    expect(header?.getAttribute('aria-expanded')).toBe('true');
    header?.click();
    expect(header?.getAttribute('aria-expanded')).toBe('false');

    setLiveUpdate(1);
    await Promise.resolve();

    expect(header?.getAttribute('aria-expanded')).toBe('false');
    expect(host.querySelector('.agent-ui-work-block-body')).not.toBeNull();
    expect(host.querySelector('.agent-ui-tool-card')).toBeNull();
    dispose();
  });

  it('uses one labelled disclosure button linked to the work body', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const step = createToolStep(['tool-1']);
    const dispose = render(
      () => createComponent(WorkBlock, {
        workId: 'work-1',
        parts: [{ kind: 'tools', step, toolUseIds: ['tool-1'] }],
        collapsed: false,
        liveTimer: true,
      }),
      host,
    );

    const header = host.querySelector<HTMLButtonElement>('.agent-ui-work-block-header');
    const bodyId = header?.getAttribute('aria-controls');
    expect(host.querySelectorAll('button')).toHaveLength(1);
    expect(header?.getAttribute('aria-label')).toBe('收起工作过程');
    expect(bodyId).toBeTruthy();
    expect(host.querySelector(`#${bodyId}`)).not.toBeNull();
    dispose();
  });
});
