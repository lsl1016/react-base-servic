import { Show, createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import type { ToolCallState } from '../../runtime/types';
import type {
  ClientToolUIContext,
  ClientToolUIRenderer,
  ClientToolUISnapshot,
  ClientToolUIView,
} from '../../tools/types';
import { clientToolUI } from '../client-tool-ui';

export interface CustomToolViewProps {
  toolCall: ToolCallState;
  renderer: ClientToolUIRenderer;
}

function cloneToolRecord(value: Record<string, unknown>): Record<string, unknown> {
  try {
    return JSON.parse(JSON.stringify(value)) as Record<string, unknown>;
  } catch {
    return { ...value };
  }
}

export function snapshotToolCall(toolCall: ToolCallState): ClientToolUISnapshot {
  const input = Object.freeze(cloneToolRecord(toolCall.input));
  const meta = toolCall.meta ? Object.freeze(cloneToolRecord(toolCall.meta)) : undefined;
  return Object.freeze({
    toolUseId: toolCall.toolUseId,
    toolName: toolCall.toolName,
    description: toolCall.description,
    frontendHint: toolCall.frontendHint,
    input,
    status: toolCall.status,
    result: toolCall.result,
    meta,
    isError: toolCall.isError,
    executedBy: toolCall.executedBy,
    durationMs: toolCall.durationMs,
    afterContent: toolCall.afterContent,
  });
}

export function CustomToolView(props: CustomToolViewProps) {
  let container!: HTMLDivElement;
  let view: ClientToolUIView | undefined;
  let mountedContext: ClientToolUIContext | undefined;
  let mountedRenderer: ClientToolUIRenderer | undefined;
  const [renderError, setRenderError] = createSignal('');
  const context = createMemo<ClientToolUIContext>(() => Object.freeze({
    toolCall: snapshotToolCall(props.toolCall),
    ui: clientToolUI,
  }));

  const unmountView = () => {
    if (!view) return;
    try {
      view.unmount();
    } catch {
      // 外部框架卸载异常不能阻断 SDK 自身清理或下一次挂载。
    }
    view = undefined;
    mountedContext = undefined;
    mountedRenderer = undefined;
  };

  const mountView = (nextContext: ClientToolUIContext, renderer: ClientToolUIRenderer) => {
    mountedContext = nextContext;
    mountedRenderer = renderer;
    setRenderError('');
    try {
      const mountedView = renderer.mount(container, nextContext);
      if (!mountedView || typeof mountedView.update !== 'function' || typeof mountedView.unmount !== 'function') {
        throw new Error('renderer.mount() 必须返回 update() 和 unmount()');
      }
      view = mountedView;
    } catch (error) {
      setRenderError(error instanceof Error ? error.message : '未知错误');
    }
  };

  onMount(() => {
    mountView(context(), props.renderer);
  });

  createEffect(() => {
    const nextContext = context();
    const nextRenderer = props.renderer;
    if (!view) return;
    if (
      nextContext.toolCall.toolUseId !== mountedContext?.toolCall.toolUseId
      || nextRenderer !== mountedRenderer
    ) {
      unmountView();
      container.replaceChildren();
      mountView(nextContext, nextRenderer);
      return;
    }
    if (nextContext === mountedContext) return;
    try {
      view.update(nextContext);
      mountedContext = nextContext;
    } catch (error) {
      setRenderError(error instanceof Error ? error.message : '未知错误');
    }
  });

  onCleanup(() => {
    unmountView();
  });

  return (
    <div class="agent-ui-tool-custom">
      <div ref={container} class="agent-ui-tool-custom-host" />
      <Show when={renderError()}>
        {(message) => <div class="agent-ui-tool-custom-error">自定义工具 UI 渲染失败：{message()}</div>}
      </Show>
    </div>
  );
}

export default CustomToolView;
