import { createComponent } from 'solid-js';
import { render } from 'solid-js/web';
import { afterEach, describe, expect, it } from 'vitest';
import { PanelMessage } from '../ui/components/PanelMessage';

afterEach(() => {
  document.body.innerHTML = '';
});

describe('PanelMessage', () => {
  it('renders the supplied message without owning message state', () => {
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(PanelMessage, { message: '上传失败，请重试' }),
      host,
    );

    const message = host.querySelector('[role="alert"]');
    expect(message?.classList.contains('agent-ui-panel-message')).toBe(true);
    expect(message?.textContent).toBe('上传失败，请重试');
    dispose();
  });

  it('does not render an empty message', () => {
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(PanelMessage, { message: '  ' }),
      host,
    );

    expect(host.querySelector('[role="alert"]')).toBeNull();
    dispose();
  });
});
