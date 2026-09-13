import type { Step, ToolCallState } from '../../runtime/types';

/**
 * 多代理泳道视图的分组模型（P3 泳道视图）：
 * 把会话 steps 按 agentPath 归组为并列泳道——主 agent 一条（main），
 * 每个委派子代理一条（如 main/geo-agent），泳道内保持原步序。
 */

/** laneMainPath 是主 agent 泳道的路径标识（外层 run 的 agentPath 为空）。 */
export const laneMainPath = 'main';

export interface AgentLane {
  /** 泳道路径：'main' 或形如 'main/geo-agent' 的子代理路径。 */
  path: string;
  /** 泳道标题：主 agent 显示「主 Agent」，子代理取路径最后一段。 */
  label: string;
  steps: Step[];
}

/** groupStepsByAgentLane 按 agentPath 把 steps 归组为泳道；主泳道置首，其余按首次出现排序。 */
export function groupStepsByAgentLane(steps: Step[]): AgentLane[] {
  const order: string[] = [];
  const grouped = new Map<string, Step[]>();
  for (const step of steps) {
    const path = step.agentPath?.trim() || laneMainPath;
    let lane = grouped.get(path);
    if (!lane) {
      lane = [];
      grouped.set(path, lane);
      order.push(path);
    }
    lane.push(step);
  }
  // sort 稳定：main 恒定置首，其余泳道保持首次出现顺序。
  order.sort((left, right) => {
    if (left === laneMainPath) return -1;
    if (right === laneMainPath) return 1;
    return 0;
  });
  return order.map((path) => ({ path, label: laneLabel(path), steps: grouped.get(path) ?? [] }));
}

export function laneLabel(path: string): string {
  if (path === laneMainPath) return '主 Agent';
  const segments = path.split('/').filter(Boolean);
  return segments[segments.length - 1] || path;
}

/**
 * resolveLaneTabPath 计算当前 tab 页应展示的泳道路径：
 * 用户点击过的泳道优先（固定不跳）；未手动选择时运行中自动跟随第一条活动泳道，
 * 全部空闲或不在运行则停在主泳道；所选泳道消失（如切换会话）时同样回退。
 */
export function resolveLaneTabPath(lanes: AgentLane[], pinnedPath: string | null, isRunning: boolean): string {
  if (pinnedPath && lanes.some((lane) => lane.path === pinnedPath)) return pinnedPath;
  if (isRunning) {
    const active = lanes.find((lane) => laneIsActive(lane));
    if (active) return active.path;
  }
  return lanes[0]?.path ?? laneMainPath;
}

/** laneIsActive 判定泳道是否有正在进行的活动（未完成的思考/正文/等待中的工具）。 */
export function laneIsActive(lane: AgentLane): boolean {
  for (let i = lane.steps.length - 1; i >= 0; i--) {
    const step = lane.steps[i];
    if (step.role !== 'assistant') continue;
    if (step.thoughts && !step.thoughtComplete) return true;
    if (step.content && !step.contentComplete) return true;
    if (step.toolCalls.some(isPendingToolCall)) return true;
    return false;
  }
  return false;
}

function isPendingToolCall(toolCall: ToolCallState): boolean {
  return toolCall.status === 'running' || toolCall.status === 'waiting';
}
