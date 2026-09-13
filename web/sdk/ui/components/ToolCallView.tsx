import { Show } from 'solid-js';
import type { AskQuestionAnswerContent } from '../../protocol/types';
import type { ToolCallState } from '../../runtime/types';
import type { ClientTool, ClientToolExploreUIOptions } from '../../tools/types';
import { AskQuestionCard } from './AskQuestionCard';
import { CustomToolView } from './CustomToolView';
import { DisplayFiles, hasDisplayFileArtifacts } from './DisplayFiles';
import { PlanBlock } from './PlanBlock';
import { ToolCallCard } from './ToolCallCard';
import { ToolExplore } from './ToolExplore';
import { isPlanExecutionToolCall, isPlanToolCall } from './turn-segments';

export interface ToolCallViewProps {
  toolCall: ToolCallState;
  resolveTool?: (toolName: string, frontendHint?: string) => ClientTool | undefined;
  onAskQuestionSubmit?: (toolUseId: string, content: AskQuestionAnswerContent) => void;
  onToolConfirmSubmit?: (toolUseId: string, approved: boolean) => void;

  activeAskQuestion?: ToolCallState;
}

function getBuiltinExploreOptions(toolName: string): ClientToolExploreUIOptions | undefined {
  switch (toolName) {
    case 'list_tools':
      return { prefixText: '查看能力' };
    case 'get_tool':
      return { prefixText: '使用能力' };
    case 'get_skill':
      return { prefixText: '使用技能' };
    case 'read_tool_result':
      return { prefixText: '阅读结果' };
    default:
      return undefined;
  }
}

export function ToolCallView(props: ToolCallViewProps) {
  if (isPlanToolCall(props.toolCall) && !isPlanExecutionToolCall(props.toolCall)) {
    return <PlanBlock toolCall={props.toolCall} />;
  }

  if (props.toolCall.toolName === 'ask_question') {
    return (
      <Show when={!(
        props.toolCall.status === 'waiting'
        && props.toolCall.toolUseId === props.activeAskQuestion?.toolUseId
      )}>
        <AskQuestionCard
          toolCall={props.toolCall}
          onSubmit={props.onAskQuestionSubmit}
          minimal={props.toolCall.status !== 'waiting'}
        />
      </Show>
    );
  }

	if (props.toolCall.toolName === 'displayFiles' || hasDisplayFileArtifacts(props.toolCall)) {
    return <DisplayFiles toolCall={props.toolCall} />;
  }

  const toolUi = props.resolveTool?.(props.toolCall.toolName, props.toolCall.frontendHint)?.ui;
  if (toolUi?.type === 'custom') {
    return <CustomToolView toolCall={props.toolCall} renderer={toolUi.renderer} />;
  }
  if (toolUi?.type === 'explore') {
    return <ToolExplore toolCall={props.toolCall} options={toolUi.options} />;
  }

  const builtinExploreOptions = getBuiltinExploreOptions(props.toolCall.toolName);
  if (builtinExploreOptions) {
    return <ToolExplore toolCall={props.toolCall} options={builtinExploreOptions} />;
  }

  return <ToolCallCard toolCall={props.toolCall} onToolConfirmSubmit={props.onToolConfirmSubmit} />;
}

export default ToolCallView;
