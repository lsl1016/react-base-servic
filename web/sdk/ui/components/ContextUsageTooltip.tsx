export interface ContextUsageTooltipProps {
  contextUsedTokens: number;
  maxContextTokens: number;
  cacheReadTokens: number;
  inputTokens: number;
}

function formatTokens(num: number): string {
  if (num >= 1000000) return `${(num / 1000000).toFixed(1)}M`;
  if (num >= 1000) return `${(num / 1000).toFixed(1)}K`;
  return num.toString();
}

function toPercent(used: number, total: number): number {
  if (total <= 0) return 0;
  return Math.max(0, Math.min(100, Math.round((used / total) * 100)));
}

export function ContextUsageTooltip(props: ContextUsageTooltipProps) {
  const contextPercent = () => toPercent(props.contextUsedTokens, props.maxContextTokens);
  const hasCache = () => props.inputTokens > 0;
  const cachePercent = () => toPercent(props.cacheReadTokens, props.inputTokens);

  return (
    <div class="agent-ui-usage-ring-tooltip" role="tooltip">
      <div class="agent-ui-usage-ring-tooltip-row">
        <span class="agent-ui-usage-ring-tooltip-label">上下文</span>
        <span class="agent-ui-usage-ring-tooltip-value">
          {formatTokens(props.contextUsedTokens)}/{formatTokens(props.maxContextTokens)} {contextPercent()}%
        </span>
      </div>
      <div class="agent-ui-usage-ring-tooltip-row">
        <span class="agent-ui-usage-ring-tooltip-label">缓存占比</span>
        <span class="agent-ui-usage-ring-tooltip-value">
          {hasCache()
            ? `${formatTokens(props.cacheReadTokens)}/${formatTokens(props.inputTokens)} ${cachePercent()}%`
            : '—'}
        </span>
      </div>
    </div>
  );
}
