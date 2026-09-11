import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AgentClient } from '../runtime/agent-client';

vi.mock('../ui/components/AgentPanel', () => ({
  AgentPanel: () => {
    const element = document.createElement('div');
    element.className = 'agent-ui-panel';
    return element;
  },
}));

import { mountAgentUI } from '../ui/mount';

describe('Agent UI theme', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  it('applies the initial theme and switches without remounting', () => {
    const unsubscribe = vi.fn();
    const client = {
      subscribe: vi.fn(() => unsubscribe),
    } as unknown as AgentClient;
    const container = document.createElement('div');
    document.body.appendChild(container);

    const handle = mountAgentUI(container, client, { theme: 'dark' });
    const root = container.querySelector('.agent-ui');

    expect(root?.getAttribute('data-agent-ui-theme')).toBe('dark');
    expect(handle.getTheme()).toBe('dark');

    handle.setTheme('light');
    expect(container.querySelector('.agent-ui')).toBe(root);
    expect(root?.getAttribute('data-agent-ui-theme')).toBe('light');
    expect(handle.getTheme()).toBe('light');

    handle.unmount();
    expect(unsubscribe).toHaveBeenCalledTimes(1);
  });

  it('defaults to the light theme', () => {
    const client = {
      subscribe: vi.fn(() => vi.fn()),
    } as unknown as AgentClient;
    const container = document.createElement('div');

    const handle = mountAgentUI(container, client);

    expect(container.querySelector('.agent-ui')?.getAttribute('data-agent-ui-theme')).toBe('light');
    expect(handle.getTheme()).toBe('light');
    handle.unmount();
  });
});
