import { createComponent } from 'solid-js';
import { render } from 'solid-js/web';
import { afterEach, describe, expect, it } from 'vitest';
import { ContextUsageTooltip } from '../ui/components/ContextUsageTooltip';
import { UsageRing } from '../ui/components/UsageRing';

afterEach(() => {
  document.body.innerHTML = '';
});

describe('ContextUsageTooltip', () => {
  it('formats context and cache usage without owning interaction state', () => {
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(ContextUsageTooltip, {
        contextUsedTokens: 12300,
        maxContextTokens: 100000,
        cacheReadTokens: 1500,
        inputTokens: 3000,
      }),
      host,
    );

    expect(host.querySelector('[role="tooltip"]')).not.toBeNull();
    expect(host.textContent).toContain('12.3K/100.0K 12%');
    expect(host.textContent).toContain('1.5K/3.0K 50%');
    dispose();
  });
});

describe('UsageRing', () => {
  it('composes the context tooltip', () => {
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(UsageRing, {
        contextUsedTokens: 1000,
        maxContextTokens: 10000,
        cacheReadTokens: 0,
        inputTokens: 0,
      }),
      host,
    );

    expect(host.querySelector('.agent-ui-usage-ring-tooltip')).not.toBeNull();
    expect(host.textContent).toContain('1.0K/10.0K 10%');
    expect(host.textContent).toContain('—');
    dispose();
  });
});
