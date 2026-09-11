import IconMdiHashtag from '~icons/mdi/hashtag';

export interface TodoStartBlockProps {
  description: string;
  items: string[];
}

export function TodoStartBlock(props: TodoStartBlockProps) {
  return (
    <div
      class="agent-ui-todo-start-block"
      role="status"
      aria-label={`${props.description}：${props.items.join('、')}`}
    >
      <span class="agent-ui-todo-start-icon" aria-hidden="true">
        <IconMdiHashtag width="13" height="13" />
      </span>
      <span class="agent-ui-todo-start-description">{props.description}</span>
      <span class="agent-ui-todo-start-status">处理中</span>
    </div>
  );
}
