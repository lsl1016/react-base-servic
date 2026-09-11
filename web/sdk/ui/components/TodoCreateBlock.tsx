import { Index, Show, createSignal } from 'solid-js';
import IconMdiChevronDown from '~icons/mdi/chevron-down';
import IconMdiChevronUp from '~icons/mdi/chevron-up';
import IconMdiHashtag from '~icons/mdi/hashtag';

export type TodoDisplayStatus = 'pending' | 'in_progress' | 'completed' | 'cancelled';

export interface TodoCreateItem {
  content: string;
  status?: TodoDisplayStatus;
}

export interface TodoCreateBlockProps {
  items: TodoCreateItem[];
  title?: string;
}

const TODO_STATUS_LABELS: Record<TodoDisplayStatus, string> = {
  pending: '待处理',
  in_progress: '处理中',
  completed: '已完成',
  cancelled: '已取消',
};

export function TodoCreateBlock(props: TodoCreateBlockProps) {
  const [collapsed, setCollapsed] = createSignal(false);
  const canCollapse = () => props.items.length > 1;

  return (
    <div class="agent-ui-todo-create-block">
      <div class="agent-ui-todo-create-header"  >
        <div class="agent-ui-todo-create-heading">
          <IconMdiHashtag width="15" height="15" class="agent-ui-todo-create-icon" />
          <span class="agent-ui-todo-create-title">{props.title || '任务清单'}</span>
        </div>
        <div class="agent-ui-todo-create-actions" onClick={() => setCollapsed(!collapsed())}>
          <span class="agent-ui-todo-create-count">{props.items.length} 项</span>
          <Show when={canCollapse()}>
            <button
              type="button"
              class="agent-ui-todo-create-toggle"
              title={collapsed() ? '展开任务清单' : '收起任务清单'}
              aria-label={collapsed() ? '展开任务清单' : '收起任务清单'}
              aria-expanded={!collapsed()}
            >
              <Show when={collapsed()} fallback={<IconMdiChevronUp width="17" height="17" />}>
                <IconMdiChevronDown width="17" height="17" />
              </Show>
            </button>
          </Show>
        </div>
      </div>

      <Show when={!collapsed()}>
        <ol class="agent-ui-todo-create-list">
          <Index each={props.items}>
            {(item) => {
              const status = () => item().status ?? 'pending';
              return (
                <li class="agent-ui-todo-create-item" data-status={status()}>
                  <span class="agent-ui-todo-create-content">{item().content}</span>
                  <span class="agent-ui-todo-create-status">{TODO_STATUS_LABELS[status()]}</span>
                </li>
              );
            }}
          </Index>
        </ol>
      </Show>
    </div>
  );
}
