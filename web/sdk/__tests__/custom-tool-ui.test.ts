import { createComponent } from 'solid-js';
import { createStore } from 'solid-js/store';
import { render } from 'solid-js/web';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ToolCallState } from '../runtime/types';
import type { ClientTool, ClientToolUIContext, ClientToolUIRenderer } from '../tools/types';
import { AssistantTurnBody } from '../ui/components/AssistantTurnBody';
import { ToolCallView } from '../ui/components/ToolCallView';

afterEach(() => {
  document.body.innerHTML = '';
});

function customTool(renderer: ClientToolUIRenderer): ClientTool {
  return {
    name: 'custom_chart',
    ui: { type: 'custom', renderer },
    execute: async () => ({ content: 'ok' }),
  };
}

describe('custom ClientTool UI', () => {
  it('mounts, updates, and unmounts a framework-neutral renderer', async () => {
    const contexts: ClientToolUIContext[] = [];
    const update = vi.fn((context: ClientToolUIContext) => {
      contexts.push(context);
    });
    const unmount = vi.fn();
    const mount = vi.fn((container: HTMLElement, context: ClientToolUIContext) => {
      contexts.push(context);
      container.textContent = `mounted:${context.toolCall.status}`;
      return { update, unmount };
    });
    const renderer: ClientToolUIRenderer = { mount };
    const [toolCall, setToolCall] = createStore<ToolCallState>({
      toolUseId: 'call_1',
      toolName: 'custom_chart',
      description: '创建图表',
      input: { chart: { type: 'bar' } },
      status: 'running',
      executedBy: 'client',
    });
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(ToolCallView, {
        toolCall,
        resolveTool: () => customTool(renderer),
      }),
      host,
    );

    expect(mount).toHaveBeenCalledTimes(1);
    expect(host.textContent).toContain('mounted:running');
    expect(contexts[0].toolCall).not.toBe(toolCall);
    expect(contexts[0].toolCall.input).not.toBe(toolCall.input);
    expect((contexts[0].toolCall.input.chart as object)).not.toBe(toolCall.input.chart);
    expect(Object.isFrozen(contexts[0].toolCall)).toBe(true);
    expect(Object.isFrozen(contexts[0].toolCall.input)).toBe(true);
    expect(typeof contexts[0].ui.codeEditor.create).toBe('function');
    expect(typeof contexts[0].ui.diffEditor.create).toBe('function');

    setToolCall('status', 'done');
    setToolCall('result', 'chart-created');
    setToolCall('meta', { revision: 2, chart: { type: 'line' } });

    await vi.waitFor(() => expect(update).toHaveBeenCalled());
    const latest = contexts.at(-1)!;
    expect(latest.toolCall.status).toBe('done');
    expect(latest.toolCall.result).toBe('chart-created');
    expect(latest.toolCall.meta).toEqual({ revision: 2, chart: { type: 'line' } });
    expect(Object.isFrozen(latest.toolCall.meta)).toBe(true);
    expect(mount).toHaveBeenCalledTimes(1);

    dispose();
    expect(unmount).toHaveBeenCalledTimes(1);
  });

  it('passes persisted meta to the renderer on the initial replay mount', () => {
    const persistedMeta = {
      version: 1,
      original: 'select 1',
      modified: 'select 2',
    };
    const mount = vi.fn((_container: HTMLElement, context: ClientToolUIContext) => {
      expect(context.toolCall.status).toBe('done');
      expect(context.toolCall.meta).toEqual(persistedMeta);
      expect(context.toolCall.meta).not.toBe(persistedMeta);
      return { update: vi.fn(), unmount: vi.fn() };
    });
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(ToolCallView, {
        toolCall: {
          toolUseId: 'call_replay',
          toolName: 'custom_chart',
          input: { old_string: 'select 1', new_string: 'select 2' },
          status: 'done',
          result: 'updated',
          meta: persistedMeta,
          executedBy: 'client',
        },
        resolveTool: () => customTool({ mount }),
      }),
      host,
    );

    expect(mount).toHaveBeenCalledTimes(1);
    dispose();
  });

  it('isolates renderer mount failures', () => {
    const renderer: ClientToolUIRenderer = {
      mount() {
        throw new Error('react root failed');
      },
    };
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(ToolCallView, {
        toolCall: {
          toolUseId: 'call_2',
          toolName: 'custom_chart',
          input: {},
          status: 'running',
          executedBy: 'client',
        },
        resolveTool: () => customTool(renderer),
      }),
      host,
    );

    expect(host.querySelector('.agent-ui-tool-custom-error')?.textContent)
      .toContain('react root failed');
    dispose();
  });

  it('resolves a custom renderer through frontendHint', () => {
    const mount = vi.fn((container: HTMLElement) => {
      container.textContent = 'alias-rendered';
      return { update: vi.fn(), unmount: vi.fn() };
    });
    const renderer: ClientToolUIRenderer = { mount };
    const tool = customTool(renderer);
    tool.aliases = ['custom_chart_alias'];
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(ToolCallView, {
        toolCall: {
          toolUseId: 'call_alias',
          toolName: 'execute_tool',
          frontendHint: 'custom_chart_alias',
          input: {},
          status: 'waiting',
          executedBy: 'client',
        },
        resolveTool: (toolName, frontendHint) => [tool.name, ...(tool.aliases ?? [])].includes(frontendHint ?? toolName)
          ? tool
          : undefined,
      }),
      host,
    );

    expect(mount).toHaveBeenCalledTimes(1);
    expect(host.textContent).toContain('alias-rendered');
    dispose();
  });

  it('renders uiPlacement=root outside WorkBlock', () => {
    const mount = vi.fn((container: HTMLElement) => {
      container.textContent = 'root-tool-rendered';
      return { update: vi.fn(), unmount: vi.fn() };
    });
    const rootTool = customTool({ mount });
    rootTool.uiPlacement = 'root';
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(AssistantTurnBody, {
        items: [{
          kind: 'step',
          step: {
            index: 0,
            runId: 'run_root',
            role: 'assistant',
            thoughts: 'prepare',
            content: '',
            toolCalls: [{
              toolUseId: 'call_root',
              toolName: rootTool.name,
              input: {},
              status: 'waiting',
              executedBy: 'client',
            }],
            thoughtComplete: true,
            contentStarted: false,
            contentComplete: false,
          },
        }],
        isActiveTurn: true,
        isRunning: true,
        resolveTool: () => rootTool,
      }),
      host,
    );

    expect(host.querySelector('.agent-ui-message-root-tool .agent-ui-tool-custom')).not.toBeNull();
    expect(host.querySelector('.agent-ui-work-block .agent-ui-tool-custom')).toBeNull();
    expect(host.textContent).toContain('root-tool-rendered');
    dispose();
  });

  it('remounts the renderer when a reused view receives a different toolUseId', async () => {
    const unmount = vi.fn();
    const mount = vi.fn((container: HTMLElement, context: ClientToolUIContext) => {
      container.textContent = context.toolCall.toolUseId;
      return { update: vi.fn(), unmount };
    });
    const renderer: ClientToolUIRenderer = { mount };
    const tool = customTool(renderer);
    const [toolCall, setToolCall] = createStore<ToolCallState>({
      toolUseId: 'call_first',
      toolName: tool.name,
      input: {},
      status: 'done',
      executedBy: 'client',
    });
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(ToolCallView, {
        toolCall,
        resolveTool: () => tool,
      }),
      host,
    );

    expect(mount).toHaveBeenCalledTimes(1);
    expect(host.textContent).toContain('call_first');

    setToolCall({
      toolUseId: 'call_second',
      toolName: tool.name,
      input: {},
      status: 'waiting',
      executedBy: 'client',
    });

    await vi.waitFor(() => expect(mount).toHaveBeenCalledTimes(2));
    expect(unmount).toHaveBeenCalledTimes(1);
    expect(host.textContent).toContain('call_second');

    dispose();
    expect(unmount).toHaveBeenCalledTimes(2);
  });
});
