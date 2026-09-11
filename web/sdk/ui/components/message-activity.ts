import type { Step } from '../../runtime/types';
import {
  buildTurnSegments,
  isLiveWorkSegment,
  type SectionItem,
} from './turn-segments';

function getCurrentTurnSteps(steps: Step[]): Step[] {
  for (let index = steps.length - 1; index >= 0; index -= 1) {
    if (steps[index].role === 'user') {
      return steps.slice(index + 1);
    }
  }
  return steps;
}

function getLastAssistantStep(steps: Step[]): Step | undefined {
  for (let index = steps.length - 1; index >= 0; index -= 1) {
    const step = steps[index];
    if (step.role === 'assistant') {
      return step;
    }
  }
  return undefined;
}

function getLastTurnStep(steps: Step[]): Step | undefined {
  return steps[steps.length - 1];
}

function hasPendingTool(step: Step): boolean {
  return step.toolCalls.some((toolCall) => toolCall.status === 'running' || toolCall.status === 'waiting');
}

function hasCompletedTool(step: Step): boolean {
  return step.toolCalls.some((toolCall) => toolCall.status === 'done' || toolCall.status === 'error');
}

function getActiveTurnSectionItems(steps: Step[]): SectionItem[] {
  const turnSteps = getCurrentTurnSteps(steps);
  const items: SectionItem[] = [];
  for (const step of turnSteps) {
    if (step.role === 'user') continue;
    items.push({
      kind: step.role === 'compact' ? 'compact' : 'step',
      step,
    });
  }
  return items;
}

/** 当前轮末尾是否存在直播中的 WorkBlock（「工作中」） */
export function activeTurnHasLiveWork(steps: Step[], isRunning: boolean): boolean {
  if (!isRunning) {
    return false;
  }
  const items = getActiveTurnSectionItems(steps);
  if (items.length === 0) {
    return false;
  }
  const options = { isActiveTurn: true, isRunning: true };
  const segments = buildTurnSegments(items, options);
  return segments.some((_, index) => isLiveWorkSegment(segments, index, options));
}

export function shouldShowDelayedActivityIndicator(
  steps: Step[],
  isRunning: boolean,
  isCompacting: boolean,
): boolean {
  if (!isRunning || isCompacting) {
    return false;
  }

  if (activeTurnHasLiveWork(steps, isRunning)) {
    return false;
  }

  const currentTurnSteps = getCurrentTurnSteps(steps);
  const lastTurnStep = getLastTurnStep(currentTurnSteps);

  // 首包空档：本轮 run 已启动，但最后一条 user 之后还没有任何非 user 事件
  if (!lastTurnStep) {
    return true;
  }

  if (lastTurnStep.role === 'compact') {
    return true;
  }

  const lastAssistantStep = getLastAssistantStep(currentTurnSteps);
  if (!lastAssistantStep) {
    return true;
  }

  if (lastAssistantStep.contentStarted && !lastAssistantStep.contentComplete) {
    return false;
  }

  if (lastAssistantStep.content && !lastAssistantStep.contentComplete) {
    return false;
  }

  if (hasPendingTool(lastAssistantStep)) {
    return false;
  }

  if (hasCompletedTool(lastAssistantStep)) {
    return true;
  }

  if (lastAssistantStep.contentComplete) {
    return true;
  }

  return lastAssistantStep.thoughtComplete;
}

export function getActivityGapSignature(
  steps: Step[],
  isRunning: boolean,
  isCompacting: boolean,
): string {
  if (!shouldShowDelayedActivityIndicator(steps, isRunning, isCompacting)) {
    return 'idle';
  }

  const currentTurnSteps = getCurrentTurnSteps(steps);
  const lastTurnStep = getLastTurnStep(currentTurnSteps);
  if (!lastTurnStep) {
    return 'first-event-gap';
  }

  if (lastTurnStep.role === 'compact') {
    return `${lastTurnStep.runId}:${lastTurnStep.index}:compact`;
  }

  const lastAssistantStep = getLastAssistantStep(currentTurnSteps);
  if (!lastAssistantStep) {
    return 'first-event-gap';
  }

  if (hasCompletedTool(lastAssistantStep)) {
    return `${lastAssistantStep.runId}:${lastAssistantStep.index}:tool:${lastAssistantStep.toolCalls.length}`;
  }

  if (lastAssistantStep.contentComplete) {
    return `${lastAssistantStep.runId}:${lastAssistantStep.index}:content:${lastAssistantStep.content.length}`;
  }

  if (lastAssistantStep.thoughtComplete) {
    return `${lastAssistantStep.runId}:${lastAssistantStep.index}:thought:${lastAssistantStep.thoughts.length}`;
  }

  return `${lastAssistantStep.runId}:${lastAssistantStep.index}:idle`;
}
