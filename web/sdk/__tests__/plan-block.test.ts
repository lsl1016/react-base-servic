import { createComponent } from 'solid-js';
import { render } from 'solid-js/web';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ToolCallState } from '../runtime/types';
import { AssistantTurnBody } from '../ui/components/AssistantTurnBody';
import { PlanBlock } from '../ui/components/PlanBlock';

function planTool(input: Record<string, unknown>, description?: string): ToolCallState {
  return {
    toolUseId: 'plan_1',
    toolName: 'todo_write',
    input,
    description,
    status: 'done',
    executedBy: 'internal',
    result: '{}',
  };
}

function createPlanTool(status: ToolCallState['planConfirmationStatus'] = 'pending'): ToolCallState {
  return {
    toolUseId: 'create_plan_1',
    toolName: 'create_plan',
    input: {
      title: '实施计划',
      overview: '先检查实现，再完成修改。',
      steps: [
        { id: 'inspect', outline: '检查现有实现', message: '读取相关模块并确认数据流。' },
        { id: 'implement', outline: '完成代码修改', message: '按确认范围实现并验证。' },
      ],
    },
    description: '制定实施计划',
    status: 'done',
    executedBy: 'internal',
    result: JSON.stringify({
      content: JSON.stringify({
        planId: 'plan_abc',
        title: '实施计划',
        overview: '先检查实现，再完成修改。',
        steps: [],
      }),
    }),
    planConfirmationStatus: status,
  };
}

afterEach(() => {
  document.body.innerHTML = '';
});

