import { describe, expect, it } from 'vitest';
import type { Step } from '../runtime/types';
import { groupStepsByAgentLane, laneIsActive, laneLabel, laneMainPath } from '../ui/components/lanes';

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

describe('groupStepsByAgentLane', () => {
  it('主泳道置首，子代理按首次出现排序，步序保持', () => {
    const steps = [
      step({ agentPath: 'main/geo-agent', content: '查天气' }),
      step({ agentPath: '', content: '开始' }),
      step({ agentPath: 'main/finance-agent', content: '查汇率' }),
      step({ agentPath: 'main/geo-agent', content: '31.9°C' }),
    ];
    const lanes = groupStepsByAgentLane(steps);
    expect(lanes.map((lane) => lane.path)).toEqual(['main', 'main/geo-agent', 'main/finance-agent']);
    expect(lanes[0].steps[0].content).toBe('开始');
    expect(lanes[1].steps.map((s) => s.content)).toEqual(['查天气', '31.9°C']);
  });

  it('空 steps 返回空数组；纯主会话只有一条泳道', () => {
    expect(groupStepsByAgentLane([])).toEqual([]);
    const lanes = groupStepsByAgentLane([step({ content: 'a' }), step({ content: 'b' })]);
    expect(lanes).toHaveLength(1);
    expect(lanes[0].path).toBe(laneMainPath);
    expect(lanes[0].label).toBe('主 Agent');
  });

  it('嵌套路径取最后一段作标题', () => {
    expect(laneLabel('main/geo-agent')).toBe('geo-agent');
    expect(laneLabel(laneMainPath)).toBe('主 Agent');
  });
});

describe('laneIsActive', () => {
  it('未完成的思考/等待中的工具视为运行中；全部完结则否', () => {
    expect(laneIsActive({ path: 'main', label: '', steps: [step({ thoughts: '推理中', thoughtComplete: false })] })).toBe(true);
    expect(laneIsActive({
      path: 'main', label: '',
      steps: [step({ toolCalls: [{ toolUseId: 't1', toolName: 'x', input: {}, status: 'waiting', executedBy: 'server' }] })],
    })).toBe(true);
    expect(laneIsActive({ path: 'main', label: '', steps: [step({ thoughts: '完', thoughtComplete: true, content: 'ok', contentComplete: true })] })).toBe(false);
    // 用户步骤不算活动
    expect(laneIsActive({ path: 'main', label: '', steps: [step({ role: 'user' })] })).toBe(false);
  });
});
