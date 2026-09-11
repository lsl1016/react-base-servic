import { describe, expect, it, vi } from 'vitest';
import { clientToolUI } from '../ui/client-tool-ui';

describe('client tool UI primitives', () => {
  it('renders a unified patch with one line-number column, markers, folding, and highlighting', async () => {
    const original = Array.from({ length: 40 }, (_, index) => (
      index === 1
        ? "select regexp_replace(s1.group_name, '[^a-z]') as old_value where dt >= '{@date-7}';"
        : `select field_${index + 1};`
    )).join('\n');
    const modifiedLines = original.split('\n');
    modifiedLines[1] = "select regexp_replace(s1.group_name, '[^a-z]') as new_value where dt >= '{@date-7}';";
    modifiedLines[34] = 'select changed_tail;';
    const patch = clientToolUI.diffEditor.createPatch({
      original,
      modified: modifiedLines.join('\n'),
      oldFileName: 'current.sql',
      newFileName: 'current.sql',
      contextLines: 2,
    });
    const summary = vi.fn();
    const outerRoot = document.createElement('div');
    outerRoot.dataset.agentUiTheme = 'dark';
    const root = document.createElement('div');
    root.dataset.agentUiTheme = 'dark';
    const host = document.createElement('div');
    root.append(host);
    outerRoot.append(root);

    const view = clientToolUI.diffEditor.create(host, {
      value: patch,
      height: 320,
      onDiffChange: summary,
    });
    await view.ready;

    expect(host.dataset.agentEditorTheme).toBe('dark');
    expect(host.querySelectorAll('.agent-ui-client-diff-row-delete')).toHaveLength(2);
    expect(host.querySelectorAll('.agent-ui-client-diff-row-insert')).toHaveLength(2);
    expect(host.querySelector('.agent-ui-client-diff-row-delete .agent-ui-client-diff-marker')?.textContent).toBe('-');
    expect(host.querySelector('.agent-ui-client-diff-row-insert .agent-ui-client-diff-marker')?.textContent).toBe('+');
    expect(host.querySelector('.agent-ui-client-diff-gap')?.textContent).toMatch(/\d+ unmodified lines/);
    expect(host.querySelector('.hljs-keyword')?.textContent).toBe('select');
    expect(host.querySelector('.hljs-built_in')?.textContent).toBe('regexp_replace');
    expect(host.querySelector('.hljs-variable')?.textContent).toBe('s1');
    expect(host.querySelector('.hljs-property')?.textContent).toBe('.group_name');
    expect(host.querySelector('.hljs-operator')?.textContent).toBe('>=');
    expect(summary).toHaveBeenLastCalledWith({ addedLines: 2, removedLines: 2 });

    for (const row of host.querySelectorAll('.agent-ui-client-diff-row')) {
      expect(row.querySelectorAll('.agent-ui-client-diff-line-number')).toHaveLength(1);
    }

    root.dataset.agentUiTheme = 'light';
    await vi.waitFor(() => expect(host.dataset.agentEditorTheme).toBe('light'));
    expect(outerRoot.dataset.agentUiTheme).toBe('dark');
    view.dispose();
    expect(host.childElementCount).toBe(0);
  });

  it('renders code as a read-only highlighted pre/code block', async () => {
    const host = document.createElement('div');
    const view = clientToolUI.codeEditor.create(host, {
      value: 'const answer = 42;',
      fileName: 'answer.ts',
      lineNumbers: false,
    });
    await view.ready;

    expect(host.querySelector('.agent-ui-client-code-scroll > pre > code.hljs')).not.toBeNull();
    expect(host.querySelector('.hljs-keyword')?.textContent).toBe('const');
    expect(host.querySelector('textarea')).toBeNull();

    view.setValue('const next = 43;');
    expect(host.textContent).toContain('next');
    view.dispose();
  });
});
