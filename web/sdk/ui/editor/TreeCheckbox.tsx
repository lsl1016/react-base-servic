import { Show } from 'solid-js';
import IconMdiCheck from '~icons/mdi/check';
import IconMdiMinus from '~icons/mdi/minus';

export interface TreeCheckboxProps {
  checked: boolean;
  indeterminate?: boolean;
  disabled?: boolean;
  label: string;
  onToggle: () => void;
}

export function TreeCheckbox(props: TreeCheckboxProps) {
  return (
    <button
      type="button"
      class="agent-ui-tree-checkbox"
      classList={{
        'agent-ui-tree-checkbox-checked': props.checked || props.indeterminate,
        'agent-ui-tree-checkbox-indeterminate': Boolean(props.indeterminate),
      }}
      role="checkbox"
      aria-checked={props.indeterminate ? 'mixed' : props.checked}
      aria-label={props.label}
      disabled={props.disabled}
      onClick={(event) => {
        event.stopPropagation();
        props.onToggle();
      }}
    >
      <Show when={props.indeterminate} fallback={
        <Show when={props.checked}>
          <IconMdiCheck width="11" height="11" />
        </Show>
      }>
        <IconMdiMinus width="12" height="12" />
      </Show>
    </button>
  );
}
