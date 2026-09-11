/**
 * 客户端工具类型定义
 *
 * ClientTool 是前端（客户端）注册的工具，当后端模型决定调用该工具时：
 * 1. 服务端通过 WS 发 client_tool_use_start 事件
 * 2. SDK 的 ClientToolExecutor 在注册表中查找该工具
 * 3. 调用 execute(input) 执行工具逻辑
 * 4. 将结果通过 client_tool_use_end 消息回填给服务端
 *
 * 与 server 端工具的区别：
 * - server 工具在服务端执行（HTTP 调用外部 API 或内置 Meta Tool）
 * - client 工具在客户端执行（如修改图表、操作编辑器等前端操作）
 */

import type { ToolCallState } from '../runtime/types';

export type Maybe<T> = T extends readonly (infer TItem)[]
  ? Array<Maybe<TItem>> | null | undefined
  : T extends object
    ? ({
        [K in keyof T]?: Maybe<T[K]>;
      } & Record<string, unknown>) | null | undefined
    : T | null | undefined | unknown;

export type JsonValue =
  | string
  | number
  | boolean
  | null
  | { [key: string]: JsonValue }
  | JsonValue[];

/** 客户端工具执行结果 */
export interface ClientToolResult<T = unknown> {
  /** 返回给模型的结果；可直接回填文本，也可回填 JSON 结构 */
  content: JsonValue;
  /** 可选的结构化数据，仅宿主应用内部使用，不会传给模型 */
  data?: T;
  /** 可选的持久化 UI 状态；不会传给模型，会随工具结果用于实时渲染和历史回放 */
  meta?: Record<string, JsonValue>;
  /** 标记执行是否出错，出错时 content 为错误描述 */
  isError?: boolean;
}

export type ClientToolRiskLevel = 'read' | 'write' | 'destructive';

export type ClientToolInputSchema = Record<string, unknown>;

export interface ClientToolExecuteContext {
  /** 工具调用唯一标识 */
  toolUseId: string;
  /** 服务端下发的工具名称，通常是可执行工具 ID */
  toolName: string;
  /** 服务端给出的展示名称 */
  description?: string;
  /** 服务端给出的前端提示，可作为额外匹配标识 */
  frontendHint?: string;
}

export type ClientToolUIType = 'default' | 'explore' | 'custom';

/** 工具 UI 在内置对话面板中的渲染位置；默认在 WorkBlock 内。 */
export type ClientToolUIPlacement = 'inner-worker' | 'root';

export type ClientToolEditorTheme = 'light' | 'dark';

export interface ClientToolCodeEditorOptions {
  value?: string;
  /** 用于按扩展名推断 highlight.js 语言；显式 language 优先。 */
  fileName?: string;
  language?: string;
  height?: number | string;
  theme?: ClientToolEditorTheme;
  lineNumbers?: boolean;
}

export interface ClientToolDiffEditorOptions {
  /** 标准 unified diff 文本。 */
  value: string;
  /** 用于按扩展名推断代码语言；未传时从 unified diff 文件头读取。 */
  fileName?: string;
  /** diff 中代码正文的语言，用于 highlight.js 高亮。 */
  language?: string;
  height?: number | string;
  theme?: ClientToolEditorTheme;
  onDiffChange?: (summary: ClientToolDiffSummary) => void;
}

export interface ClientToolDiffPatchOptions {
  original: string;
  modified: string;
  oldFileName?: string;
  newFileName?: string;
  oldHeader?: string;
  newHeader?: string;
  contextLines?: number;
}

export interface ClientToolDiffSummary {
  readonly addedLines: number;
  readonly removedLines: number;
}

export interface ClientToolCodeEditorView {
  /** 首次高亮渲染完成。 */
  readonly ready: Promise<void>;
  getValue(): string;
  setValue(value: string): void;
  focus(): void;
  layout(): void;
  dispose(): void;
}

export interface ClientToolDiffEditorView {
  /** 首次渲染完成。 */
  readonly ready: Promise<void>;
  getValue(): string;
  setValue(value: string): void;
  focus(): void;
  layout(): void;
  dispose(): void;
}

export interface ClientToolUIPrimitives {
  readonly codeEditor: {
    create(container: HTMLElement, options?: ClientToolCodeEditorOptions): ClientToolCodeEditorView;
  };
  readonly diffEditor: {
    createPatch(options: ClientToolDiffPatchOptions): string;
    create(container: HTMLElement, options: ClientToolDiffEditorOptions): ClientToolDiffEditorView;
  };
}

