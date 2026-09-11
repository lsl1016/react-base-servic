import { createComponent, createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { render } from 'solid-js/web';
import { afterEach, describe, expect, it } from 'vitest';
import type { Step, ToolCallState } from '../runtime/types';
import { AssistantTurnBody } from '../ui/components/AssistantTurnBody';

afterEach(() => {
  document.body.innerHTML = '';
});

describe('ask_question rendering', () => {
  it('shows the mini answer panel after a waiting question completes', () => {
    const toolCall: ToolCallState = {
      toolUseId: 'call_1',
      toolName: 'ask_question',
      input: {
        questions: [{
          id: 'language',
          prompt: '请选择编程语言',
          options: [{ id: 'python', label: 'Python（推荐）' }],
        }],
      },
      status: 'waiting',
      executedBy: 'internal',
    };
    const [step, setStep] = createStore<Step>({
      index: 0,
      runId: 'run_1',
      role: 'assistant',
      thoughts: '',
      content: '',
      toolCalls: [toolCall],
      thoughtComplete: true,
      contentStarted: false,
      contentComplete: false,
    });
    const [activeAskQuestion, setActiveAskQuestion] = createSignal<ToolCallState | undefined>(step.toolCalls[0]);
    const host = document.createElement('div');
    document.body.appendChild(host);

    const dispose = render(
      () => createComponent(AssistantTurnBody, {
        items: [{ kind: 'step', step }],
        isActiveTurn: true,
        isRunning: true,
        get activeAskQuestion() {
          return activeAskQuestion();
        },
      }),
      host,
    );

    expect(host.querySelector('.agent-ui-ask-question-card')).toBeNull();

    setStep('toolCalls', 0, 'status', 'done');
    setStep('toolCalls', 0, 'result', JSON.stringify({
      answers: [{
        questionId: 'language',
        prompt: '请选择编程语言',
        selected: [{ id: 'python', label: 'Python（推荐）' }],
      }],
      skipped: false,
    }));
    setActiveAskQuestion(undefined);

    expect(host.querySelector('.agent-ui-ask-question-card-minimal')).not.toBeNull();
    expect(host.textContent).toContain('Python（推荐）');
    dispose();
  });
});
