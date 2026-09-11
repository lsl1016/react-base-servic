import type { PlanConfirmationStatus, ToolCallState } from '../../runtime/types';

export interface PlanInputItem {
  id?: string;
  content: string;
  message?: string;
  status?: string;
}

export interface PlanToolInput {
  kind: 'todo' | 'plan';
  merge: boolean;
  todos: PlanInputItem[];
  planId?: string;
  title?: string;
  overview?: string;
  confirmationStatus: PlanConfirmationStatus;
}

const isRecord = (value: unknown): value is Record<string, unknown> => (
  typeof value === 'object' && value !== null && !Array.isArray(value)
);

function parsePlanResult(result?: string): Record<string, unknown> | undefined {
  if (!result) return undefined;
  try {
    const parsed = JSON.parse(result) as unknown;
    if (!isRecord(parsed)) return undefined;
    if (typeof parsed.content === 'string') {
      const content = JSON.parse(parsed.content) as unknown;
      return isRecord(content) ? content : undefined;
    }
    return parsed;
  } catch {
    return undefined;
  }
}

export function readPlanToolInput(toolCall: ToolCallState): PlanToolInput {
  const input = toolCall.input;
  if (toolCall.toolName === 'create_plan') {
    const result = parsePlanResult(toolCall.result);
    const rawSteps = Array.isArray(input.steps) ? input.steps : [];
    const todos = rawSteps.flatMap((item) => {
      if (!isRecord(item) || typeof item.outline !== 'string' || !item.outline.trim()) return [];
      return [{
        id: typeof item.id === 'string' ? item.id : undefined,
        content: item.outline,
        message: typeof item.message === 'string' ? item.message : undefined,
      }];
    });
    return {
      kind: 'plan',
      merge: false,
      todos,
      planId: typeof result?.planId === 'string' ? result.planId : undefined,
      title: typeof input.title === 'string' ? input.title : undefined,
      overview: typeof input.overview === 'string' ? input.overview : undefined,
      confirmationStatus: toolCall.planConfirmationStatus ?? 'pending',
    };
  }

  const legacyItems = Array.isArray(input.items);
  const merge = typeof input.merge === 'boolean' ? input.merge : legacyItems;
  const rawTodos = !merge && toolCall.todoItems
    ? toolCall.todoItems
    : input.todos ?? input.items;
  const todos = Array.isArray(rawTodos)
    ? rawTodos.flatMap((item) => {
      if (!isRecord(item) || typeof item.content !== 'string' || !item.content.trim()) return [];
      return [{
        id: typeof item.id === 'string' ? item.id : undefined,
        content: item.content,
        status: typeof item.status === 'string' ? item.status : undefined,
      }];
    })
    : [];

  return {
    kind: 'todo',
    merge,
    todos,
    confirmationStatus: 'accepted',
  };
}

export function getStartedPlanItems(toolCall: ToolCallState): PlanInputItem[] {
  return readPlanToolInput(toolCall).todos.filter((todo) => todo.status === 'in_progress');
}

export function shouldRenderPlanTool(toolCall: ToolCallState): boolean {
  const input = readPlanToolInput(toolCall);
  if (input.kind === 'plan') {
    return input.todos.length > 0;
  }
  return input.merge ? getStartedPlanItems(toolCall).length > 0 : input.todos.length > 0;
}
