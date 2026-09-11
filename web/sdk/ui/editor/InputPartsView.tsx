import { For } from 'solid-js';
import type { AgentInputPart } from '../../runtime/types';
import type { AgentQuickInsertItem } from './types';
import { parseAgentInputText } from './types';

export interface InputPartsViewProps {
  parts?: AgentInputPart[];
  fallback: string;
  quickInsertItems?: AgentQuickInsertItem[];
}

export function InputPartsView(props: InputPartsViewProps) {
  const displayParts = () => {
    if (props.parts?.length) {
      return props.parts;
    }
    return parseAgentInputText(props.fallback, props.quickInsertItems);
  };

  return (
    <span class="agent-ui-input-parts-view">
      <For each={displayParts()}>
        {(part) =>
          part.type === 'text' ? (
            <span class="agent-ui-input-part-text">{part.text}</span>
          ) : (
            <span class="agent-ui-shortcut-node" data-shortcut-id={part.id}>
              {part.label}
            </span>
          )
        }
      </For>
    </span>
  );
}