export type ClientToolUISnapshot = Readonly<
  Omit<ToolCallState, 'input'> & {
    input: Readonly<Record<string, unknown>>;
  }
>;

export interface ClientToolUIContext {
  /** 当前工具调用的普通对象快照，不包含 Solid 响应式 Proxy */
  readonly toolCall: ClientToolUISnapshot;
  /** SDK 内置的框架无关 UI 绘制能力 */
  readonly ui: ClientToolUIPrimitives;
}

export interface ClientToolUIView {
  /** 同一次 toolUseId 的状态变化时更新外部框架 UI */
  update(context: ClientToolUIContext): void;
  /** 宿主节点移除时卸载外部框架 UI */
  unmount(): void;
}

export interface ClientToolUIRenderer {
  /** 在 SDK 提供的 DOM 容器内挂载 React、Vue 或原生 DOM UI；toolUseId 改变时 SDK 会重新挂载 */
  mount(container: HTMLElement, context: ClientToolUIContext): ClientToolUIView;
}

export interface ClientToolInteractionUI {
  /** 交互 UI 的固定挂载区域；当前支持输入框上方的 feedback 区域。 */
  placement: 'feedback';
  /** 可选的展示条件；用于跳过无需用户操作的等待态，例如前置校验失败。 */
  shouldRender?: (toolCall: Readonly<ToolCallState>) => boolean;
  /** 仅在当前工具等待用户操作时挂载，不参与历史消息回放。 */
  renderer: ClientToolUIRenderer;
}

export interface ClientToolExploreUIOptions {
  /** explore 卡片前缀文案，例如“使用技能” */
  prefixText?: string;
  /** 是否默认展开详情 */
  defaultExpanded?: boolean;
  /** 是否允许折叠/展开，默认 true */
  collapsible?: boolean;
  /** 是否展示输入参数，默认 true */
  showInput?: boolean;
  /** 是否展示结果内容，默认 true */
  showResult?: boolean;
}

export type ClientToolUI =
  | {
      type?: 'default';
    }
  | {
      type: 'explore';
      options?: ClientToolExploreUIOptions;
    }
  | {
      type: 'custom';
      renderer: ClientToolUIRenderer;
    };

/**
 * 客户端工具定义
 *
 * @template TInput  - 工具输入类型（模型传入的参数）
 * @template TResult - 工具结果中的 data 字段类型
 */
export interface ClientTool<TInput = unknown, TResult = unknown> {
  /** 工具名称，需与服务端 client_tool_use_start.toolName 或 frontendHint 一致 */
  name: string;
  /** 供模型理解工具能力的说明；客户端工具可选声明 */
  description?: string;
  /** 工具风险等级；客户端工具可选声明 */
  riskLevel?: ClientToolRiskLevel;
  /** 是否需要确认；客户端工具可选声明 */
  requiresConfirmation?: boolean;
  /** 工具入参 JSON Schema；客户端工具可选声明 */
  inputSchema?: ClientToolInputSchema;
  /** 额外匹配名称，用于兼容 toolId、callName 等不同服务端标识 */
  aliases?: string[];
  /** 工具对应的 UI 展示配置；不传时使用默认卡片 */
  ui?: ClientToolUI;
  /** 工具 UI 的渲染位置；默认 inner-worker，root 表示作为一轮消息的根级块展示 */
  uiPlacement?: ClientToolUIPlacement;
  /** 等待用户操作时展示的临时交互 UI；与消息流中的 ui 相互独立 */
  interactionUI?: ClientToolInteractionUI;
  /** 工具执行函数，接收模型传入的原始输入，调用方必须做运行时校验 */
  execute: (input: Maybe<TInput>, context: ClientToolExecuteContext) => Promise<ClientToolResult<TResult>>;
}

/**
 * 定义客户端工具的辅助函数
 *
 * 纯类型透传，提供 IDE 类型推断。用法：
 *
 * @example
 * const myTool = defineClientTool<{ chartType: string }, { chartHash: string }>({
 *   name: 'MutateChart',
 *   aliases: ['tool_xxx'],
 *   execute: async (input) => ({
 *     content: `图表 ${input.chartType} 已创建`,
 *     data: { chartHash: 'abc123' },
 *   }),
 * });
 */
export function defineClientTool<TInput = unknown, TResult = unknown>(
  tool: ClientTool<TInput, TResult>,
): ClientTool<TInput, TResult> {
  return tool;
}
