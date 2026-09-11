import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('tree picker theme styles', () => {
  it('uses SDK CSS variables and inherits the dark theme tokens without transitions', () => {
    const inputStyles = readFileSync(resolve(process.cwd(), 'ui/styles/input.scss'), 'utf8');
    const themeStyles = readFileSync(resolve(process.cwd(), 'ui/styles/_theme.scss'), 'utf8');
    const pickerStyles = inputStyles.slice(inputStyles.indexOf('.agent-ui-tree-picker {'), inputStyles.indexOf('.agent-ui-slash-menu-header'));

    expect(pickerStyles).toContain('$agent-ui-bg-primary');
    expect(pickerStyles).toContain('$agent-ui-text-primary');
    expect(pickerStyles).toContain('$agent-ui-border-color');
    expect(pickerStyles).toContain('$agent-ui-primary');
    expect(pickerStyles).not.toContain('@include agent-ui-transition');
    expect(themeStyles).toContain("[data-agent-ui-theme='dark']");
    expect(themeStyles).toContain('--agent-ui-bg-primary: #18191c');
    expect(themeStyles).toContain('--agent-ui-text-primary: #f1f3f5');
  });
});