describe('PlanBlock', () => {
  it('uses TodoCreateBlock for merge=false and exposes every item status', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(
      () => createComponent(PlanBlock, {
        toolCall: planTool({
          merge: false,
          todos: [
            { id: 'a', content: '检查实现', status: 'in_progress' },
            { id: 'b', content: '修改代码', status: 'completed' },
          ],
        }, '制定执行计划'),
      }),
      host,
    );

    expect(host.querySelector('.agent-ui-todo-create-block')).not.toBeNull();
    expect(host.querySelector('.agent-ui-todo-create-header')?.textContent).toContain('制定执行计划');
    const toggle = host.querySelector<HTMLButtonElement>('.agent-ui-todo-create-toggle');
    expect(toggle).not.toBeNull();
    expect(host.querySelectorAll('.agent-ui-todo-create-item')).toHaveLength(2);
    const items = [...host.querySelectorAll('.agent-ui-todo-create-item')];
    expect(items.map((item) => item.getAttribute('data-status')))
      .toEqual(['in_progress', 'completed']);
    expect(items[0].textContent).toContain('处理中');
    expect(items[1].textContent).toContain('已完成');

    toggle?.click();
    expect(toggle?.getAttribute('aria-expanded')).toBe('false');
    expect(host.querySelector('.agent-ui-todo-create-list')).toBeNull();
    toggle?.click();
    expect(host.querySelectorAll('.agent-ui-todo-create-item')).toHaveLength(2);
    dispose();
  });

  it('prefers the latest reducer Todo snapshot for merge=false cards', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const toolCall = planTool({
      merge: false,
      todos: [
        { id: 'a', content: '旧任务内容', status: 'pending' },
        { id: 'b', content: '修改代码', status: 'pending' },
      ],
    }, '任务进度');
    toolCall.todoItems = [
      { id: 'a', content: '检查实现', status: 'completed', createdAt: 1000, updatedAt: 2000 },
      { id: 'b', content: '修改代码', status: 'in_progress', createdAt: 1000, updatedAt: 2000 },
    ];

    const dispose = render(
      () => createComponent(PlanBlock, { toolCall }),
      host,
    );

    const items = [...host.querySelectorAll('.agent-ui-todo-create-item')];
    expect(items.map((item) => item.getAttribute('data-status')))
      .toEqual(['completed', 'in_progress']);
    expect(host.textContent).toContain('检查实现');
    expect(host.textContent).not.toContain('旧任务内容');
    dispose();
  });

  it('uses TodoStartBlock for merge=true and only shows started items', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(
      () => createComponent(PlanBlock, {
        toolCall: planTool({
          merge: true,
          todos: [
            { id: 'a', content: '检查实现', status: 'completed' },
            { id: 'b', content: '修改代码', status: 'in_progress' },
            { id: 'c', content: '运行验证', status: 'in_progress' },
          ],
        }, '更新进度'),
      }),
      host,
    );

    expect(host.querySelector('.agent-ui-todo-start-block')).not.toBeNull();
    expect(host.querySelector('.agent-ui-todo-create-header')).toBeNull();
    expect(host.querySelector('.agent-ui-todo-create-toggle')).toBeNull();
    expect(host.querySelector('button')).toBeNull();
    expect(host.querySelector('.agent-ui-todo-start-block')?.getAttribute('aria-label'))
      .toBe('更新进度：修改代码、运行验证');
    expect(host.querySelector('.agent-ui-todo-start-description')?.textContent).toBe('更新进度');
    expect(host.querySelector('.agent-ui-todo-start-icon svg')?.getAttribute('width')).toBe('13');
    expect(host.querySelector('.agent-ui-todo-start-icon svg')?.getAttribute('height')).toBe('13');
    expect(host.querySelector('.agent-ui-todo-start-tooltip')).toBeNull();
    expect(host.textContent).not.toContain('推进');
    expect(host.querySelector('.agent-ui-todo-start-track')).toBeNull();
    expect(host.querySelector('.agent-ui-todo-start-status')?.textContent).toBe('处理中');
    dispose();
  });

  it('keeps a single started item static', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(
      () => createComponent(PlanBlock, {
        toolCall: planTool({
          merge: true,
          todos: [{ id: 'a', content: '修改代码', status: 'in_progress' }],
        }, '开始修改'),
      }),
      host,
    );

    expect(host.querySelector('.agent-ui-todo-start-description')?.textContent).toBe('开始修改');
    expect(host.querySelector('.agent-ui-todo-start-tooltip')).toBeNull();
    expect(host.querySelector('.agent-ui-todo-start-track')).toBeNull();
    dispose();
  });

  it('renders nothing for merge=true without an in-progress item', () => {
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(PlanBlock, {
        toolCall: planTool({
          merge: true,
          todos: [
            { id: 'a', content: '检查实现', status: 'completed' },
            { id: 'b', content: '修改代码', status: 'cancelled' },
          ],
        }),
      }),
      host,
    );

    expect(host.children).toHaveLength(0);
    dispose();
  });

  // 暂时下线 create_plan，恢复工具时取消 skip。
  it.skip('renders create_plan details and submits the stable planId', () => {
    const host = document.createElement('div');
    const onConfirm = vi.fn();
    document.body.appendChild(host);
    const dispose = render(
      () => createComponent(PlanBlock, {
        toolCall: createPlanTool('pending'),
        onConfirm,
      }),
      host,
    );

    expect(host.textContent).toContain('实施计划');
    expect(host.textContent).toContain('先检查实现，再完成修改。');
    expect(host.textContent).toContain('检查现有实现');
    expect(host.textContent).toContain('读取相关模块并确认数据流。');
    expect(host.textContent).toContain('完成代码修改');
    expect(host.textContent).toContain('按确认范围实现并验证。');
    expect(host.querySelectorAll('.agent-ui-plan-step-item')).toHaveLength(2);
    expect(host.querySelector('.agent-ui-plan-confirmation-card .agent-ui-todo-create-block')).toBeNull();
    const button = host.querySelector<HTMLButtonElement>('.agent-ui-plan-confirmation-button');
    expect(button?.textContent).toBe('开始任务');
    expect(button?.disabled).toBe(false);
    button?.click();
    expect(onConfirm).toHaveBeenCalledWith('plan_abc');
    dispose();
  });

  // 暂时下线 create_plan，恢复工具时取消 skip。
  it.skip('supports independent plan and step collapsing', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(
      () => createComponent(PlanBlock, {
        toolCall: createPlanTool('pending'),
        onConfirm: vi.fn(),
      }),
      host,
    );

    const header = host.querySelector<HTMLButtonElement>('.agent-ui-plan-confirmation-header');
    const stepToggles = host.querySelectorAll<HTMLButtonElement>('.agent-ui-plan-step-toggle');
    expect(header?.getAttribute('aria-expanded')).toBe('true');
    expect(stepToggles).toHaveLength(2);
    expect(stepToggles[0].getAttribute('aria-expanded')).toBe('true');

    stepToggles[0].click();
    expect(host.querySelectorAll<HTMLButtonElement>('.agent-ui-plan-step-toggle')[0]
      .getAttribute('aria-expanded')).toBe('false');
    expect(host.textContent).not.toContain('读取相关模块并确认数据流。');
    expect(host.textContent).toContain('按确认范围实现并验证。');
    expect(host.querySelector('.agent-ui-plan-confirmation-button')).not.toBeNull();

    header?.click();
    expect(header?.getAttribute('aria-expanded')).toBe('false');
    expect(host.querySelector<HTMLElement>('.agent-ui-plan-confirmation-content')?.hidden).toBe(true);
    expect(host.textContent).toContain('实施计划');
    expect(host.textContent).toContain('2 个步骤');

    header?.click();
    expect(header?.getAttribute('aria-expanded')).toBe('true');
    expect(host.querySelector<HTMLElement>('.agent-ui-plan-confirmation-content')?.hidden).toBe(false);
    const restoredStepToggles = host.querySelectorAll<HTMLButtonElement>('.agent-ui-plan-step-toggle');
    expect(restoredStepToggles[0].getAttribute('aria-expanded')).toBe('false');
    expect(restoredStepToggles[1].getAttribute('aria-expanded')).toBe('true');
    expect(host.textContent).not.toContain('读取相关模块并确认数据流。');
    expect(host.textContent).toContain('按确认范围实现并验证。');
    dispose();
  });

  // 暂时下线 create_plan，恢复工具时取消 skip。
  it.skip('disables create_plan while submitting and hides the button after acceptance', () => {
    const submittingHost = document.createElement('div');
    const submittingDispose = render(
      () => createComponent(PlanBlock, { toolCall: createPlanTool('submitting'), onConfirm: vi.fn() }),
      submittingHost,
    );
    const submittingButton = submittingHost.querySelector<HTMLButtonElement>('.agent-ui-plan-confirmation-button');
    expect(submittingButton?.textContent).toBe('发送中');
    expect(submittingButton?.disabled).toBe(true);
    submittingDispose();

    const acceptedHost = document.createElement('div');
    const acceptedDispose = render(
      () => createComponent(PlanBlock, { toolCall: createPlanTool('accepted'), onConfirm: vi.fn() }),
      acceptedHost,
    );
    expect(acceptedHost.querySelector('.agent-ui-plan-confirmation-button')).toBeNull();
    acceptedDispose();
  });

  it('renders TodoCreateBlock in the root flow rather than inside WorkBlock', () => {
    const host = document.createElement('div');
    const toolCall = planTool({
      merge: false,
      todos: [{ id: 'a', content: '检查实现', status: 'in_progress' }],
    });
    const dispose = render(
      () => createComponent(AssistantTurnBody, {
        items: [{
          kind: 'step',
          step: {
            index: 0,
            runId: 'run_1',
            role: 'assistant',
            thoughts: '先制定计划',
            content: '',
            toolCalls: [toolCall],
            thoughtComplete: true,
            contentStarted: false,
            contentComplete: false,
          },
        }],
        isActiveTurn: false,
        isRunning: false,
      }),
      host,
    );

    expect(host.querySelector('.agent-ui-message-plan .agent-ui-todo-create-block')).not.toBeNull();
    expect(host.querySelector('.agent-ui-work-block .agent-ui-todo-create-block')).toBeNull();
    dispose();
  });

  it('renders TodoStartBlock inside WorkBlock rather than the root plan flow', () => {
    const host = document.createElement('div');
    const toolCall = planTool({
      merge: true,
      todos: [{ id: 'a', content: '修改代码', status: 'in_progress' }],
    }, '开始修改');
    const dispose = render(
      () => createComponent(AssistantTurnBody, {
        items: [{
          kind: 'step',
          step: {
            index: 0,
            runId: 'run_1',
            role: 'assistant',
            thoughts: '',
            content: '',
            toolCalls: [toolCall],
            thoughtComplete: true,
            contentStarted: false,
            contentComplete: false,
          },
        }],
        isActiveTurn: true,
        isRunning: true,
      }),
      host,
    );

    expect(host.querySelector('.agent-ui-work-block .agent-ui-todo-start-block')).not.toBeNull();
    expect(host.querySelector('.agent-ui-message-plan')).toBeNull();
    dispose();
  });
});
