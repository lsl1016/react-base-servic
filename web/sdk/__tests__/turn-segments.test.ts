import { describe, expect, it } from 'vitest';
import type { Step } from '../runtime/types';
import {
  buildTurnSegments,
  formatWorkSummary,
  isLiveWorkSegment,
  summarizeWorkParts,
} from '../ui/components/turn-segments';

function assistant(overrides: Partial<Step> = {}): Step {
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

describe('buildTurnSegments', () => {
  it('groups thought and tools into one collapsed work block before content', () => {
    const items = [
      {
        kind: 'step' as const,
        step: assistant({
          index: 0,
          thoughts: 'plan',
          thoughtComplete: true,
          toolCalls: [{
            toolUseId: 't1',
            toolName: 'Read',
            input: {},
            status: 'done',
            executedBy: 'server',
            result: 'ok',
          }],
        }),
      },
      {
        kind: 'step' as const,
        step: assistant({
          index: 1,
          content: '结论',
          contentComplete: true,
        }),
      },
    ];

    const segments = buildTurnSegments(items, { isActiveTurn: false, isRunning: false });
    expect(segments.map((s) => s.kind)).toEqual(['work', 'content']);
    expect(segments[0].kind === 'work' && segments[0].collapsed).toBe(true);
    expect(segments[0].kind === 'work' && segments[0].parts.length).toBe(2);
  });

  it('ends work at content_start before first content_delta', () => {
    const items = [
      {
        kind: 'step' as const,
        step: assistant({
          thoughts: 'plan',
          thoughtComplete: true,
          contentStarted: true,
          content: '',
          contentComplete: false,
        }),
      },
    ];
    const segments = buildTurnSegments(items, { isActiveTurn: true, isRunning: true });
    expect(segments.map((s) => s.kind)).toEqual(['work', 'content']);
    expect(segments[0].kind === 'work' && segments[0].collapsed).toBe(true);
    expect(isLiveWorkSegment(segments, 0, { isActiveTurn: true, isRunning: true })).toBe(false);
  });

  it('forces work collapsed once any content segment exists', () => {
    const items = [
      {
        kind: 'step' as const,
        step: assistant({
          index: 0,
          thoughts: 'plan',
          content: '你好',
          contentComplete: false,
        }),
      },
    ];
    const segments = buildTurnSegments(items, { isActiveTurn: true, isRunning: true });
    expect(segments.map((s) => s.kind)).toEqual(['work', 'content']);
    expect(segments[0].kind === 'work' && segments[0].collapsed).toBe(true);
    expect(isLiveWorkSegment(segments, 0, { isActiveTurn: true, isRunning: true })).toBe(false);
  });

  it('renders ask_question as root segment after content, not inside Work', () => {
    const askTool = {
      toolUseId: 'call_1',
      toolName: 'ask_question',
      input: {},
      status: 'done' as const,
      executedBy: 'internal' as const,
      afterContent: true,
    };
    const items = [
      {
        kind: 'step' as const,
        step: assistant({
          thoughts: 'plan',
          thoughtComplete: true,
          contentStarted: true,
          content: '信息有些不足',
          contentComplete: true,
          toolCalls: [askTool],
        }),
      },
    ];
    const segments = buildTurnSegments(items, { isActiveTurn: true, isRunning: true });
    expect(segments.map((s) => s.kind)).toEqual(['work', 'content', 'ask_question']);
    if (segments[0].kind === 'work') {
      expect(summarizeWorkParts(segments[0].parts)).toEqual({ thoughtCount: 1, toolRunCount: 0 });
    }
    if (segments[2].kind === 'ask_question') {
      expect(segments[2].toolUseId).toBe('call_1');
    }
  });

  it('renders plan tools as root segments in their original tool order', () => {
    const items = [{
      kind: 'step' as const,
      step: assistant({
        thoughts: 'plan',
        toolCalls: [
          {
            toolUseId: 'read_1',
            toolName: 'Read',
            input: {},
            status: 'done',
            executedBy: 'server',
          },
          {
            toolUseId: 'plan_1',
            toolName: 'todo_write',
            input: { merge: false, todos: [{ id: 'inspect', content: '检查实现' }] },
            status: 'done',
            executedBy: 'internal',
          },
          {
            toolUseId: 'read_2',
            toolName: 'Read',
            input: {},
            status: 'done',
            executedBy: 'server',
          },
        ],
      }),
    }];

    const segments = buildTurnSegments(items, { isActiveTurn: true, isRunning: true });
    expect(segments.map((segment) => segment.kind)).toEqual(['work', 'plan', 'work']);
    expect(segments[1].id).toContain('plan_1');
    if (segments[0].kind === 'work') {
      expect(summarizeWorkParts(segments[0].parts)).toEqual({ thoughtCount: 1, toolRunCount: 1 });
    }
    if (segments[2].kind === 'work') {
      expect(summarizeWorkParts(segments[2].parts)).toEqual({ thoughtCount: 0, toolRunCount: 1 });
    }
  });

  it('同一 Plan 恢复重试时只保留最新主卡片', () => {
    const planExecutionId = 'plan_retry';
    const items = [
      {
        kind: 'step' as const,
        step: assistant({
          index: 0,
          toolCalls: [{
            toolUseId: 'resume_failed',
            toolName: 'resume_template_plan',
            input: { plan_execution_id: planExecutionId },
            status: 'error',
            executedBy: 'internal',
            result: 'WAIT_EXTERNAL_TASK resume requires current wait_request_id',
            isError: true,
          }],
        }),
      },
      {
        kind: 'step' as const,
        step: assistant({
          index: 1,
          toolCalls: [{
            toolUseId: 'resume_succeeded',
            toolName: 'resume_template_plan',
            input: { plan_execution_id: planExecutionId, wait_request_id: 'wait_1' },
            status: 'done',
            executedBy: 'internal',
            result: '{"plan_execution_id":"plan_retry","status":"SUCCEEDED","steps":[]}',
          }],
        }),
      },
    ];

    const segments = buildTurnSegments(items, { isActiveTurn: false, isRunning: false });
    expect(segments.map((segment) => segment.kind)).toEqual(['work', 'plan']);
    if (segments[0].kind === 'work') {
      expect(summarizeWorkParts(segments[0].parts)).toEqual({ thoughtCount: 0, toolRunCount: 1 });
      expect(segments[0].parts[0].kind === 'tools' && segments[0].parts[0].toolUseIds).toEqual(['resume_failed']);
    }
    if (segments[1].kind === 'plan') {
      expect(segments[1].toolUseId).toBe('resume_succeeded');
    }
  });

  // 暂时下线 create_plan，恢复工具时取消 skip。
  it.skip('keeps create_plan confirmation content after the plan in a separate step', () => {
    const original = assistant({
      index: 0,
      content: '计划如下。',
      contentStarted: true,
      contentComplete: true,
      toolCalls: [{
        toolUseId: 'plan_1',
        toolName: 'create_plan',
        input: {
          title: '实施计划',
          overview: '完成改造',
          steps: [{ id: 'implement', outline: '完成代码修改', message: '实现并验证。' }],
        },
        status: 'done',
        executedBy: 'internal',
        result: '{"planId":"plan_1"}',
        afterContent: true,
      }],
    });
    const confirmation = assistant({
      index: 1,
      content: '### 计划已创建，等待用户确认',
      contentStarted: true,
      contentComplete: true,
    });

    const segments = buildTurnSegments([
      { kind: 'step', step: original },
      { kind: 'step', step: confirmation },
    ], { isActiveTurn: false, isRunning: false });

    expect(segments.map((segment) => segment.kind)).toEqual(['content', 'plan', 'content']);
    expect(segments[0].kind === 'content' && segments[0].step.content).toBe('计划如下。');
    expect(segments[2].kind === 'content' && segments[2].step.content).toBe('### 计划已创建，等待用户确认');
  });

  it('renders configured root client tools outside Work while keeping default tools inside', () => {
    const items = [{
      kind: 'step' as const,
      step: assistant({
        thoughts: 'prepare edit',
        toolCalls: [
          {
            toolUseId: 'read_1',
            toolName: 'read_sql',
            input: {},
            status: 'done',
            executedBy: 'client',
          },
          {
            toolUseId: 'replace_1',
            toolName: 'str_replace',
            input: { old_string: 'a', new_string: 'b' },
            status: 'waiting',
            executedBy: 'client',
          },
          {
            toolUseId: 'read_2',
            toolName: 'read_sql',
            input: {},
            status: 'waiting',
            executedBy: 'client',
          },
        ],
      }),
    }];

    const segments = buildTurnSegments(items, {
      isActiveTurn: true,
      isRunning: true,
      isRootTool: (toolCall) => toolCall.toolName === 'str_replace',
    });

    expect(segments.map((segment) => segment.kind)).toEqual(['work', 'root_tool', 'work']);
    expect(segments[1].id).toContain('replace_1');
    if (segments[0].kind === 'work') {
      expect(summarizeWorkParts(segments[0].parts)).toEqual({ thoughtCount: 1, toolRunCount: 1 });
    }
    if (segments[2].kind === 'work') {
      expect(summarizeWorkParts(segments[2].parts)).toEqual({ thoughtCount: 0, toolRunCount: 1 });
    }
  });

  it('renders displayFiles as a built-in root segment', () => {
    const items = [{
      kind: 'step' as const,
      step: assistant({
        thoughts: 'prepare files',
        toolCalls: [{
          toolUseId: 'files_1',
          toolName: 'displayFiles',
          input: {
            artifacts: [{ type: 'application/pdf', uri: '/artifact/report' }],
          },
          status: 'done',
          executedBy: 'internal',
          result: '{"displayed":1}',
        }],
      }),
    }];

    const segments = buildTurnSegments(items, { isActiveTurn: false, isRunning: false });
    expect(segments.map((segment) => segment.kind)).toEqual(['work', 'root_tool']);
    expect(segments[1].id).toContain('files_1');
  });

	it('renders python_exec artifact meta as a root file segment', () => {
		const items = [{
			kind: 'step' as const,
			step: assistant({
				toolCalls: [{
					toolUseId: 'python_1',
					toolName: 'python_exec',
					input: { python: 'print({})' },
					status: 'done',
					executedBy: 'internal',
					result: '{"artifacts":[{"type":"application/pdf","bytes":1024}]}',
					meta: {
						artifacts: [{ type: 'application/pdf', name: '报告.pdf', uri: '/artifact/report' }],
					},
				}],
			}),
		}];

		const segments = buildTurnSegments(items, { isActiveTurn: false, isRunning: false });
		expect(segments.map((segment) => segment.kind)).toEqual(['root_tool']);
		expect(segments[0].id).toContain('python_1');
	});

  it('filters tools without results during replay while retaining completed tools', () => {
    const items = [{
      kind: 'step' as const,
      step: assistant({
        toolCalls: [
          {
            toolUseId: 'read_done',
            toolName: 'read_sql',
            input: {},
            status: 'done',
            executedBy: 'client',
            result: '',
          },
          {
            toolUseId: 'read_pending',
            toolName: 'read_sql',
            input: {},
            status: 'cancelled',
            executedBy: 'client',
          },
          {
            toolUseId: 'replace_pending',
            toolName: 'str_replace',
            input: { old_string: 'a', new_string: 'b' },
            status: 'error',
            executedBy: 'client',
            result: '连接断开，客户端工具未返回结果。',
            isError: true,
          },
          {
            toolUseId: 'ask_pending',
            toolName: 'ask_question',
            input: {},
            status: 'cancelled',
            executedBy: 'internal',
          },
          {
            toolUseId: 'cancelled_pending',
            toolName: 'read_sql',
            input: {},
            status: 'error',
            executedBy: 'client',
            result: '用户取消了本次运行，客户端工具未返回结果。',
            isError: true,
          },
          {
            toolUseId: 'read_error',
            toolName: 'read_sql',
            input: {},
            status: 'error',
            executedBy: 'client',
            result: 'SQL 读取失败',
            isError: true,
          },
        ],
      }),
    }];

    const segments = buildTurnSegments(items, {
      isActiveTurn: false,
      isRunning: false,
      isRootTool: (toolCall) => toolCall.toolName === 'str_replace',
    });

    expect(segments.map((segment) => segment.kind)).toEqual(['work']);
    if (segments[0].kind === 'work') {
      expect(summarizeWorkParts(segments[0].parts)).toEqual({ thoughtCount: 0, toolRunCount: 2 });
      expect(segments[0].parts[0]).toMatchObject({ toolUseIds: ['read_done', 'read_error'] });
    }
  });

  it('keeps a post-content plan update inside Work', () => {
    const items = [{
      kind: 'step' as const,
      step: assistant({
        contentStarted: true,
        content: '阶段结果',
        toolCalls: [{
          toolUseId: 'plan_after_content',
          toolName: 'todo_write',
          input: { merge: true, todos: [{ id: 'implement', content: '开始修改代码', status: 'in_progress' }] },
          status: 'done',
          executedBy: 'internal',
          afterContent: true,
        }],
      }),
    }];

    const segments = buildTurnSegments(items, { isActiveTurn: true, isRunning: true });
    expect(segments.map((segment) => segment.kind)).toEqual(['content', 'work']);
    if (segments[1].kind === 'work') {
      expect(summarizeWorkParts(segments[1].parts)).toEqual({ thoughtCount: 0, toolRunCount: 1 });
      expect(segments[1].parts[0].kind).toBe('tools');
    }
  });

  it('ignores a plan update without an in-progress item', () => {
    const items = [{
      kind: 'step' as const,
      step: assistant({
        toolCalls: [{
          toolUseId: 'plan_finished',
          toolName: 'todo_write',
          input: {
            merge: true,
            todos: [
              { id: 'inspect', content: '检查实现', status: 'completed' },
              { id: 'implement', content: '修改代码', status: 'cancelled' },
            ],
          },
          status: 'done',
          executedBy: 'internal',
        }],
      }),
    }];

    const segments = buildTurnSegments(items, { isActiveTurn: false, isRunning: false });
    expect(segments).toEqual([]);
  });

  it('merges post-content work tools with later step thoughts until next content', () => {
    const readTool = {
      toolUseId: 'call_1',
      toolName: 'Read',
      input: {},
      status: 'done' as const,
      executedBy: 'server' as const,
      afterContent: true,
      result: 'ok',
    };
    const items = [
      {
        kind: 'step' as const,
        step: assistant({
          index: 0,
          thoughts: 'plan0',
          thoughtComplete: true,
          contentStarted: true,
          content: '第一段正文',
          contentComplete: true,
          toolCalls: [readTool],
        }),
      },
      {
        kind: 'step' as const,
        step: assistant({
          index: 1,
          thoughts: 'plan1',
          thoughtComplete: true,
          contentStarted: true,
          content: '第二段正文',
          contentComplete: true,
        }),
      },
    ];
    const segments = buildTurnSegments(items, { isActiveTurn: false, isRunning: false });
    expect(segments.map((s) => s.kind)).toEqual(['work', 'content', 'work', 'content']);
    if (segments[2].kind === 'work') {
      expect(summarizeWorkParts(segments[2].parts)).toEqual({ thoughtCount: 1, toolRunCount: 1 });
      expect(formatWorkSummary(1, 1)).toBe('已思考1次，运行1次');
    }
  });

  it('keeps trailing work expanded while active turn is running', () => {
    const items = [
      {
        kind: 'step' as const,
        step: assistant({ thoughts: 'thinking', thoughtComplete: true }),
      },
    ];

    const segments = buildTurnSegments(items, { isActiveTurn: true, isRunning: true });
    expect(segments).toHaveLength(1);
    expect(segments[0].kind).toBe('work');
    if (segments[0].kind === 'work') {
      expect(segments[0].collapsed).toBe(false);
    }
    expect(isLiveWorkSegment(segments, 0, { isActiveTurn: true, isRunning: true })).toBe(true);
  });
});

describe('formatWorkSummary', () => {
  it('formats thought-only, tool-only, and combined labels', () => {
    expect(formatWorkSummary(3, 0)).toBe('已思考3次');
    expect(formatWorkSummary(0, 2)).toBe('已运行2次');
    expect(formatWorkSummary(3, 2)).toBe('已思考3次，运行2次');
  });

  it('counts parts in a work segment', () => {
    const parts = [
      { kind: 'thought' as const, step: assistant({ index: 0, thoughts: 'a' }) },
      { kind: 'thought' as const, step: assistant({ index: 1, thoughts: 'b' }) },
      {
        kind: 'tools' as const,
        step: assistant({
          index: 0,
          toolCalls: [{
            toolUseId: 't1',
            toolName: 'Read',
            input: {},
            status: 'done',
            executedBy: 'server',
          }],
        }),
      },
    ];
    expect(summarizeWorkParts(parts)).toEqual({ thoughtCount: 2, toolRunCount: 1 });
    expect(formatWorkSummary(2, 1)).toBe('已思考2次，运行1次');
  });
});
