/**
 * UsageStats - Token 用量统计展示
 *
 * 以紧凑格式展示输入/输出 token 数和推理轮次。
 */

import type { UsageStats as UsageStatsData } from "../../protocol/types";

export interface UsageStatsProps {
  /** Token 用量统计数据 */
  usage: UsageStatsData;
}

export function UsageStats(props: UsageStatsProps) {
  const formatNumber = (num: number) => {
    if (num >= 1000000) {
      return `${(num / 1000000).toFixed(1)}M`;
    }
    if (num >= 1000) {
      return `${(num / 1000).toFixed(1)}K`;
    }
    return num.toString();
  };

  return (
    <div class="agent-ui-usage-stats">
      2000积分
    </div>
  );
}