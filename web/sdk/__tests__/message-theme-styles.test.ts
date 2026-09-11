import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

function contrastRatio(foreground: string, background: string) {
  const luminance = (hex: string) => {
    const channels = hex.match(/[a-f\d]{2}/gi)?.map(value => Number.parseInt(value, 16) / 255) ?? [];
    const [red, green, blue] = channels.map(value => (
      value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
    ));
    return 0.2126 * red + 0.7152 * green + 0.0722 * blue;
  };

  const values = [luminance(foreground), luminance(background)].sort((a, b) => b - a);
  return (values[0] + 0.05) / (values[1] + 0.05);
}

describe('message theme styles', () => {
  it('uses dedicated accessible user-message colors in both themes', () => {
    const themeStyles = readFileSync(resolve(process.cwd(), 'ui/styles/_theme.scss'), 'utf8');

    expect(themeStyles).toContain('--agent-ui-user-message-bg: #dfe7ff');
    expect(themeStyles).toContain('--agent-ui-user-message-text: #182b5c');
    expect(themeStyles).toContain('--agent-ui-user-message-bg: #34457c');
    expect(themeStyles).toContain('--agent-ui-user-message-text: #ffffff');
    expect(themeStyles).toContain('--agent-ui-message-meta: #525c6d');
    expect(themeStyles).toContain('--agent-ui-message-meta: #bcc2cc');
    expect(contrastRatio('#182b5c', '#dfe7ff')).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio('#ffffff', '#34457c')).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio('#525c6d', '#ffffff')).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio('#bcc2cc', '#18191c')).toBeGreaterThanOrEqual(4.5);
  });

  it('renders user messages as right-aligned bubbles without dimming work content', () => {
    const messageStyles = readFileSync(resolve(process.cwd(), 'ui/styles/message.scss'), 'utf8');
    const workStyles = readFileSync(resolve(process.cwd(), 'ui/styles/work.scss'), 'utf8');
    const thoughtStyles = readFileSync(resolve(process.cwd(), 'ui/styles/thought.scss'), 'utf8');
    const toolStyles = readFileSync(resolve(process.cwd(), 'ui/styles/tool.scss'), 'utf8');
    const userCardStyles = messageStyles.slice(
      messageStyles.indexOf('.agent-ui-message-user-card {'),
      messageStyles.indexOf('.agent-ui-message-item {'),
    );

    expect(userCardStyles).toContain('align-self: flex-end');
    expect(userCardStyles).toContain('width: fit-content');
    expect(userCardStyles).toContain('$agent-ui-user-message-bg');
    expect(userCardStyles).toContain('max-height: calc(#{$agent-ui-user-message-max-height} - 22px)');
    expect(userCardStyles).toContain('overflow-y: auto');
    expect(userCardStyles).toContain('@include agent-ui-scrollbar');
    expect(messageStyles).toContain('color: $agent-ui-message-meta');
    expect(workStyles).toContain('color: $agent-ui-message-meta');
    expect(thoughtStyles).toContain('color: $agent-ui-message-meta');
    expect(toolStyles).toContain('color: $agent-ui-message-meta');
    expect(workStyles).not.toContain('opacity: 0.6');
  });
});
