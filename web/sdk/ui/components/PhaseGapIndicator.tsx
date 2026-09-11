import IconMdiChevronDown from "~icons/mdi/chevron-down";
import IconMdiChevronRight from "~icons/mdi/chevron-right";
import { Show } from "solid-js";
import spinIcon from "../styles/spin.svg?raw";

export type PhaseGapIndicatorProps = {
  text?: string;
  /** 默认 true */
  showIcon?: boolean;
  /** 由外层交互容器处理点击时，仅展示 chevron。 */
  showChevron?: boolean;
  /** 与 onToggleExpand 同时传入时在末尾展示展开/折叠 chevron */
  expanded?: boolean;
  onToggleExpand?: () => void;
};

export function PhaseGapIndicator(props: PhaseGapIndicatorProps) {
  const handleChevronClick = (event: MouseEvent) => {
    event.stopPropagation();
    props.onToggleExpand?.();
  };

  return (
    <div
      class="agent-ui-phase-gap-indicator"
      classList={{ 'agent-ui-phase-gap-indicator-static': props.showIcon === false }}
    >
      <Show when={props.showIcon !== false}>
        <span
          class="agent-ui-phase-gap-indicator-icon"
          innerHTML={spinIcon}
          aria-hidden="true"
        />
      </Show>
      <span class="agent-ui-phase-gap-indicator-text">{props.text ?? "请稍等"}</span>
      <Show when={props.showChevron || props.onToggleExpand}>
        <Show
          when={props.onToggleExpand}
          fallback={(
            <span class="agent-ui-phase-gap-indicator-chevron" aria-hidden="true">
              <Show when={props.expanded} fallback={<IconMdiChevronRight width="16" height="16" />}>
                <IconMdiChevronDown width="16" height="16" />
              </Show>
            </span>
          )}
        >
          <button
            type="button"
            class="agent-ui-phase-gap-indicator-chevron"
            aria-expanded={props.expanded ?? false}
            onClick={handleChevronClick}
          >
            <Show when={props.expanded} fallback={<IconMdiChevronRight width="16" height="16" />}>
              <IconMdiChevronDown width="16" height="16" />
            </Show>
          </button>
        </Show>
      </Show>
    </div>
  );
}
