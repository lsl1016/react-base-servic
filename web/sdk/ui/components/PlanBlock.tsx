import { Index, Show, createMemo, createSignal } from 'solid-js';
import IconMdiChevronDown from '~icons/mdi/chevron-down';
import IconMdiChevronUp from '~icons/mdi/chevron-up';
import IconMdiCodeTags from '~icons/mdi/code-tags';
import type { ToolCallState } from '../../runtime/types';
import { TodoCreateBlock, type TodoCreateItem, type TodoDisplayStatus } from './TodoCreateBlock';
import { TodoStartBlock } from './TodoStartBlock';
import { getStartedPlanItems, readPlanToolInput, shouldRenderPlanTool, type PlanInputItem } from './plan-tool';

const TODO_STATUSES = new Set<TodoDisplayStatus>(['pending', 'in_progress', 'completed', 'cancelled']);

function toTodoCreateItems(items: ReturnType<typeof readPlanToolInput>['todos']): TodoCreateItem[] {
  return items.map((todo) => ({
    content: todo.content,
    status: TODO_STATUSES.has(todo.status as TodoDisplayStatus)
      ? todo.status as TodoDisplayStatus
      : 'pending',
  }));
}

function PlanStep(props: {
  step: PlanInputItem;
  contentId: string;
}) {
  const [collapsed, setCollapsed] = createSignal(false);

  return (
    <li class="agent-ui-plan-step-item" data-collapsed={collapsed()}>
      <span class="agent-ui-plan-step-marker" aria-hidden="true">
        <IconMdiCodeTags width="13" height="13" />
      </span>
      <div class="agent-ui-plan-step-body">
        <button
          type="button"
          class="agent-ui-plan-step-toggle"
          aria-expanded={!collapsed()}
          aria-controls={props.contentId}
          onClick={() => setCollapsed(!collapsed())}
        >
          <span class="agent-ui-plan-step-outline">{props.step.content}</span>
          <span class="agent-ui-plan-step-collapse-icon" aria-hidden="true">
            <Show when={collapsed()} fallback={<IconMdiChevronUp width="16" height="16" />}>
              <IconMdiChevronDown width="16" height="16" />
            </Show>
          </span>
        </button>
        <Show when={!collapsed() && props.step.message}>
          <div id={props.contentId} class="agent-ui-plan-step-message">
            {props.step.message}
          </div>
        </Show>
      </div>
    </li>
  );
}

export function PlanBlock(props: {
  toolCall: ToolCallState;
  onConfirm?: (planId: string) => void;
}) {
  const input = createMemo(() => readPlanToolInput(props.toolCall));
  const startedItems = createMemo(() => getStartedPlanItems(props.toolCall));
  const [collapsed, setCollapsed] = createSignal(false);
  const isCreatePlan = () => input().kind === 'plan';
  const planContentId = () => `agent-ui-plan-content-${props.toolCall.toolUseId}`;
  const stepKey = (index: number) => input().todos[index]?.id || `step-${index}`;
  const stepContentId = (index: number) => `${planContentId()}-${stepKey(index)}`;
  const canConfirm = () => (
    input().confirmationStatus === 'pending'
    && !!input().planId
    && !!props.onConfirm
  );

  return (
    <Show when={shouldRenderPlanTool(props.toolCall)}>
      <Show
        when={isCreatePlan()}
        fallback={(
          <Show
            when={input().merge}
            fallback={(
              <TodoCreateBlock
                items={toTodoCreateItems(input().todos)}
                title={props.toolCall.description}
              />
            )}
          >
            <TodoStartBlock
              description={props.toolCall.description ?? ''}
              items={startedItems().map((todo) => todo.content)}
            />
          </Show>
        )}
      >
        <div class="agent-ui-plan-confirmation-card" data-collapsed={collapsed()}>
          <button
            type="button"
            class="agent-ui-plan-confirmation-header"
            aria-expanded={!collapsed()}
            aria-controls={planContentId()}
            onClick={() => setCollapsed(!collapsed())}
          >
            <span class="agent-ui-plan-confirmation-header-icon" aria-hidden="true">
              <IconMdiCodeTags width="16" height="16" />
            </span>
            <span class="agent-ui-plan-confirmation-title">
              {input().title || props.toolCall.description || '执行计划'}
            </span>
            <span class="agent-ui-plan-confirmation-count">{input().todos.length} 个步骤</span>
            <span class="agent-ui-plan-collapse-icon" aria-hidden="true">
              <Show when={collapsed()} fallback={<IconMdiChevronUp width="18" height="18" />}>
                <IconMdiChevronDown width="18" height="18" />
              </Show>
            </span>
          </button>
          <div
            id={planContentId()}
            class="agent-ui-plan-confirmation-content"
            hidden={collapsed()}
          >
            <Show when={input().overview}>
              <div class="agent-ui-plan-confirmation-overview">{input().overview}</div>
            </Show>
            <ol class="agent-ui-plan-step-list">
              <Index each={input().todos}>
                {(step, index) => (
                  <PlanStep step={step()} contentId={stepContentId(index)} />
                )}
              </Index>
            </ol>
            <Show when={input().confirmationStatus !== 'accepted'}>
              <div class="agent-ui-plan-confirmation-actions">
                <button
                  type="button"
                  class="agent-ui-plan-confirmation-button"
                  disabled={!canConfirm()}
                  onClick={() => {
                    const planId = input().planId;
                    if (planId && canConfirm()) props.onConfirm?.(planId);
                  }}
                >
                  {input().confirmationStatus === 'submitting' ? '发送中' : '开始任务'}
                </button>
              </div>
            </Show>
          </div>
        </div>
      </Show>
    </Show>
  );
}
