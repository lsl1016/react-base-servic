import { render } from 'solid-js/web';
import { afterEach, describe, expect, it } from 'vitest';
import type { Step } from '../runtime/types';
import { AgentLaneView } from '../ui/components/AgentLaneView';

function step(overrides: Partial<Step> = {}): Step {
  return {
    index: 0,
    runId: 'run_1',
    role: 'assistant',
    thoughts: '',
    content: '',
    toolCalls: [],
    thoughtComplete: false,
    contentStarted: false,
    contentComplete: false,
    ...overrides,
  };
}

let dispose: (() => void) | undefined;
afterEach(() => {
  dispose?.();
  dispose = undefined;
});

const mount = (steps: Step[], isRunning = false) => {
  const container = document.createElement('div');
  document.body.appendChild(container);
  dispose = render(() => <AgentLaneView steps={steps} isRunning={isRunning} />, container);
  return container;
};

describe('AgentLaneView 泳道 tab 分页', () => {
  const steps = [
    step({ agentPath: '', content: '开始' }),
    step({ agentPath: 'main/geo-agent', content: '查天气' }),
    step({ agentPath: 'main/finance-agent', content: '查汇率' }),
  ];

  it('多泳道时渲染 tab 栏且一次只展示一条泳道（默认主泳道）', () => {
    const root = mount(steps);
    const tabs = [...root.querySelectorAll('.agent-ui-lane-tab')] as HTMLElement[];
    expect(tabs.map((tab) => tab.getAttribute('aria-selected'))).toEqual(['true', 'false', 'false']);
    expect(root.querySelectorAll('.agent-ui-lane')).toHaveLength(1);
    expect(root.querySelector('.agent-ui-lane-body')?.textContent).toContain('开始');
  });

  it('点击 tab 切换到对应泳道并固定选中', () => {
    const root = mount(steps);
    const tabs = [...root.querySelectorAll('.agent-ui-lane-tab')] as HTMLElement[];
    tabs[1].click();
    expect(root.querySelector('.agent-ui-lane-body')?.textContent).toContain('查天气');
    expect(tabs[1].getAttribute('aria-selected')).toBe('true');
    expect(tabs[0].getAttribute('aria-selected')).toBe('false');
  });

  it('运行中未手动选择时自动跟随活动泳道', () => {
    const running = [
      step({ agentPath: '', content: '开始', contentComplete: true }),
      step({ agentPath: 'main/geo-agent', thoughts: '推理中', thoughtComplete: false }),
    ];
    const root = mount(running, true);
    expect(root.querySelector('.agent-ui-lane-body')?.textContent).toContain('推理中');
    const tabs = [...root.querySelectorAll('.agent-ui-lane-tab')] as HTMLElement[];
    expect(tabs[1].getAttribute('aria-selected')).toBe('true');
  });

  it('单泳道时不渲染 tab 栏，保留泳道头部信息', () => {
    const root = mount([step({ agentPath: '', content: '只有主泳道' })]);
    expect(root.querySelector('.agent-ui-lane-tabs')).toBeNull();
    expect(root.querySelector('.agent-ui-lane-head')).not.toBeNull();
  });
});
