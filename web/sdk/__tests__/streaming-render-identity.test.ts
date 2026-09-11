import { createComponent, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Step } from '../runtime/types';
import { MessageList } from '../ui/components/MessageList';

vi.mock('../ui/components/SmartScroll', () => ({
  SmartScroll: (props: { children: unknown }) => props.children,
}));

function createStep(overrides: Partial<Step>): Step {
  return {
    index: 0,
    runId: 'run-1',
    role: 'assistant',
    thoughts: '',
    content: '',
    toolCalls: [],
    thoughtComplete: true,
    contentStarted: true,
    contentComplete: false,
    ...overrides,
  };
}

afterEach(() => {
  document.body.innerHTML = '';
});

describe('streaming render identity', () => {
  it('preserves the conversation, turn, and code block while content streams', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const initialSteps = [
      createStep({ index: -1, role: 'user', content: '写一段代码', contentStarted: false }),
      createStep({ content: '```typescript\nconst answer = 4;\n```' }),
    ];
    const [steps, setSteps] = createSignal(initialSteps);
    const dispose = render(
      () => createComponent(MessageList, {
        get steps() {
          return steps();
        },
        isRunning: true,
      }),
      host,
    );

    const section = host.querySelector('.agent-ui-message-section');
    const assistantMessage = host.querySelector('.agent-ui-message-assistant');
    host.querySelector<HTMLButtonElement>('button[aria-label="全屏"]')?.click();
    const fullscreenBlock = document.body.querySelector('.agent-ui-code-block-fullscreen');

    setSteps(initialSteps.map((step) => ({
      ...step,
      toolCalls: [...step.toolCalls],
      content: step.role === 'assistant'
        ? '```typescript\nconst answer = 42;\n```'
        : step.content,
    })));

    expect(host.querySelector('.agent-ui-message-section')).toBe(section);
    expect(host.querySelector('.agent-ui-message-assistant')).toBe(assistantMessage);
    expect(document.body.querySelector('.agent-ui-code-block-fullscreen')).toBe(fullscreenBlock);
    expect(fullscreenBlock?.textContent).toContain('42');
    dispose();
  });
});
