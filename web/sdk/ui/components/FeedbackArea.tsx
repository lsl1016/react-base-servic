/**
 * FeedbackArea - 反馈区域
 *
 * 固定在 AgentPanel 主内容区与输入区之间，承载当前需要用户交互的反馈。
 * 承载 `ask_question` 和 ClientTool 等待用户操作时的临时交互 UI。
 */

import { createMemo, Show } from "solid-js";
import type { AskQuestionAnswerContent } from "../../protocol/types";
import type { ToolCallState } from "../../runtime/types";
import type { ClientToolUIRenderer } from "../../tools/types";
import { AskQuestionCard } from "./AskQuestionCard";
import { CustomToolView } from "./CustomToolView";

export interface FeedbackAreaProps {
  /** 当前活跃的反馈工具调用 */
  toolCall?: ToolCallState;
  /** ClientTool 注册的 feedback 交互区 renderer；ask_question 不需要传入 */
  renderer?: ClientToolUIRenderer;
  /** 提交 ask_question 的用户答案 */
  onAskQuestionSubmit?: (toolUseId: string, content: AskQuestionAnswerContent) => void;
  onToolConfirmSubmit?: (toolUseId: string, approved: boolean) => void;
}

function isWaitingAskQuestion(toolCall?: ToolCallState): toolCall is ToolCallState {
  return !!toolCall && toolCall.toolName === "ask_question" && toolCall.status === "waiting";
}

export function FeedbackArea(props: FeedbackAreaProps) {
  const activeToolCall = createMemo(() => (props.toolCall?.status === "waiting" ? props.toolCall : undefined));

  return (
    <Show when={activeToolCall()}>
      {(toolCall) => (
        <div class="agent-ui-panel-feedback">
          <Show
            when={!isWaitingAskQuestion(toolCall()) && props.renderer}
            fallback={<AskQuestionCard toolCall={toolCall()} onSubmit={props.onAskQuestionSubmit} />}
          >
            {(renderer) => <CustomToolView toolCall={toolCall()} renderer={renderer()} />}
          </Show>
        </div>
      )}
    </Show>
  );
}

export default FeedbackArea;
