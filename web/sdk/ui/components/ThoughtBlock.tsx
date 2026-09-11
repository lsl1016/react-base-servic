/**
 * ThoughtBlock - 模型思考过程展示
 *
 * 以可折叠方式展示模型的思考内容。
 * 收起时显示摘要，展开后显示完整思考过程。
 */

import { createSignal, createEffect, Show } from "solid-js";
import IconLucideBrain from "~icons/lucide/brain";
import IconMdiChevronDown from "~icons/mdi/chevron-down";
import IconMdiChevronRight from "~icons/mdi/chevron-right";
import { AutoScrollPanel } from "./AutoScrollPanel";

export interface ThoughtBlockProps {
  /** 思考内容 */
  content: string;
  /** 思考是否已完成 */
  complete: boolean;
  /** 默认是否收起 */
  defaultCollapsed?: boolean;
}

export function ThoughtBlock(props: ThoughtBlockProps) {
  const [collapsed, setCollapsed] = createSignal(props.defaultCollapsed ?? true);
  const [userToggled, setUserToggled] = createSignal(false);

  createEffect(() => {
    if (userToggled()) return;
    if (!props.complete && props.content) {
      setCollapsed(false);
    } else if (props.complete) {
      setCollapsed(true);
    }
  });

  const toggle = () => {
    setUserToggled(true);
    setCollapsed(!collapsed());
  };

  return (
    <div class="agent-ui-thought-block">
      <button
        class="agent-ui-thought-toggle"
        onClick={toggle}
      >
        <span class="agent-ui-thought-icon">
          <Show when={collapsed()} fallback={<IconMdiChevronDown width="16" height="16" />}>
            <IconMdiChevronRight width="16" height="16" />
          </Show>
        </span>
        <IconLucideBrain width="15" height="15" class="agent-ui-thought-icon-brain"/>
        <span class="agent-ui-thought-label">
          {props.complete ? "Thought" : "Thinking"}
        </span>
      </button>

      <Show when={!collapsed()}>
        <AutoScrollPanel class="agent-ui-thought-content" followKey={props.content} maxHeight="100px">
          {props.content}
        </AutoScrollPanel>
      </Show>
    </div>
  );
}