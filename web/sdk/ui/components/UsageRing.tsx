/**
 * UsageRing - 上下文/缓存占用环形图（无状态展示组件）
 *
 * 在输入框发送按钮左侧展示一个 30x30、描边 3px 的百分比环形图，
 * 环形进度对应上下文窗口占用率，圈内默认展示上下文占用百分比。
 * hover 时通过纯 CSS tooltip 展示上下文与缓存的详情。
 *
 * 纯展示组件：输出完全由 props 决定，不持有任何内部状态。
 */

import { ContextUsageTooltip } from './ContextUsageTooltip';

const RING_SIZE = 22;
const RING_STROKE = 2.5;
const RING_RADIUS = (RING_SIZE - RING_STROKE) / 2;
const RING_CIRCUMFERENCE = 2 * Math.PI * RING_RADIUS;

export interface UsageRingProps {
  /** 上下文已占用 token 数 */
  contextUsedTokens: number;
  /** 模型上下文窗口总大小（token） */
  maxContextTokens: number;
  /** 缓存命中 token 数 */
  cacheReadTokens: number;
  /** 本轮输入 token 数（缓存占比的分母） */
  inputTokens: number;
}

function toPercent(used: number, total: number): number {
  if (total <= 0) return 0;
  return Math.max(0, Math.min(100, Math.round((used / total) * 100)));
}

export function UsageRing(props: UsageRingProps) {
  const contextPercent = () => toPercent(props.contextUsedTokens, props.maxContextTokens);
  const dashOffset = () => RING_CIRCUMFERENCE * (1 - contextPercent() / 100);

  return (
    <div class="agent-ui-usage-ring" aria-label="上下文与缓存占用">
      <svg
        class="agent-ui-usage-ring-svg"
        width={RING_SIZE}
        height={RING_SIZE}
        viewBox={`0 0 ${RING_SIZE} ${RING_SIZE}`}
      >
        <circle
          class="agent-ui-usage-ring-track"
          cx={RING_SIZE / 2}
          cy={RING_SIZE / 2}
          r={RING_RADIUS}
          fill="none"
          stroke-width={RING_STROKE}
        />
        <circle
          class="agent-ui-usage-ring-progress"
          cx={RING_SIZE / 2}
          cy={RING_SIZE / 2}
          r={RING_RADIUS}
          fill="none"
          stroke-width={RING_STROKE}
          stroke-linecap="round"
          stroke-dasharray={`${RING_CIRCUMFERENCE}`}
          stroke-dashoffset={`${dashOffset()}`}
          transform={`rotate(-90 ${RING_SIZE / 2} ${RING_SIZE / 2})`}
        />
      </svg>
      <ContextUsageTooltip
        contextUsedTokens={props.contextUsedTokens}
        maxContextTokens={props.maxContextTokens}
        cacheReadTokens={props.cacheReadTokens}
        inputTokens={props.inputTokens}
      />
    </div>
  );
}
