/**
 * MessageItem - 单条消息渲染组件
 *
 * 组合 ThoughtBlock、ContentBlock 和 ToolCallCard，
 * 展示一条完整的消息步骤。
 */

import { For, Show, createMemo } from "solid-js";
import type { AskQuestionAnswerContent } from "../../protocol/types";
import type { Step, ToolCallState } from "../../runtime/types";
import type { ClientTool } from "../../tools/types";
import { InputPartsView } from "../editor/InputPartsView";
import type { AgentQuickInsertItem } from "../editor/types";
import type { NextButtonAction } from "../events";
import { ContentBlock } from "./ContentBlock";
import { ThoughtBlock } from "./ThoughtBlock";
import { ToolCallView } from "./ToolCallView";

export interface MessageItemProps {
  /** 消息步骤数据 */
  step: Step;
  /** 用于把历史文本反解析为快捷节点 */
  quickInsertItems?: AgentQuickInsertItem[];
  /** 根据工具名查找已注册的客户端工具定义 */
  resolveTool?: (toolName: string, frontendHint?: string) => ClientTool | undefined;
  /** 点击快捷下一步 */
  onNextButtonClick?: (action: NextButtonAction) => void;
  /** 提交 ask_question 的用户答案（tool_use_answer 回填） */
  onAskQuestionSubmit?: (toolUseId: string, content: AskQuestionAnswerContent) => void;
  /** 当前活跃的等待作答 ask_question，消息列表中不再重复渲染 */
  activeAskQuestion?: ToolCallState;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function isTodoToolCall(toolCall: ToolCallState): boolean {
  const name = toolCall.toolName.toLowerCase().replace(/[\s_-]/g, '');
  if (name.includes('todo')) return true;

  const input = toolCall.input;
  return Array.isArray(input.todos)
    || Array.isArray(input.items)
    || (isRecord(input.todoState) && Array.isArray(input.todoState.items));
}

export function MessageItem(props: MessageItemProps) {
  const normalToolCalls = createMemo(() => props.step.toolCalls.filter((toolCall) => !isTodoToolCall(toolCall)));

  // 用户消息
  if (props.step.role === 'user') {
    return (
      <div class="agent-ui-message-user">
        <div class="agent-ui-message-content">
          <InputPartsView
            parts={props.step.displayParts}
            fallback={props.step.content}
            quickInsertItems={props.quickInsertItems}
          />
        </div>
      </div>
    );
  }

  // 助手消息（含错误）
  return (
    <>
      <div class="agent-ui-message-assistant" classList={{ 'agent-ui-message-error': props.step.isError }}>
        <div class="agent-ui-message-body">
          {/* 思考过程 */}
          <Show when={props.step.thoughts}>
            <ThoughtBlock
              content={props.step.thoughts}
              complete={props.step.thoughtComplete}
            />
          </Show>

          {/* 回答内容或错误信息 */}
          <Show when={props.step.content}>
            <ContentBlock
              content={props.step.content}
              complete={props.step.contentComplete}
              onNextButtonClick={(buttonText, buttonIndex) => props.onNextButtonClick?.({
                sourceRunId: props.step.runId,
                sourceStepIndex: props.step.index,
                buttonText,
                buttonIndex,
              })}
            />
          </Show>

          {/* 工具调用 */}
          <Show when={normalToolCalls().length > 0}>
            <div class="agent-ui-message-tools">
              <For each={normalToolCalls()}>
                {(toolCall) => (
                  <ToolCallView
                    toolCall={toolCall}
                    resolveTool={props.resolveTool}
                    onAskQuestionSubmit={props.onAskQuestionSubmit}
                    activeAskQuestion={props.activeAskQuestion}
                  />
                )}
              </For>
            </div>
          </Show>
        </div>
      </div>
    </>
  );
}
