import type { Step } from '../../runtime/types';

export function CompactDivider(props: { step: Step }) {
  const summary = () => props.step.compact?.summary?.trim() ?? '';
  return (
    <div class="agent-ui-compact-divider" title={summary() || undefined}>
      <span class="agent-ui-compact-divider-line" />
      <span class="agent-ui-compact-divider-label">上下文已压缩</span>
      <span class="agent-ui-compact-divider-line" />
    </div>
  );
}
