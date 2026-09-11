import { createTwoFilesPatch, parsePatch } from 'diff';
import hljs from 'highlight.js/lib/core';
import bash from 'highlight.js/lib/languages/bash';
import css from 'highlight.js/lib/languages/css';
import diffLanguage from 'highlight.js/lib/languages/diff';
import go from 'highlight.js/lib/languages/go';
import java from 'highlight.js/lib/languages/java';
import javascript from 'highlight.js/lib/languages/javascript';
import json from 'highlight.js/lib/languages/json';
import markdown from 'highlight.js/lib/languages/markdown';
import python from 'highlight.js/lib/languages/python';
import sql from 'highlight.js/lib/languages/sql';
import typescript from 'highlight.js/lib/languages/typescript';
import xml from 'highlight.js/lib/languages/xml';
import yaml from 'highlight.js/lib/languages/yaml';
import type {
  ClientToolCodeEditorOptions,
  ClientToolCodeEditorView,
  ClientToolDiffEditorOptions,
  ClientToolDiffEditorView,
  ClientToolDiffPatchOptions,
  ClientToolEditorTheme,
  ClientToolUIPrimitives,
} from '../tools/types';

const enhancedSql: typeof sql = (hljsApi) => {
  const base = sql(hljsApi);
  return {
    ...base,
    contains: [
      {
        scope: 'template-variable',
        match: /\$\{[a-z_][\w:.-]*\}|\{@[a-z_][\w.-]*\}/i,
        relevance: 0,
      },
      {
        scope: 'built_in',
        match: /\b[a-z_][\w$]*(?=\s*\()/i,
        relevance: 0,
      },
      {
        scope: 'variable',
        match: /\b[a-z_][\w$]*(?=\.)/i,
        relevance: 0,
      },
      {
        scope: 'property',
        match: /\.[a-z_][\w$]*/i,
        relevance: 0,
      },
      {
        scope: 'type',
        match: /\b(?:string|tinyint|smallint|bigint|decimal|double|binary|timestamp|array|map|struct)\b/i,
        relevance: 0,
      },
      ...(base.contains ?? []),
    ],
  };
};

const highlightLanguages = {
  bash,
  css,
  diff: diffLanguage,
  go,
  java,
  javascript,
  json,
  markdown,
  python,
  sql: enhancedSql,
  typescript,
  xml,
  yaml,
};

for (const [name, language] of Object.entries(highlightLanguages)) {
  hljs.registerLanguage(name, language);
}

const languageAliases: Record<string, string> = {
  html: 'xml',
  js: 'javascript',
  jsx: 'javascript',
  md: 'markdown',
  py: 'python',
  sh: 'bash',
  shell: 'bash',
  ts: 'typescript',
  tsx: 'typescript',
  yml: 'yaml',
};

const extensionLanguages: Record<string, string> = {
  bash: 'bash',
  cjs: 'javascript',
  css: 'css',
  go: 'go',
  htm: 'xml',
  html: 'xml',
  java: 'java',
  js: 'javascript',
  json: 'json',
  jsonc: 'json',
  jsx: 'javascript',
  md: 'markdown',
  mdx: 'markdown',
  mjs: 'javascript',
  mts: 'typescript',
  py: 'python',
  scss: 'css',
  sh: 'bash',
  sql: 'sql',
  svelte: 'xml',
  ts: 'typescript',
  tsx: 'typescript',
  vue: 'xml',
  xml: 'xml',
  yaml: 'yaml',
  yml: 'yaml',
  zsh: 'bash',
};

type DiffRow =
  | { kind: 'context' | 'delete' | 'insert'; lineNumber: number; marker: '' | '-' | '+'; code: string }
  | { kind: 'gap'; count: number };

function applyContainerSize(container: HTMLElement, height: number | string | undefined, fallback: number): void {
  container.style.width = '100%';
  container.style.height = typeof height === 'number' ? `${height}px` : height || `${fallback}px`;
  container.style.minWidth = '0';
}

function resolveTheme(container: HTMLElement, theme?: ClientToolEditorTheme): ClientToolEditorTheme {
  if (theme) return theme;
  const themeRoot = container.closest<HTMLElement>('[data-agent-ui-theme]');
  return themeRoot?.dataset.agentUiTheme === 'dark' ? 'dark' : 'light';
}

function observeTheme(container: HTMLElement, theme?: ClientToolEditorTheme): MutationObserver | undefined {
  const apply = () => {
    container.dataset.agentEditorTheme = resolveTheme(container, theme);
  };
  apply();
  if (theme) return undefined;

  const themeRoot = container.closest('[data-agent-ui-theme]');
  if (!themeRoot) return undefined;
  const observer = new MutationObserver(apply);
  observer.observe(themeRoot, { attributes: true, attributeFilter: ['data-agent-ui-theme'] });
  return observer;
}

function normalizeLanguage(language?: string): string {
  const candidate = language?.trim().toLowerCase() ?? '';
  return languageAliases[candidate] ?? candidate;
}

function languageFromFileName(fileName?: string): string {
  const normalizedFileName = fileName?.trim().split('\t', 1)[0] ?? '';
  const baseName = normalizedFileName.split(/[\\/]/).pop()?.toLowerCase() ?? '';
  const extension = baseName.includes('.') ? baseName.slice(baseName.lastIndexOf('.') + 1) : '';
  return extensionLanguages[extension] ?? '';
}

function resolveLanguage(language?: string, fileName?: string): string | undefined {
  const explicit = normalizeLanguage(language);
  if (explicit && hljs.getLanguage(explicit)) return explicit;
  const inferred = languageFromFileName(fileName);
  return inferred && hljs.getLanguage(inferred) ? inferred : undefined;
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function highlightCode(value: string, language?: string): string {
  const normalized = normalizeLanguage(language);
  if (!normalized || !hljs.getLanguage(normalized)) return escapeHtml(value);
  return hljs.highlight(value, { language: normalized, ignoreIllegals: true }).value;
}

function createCodeRows(value: string, language: string | undefined, lineNumbers: boolean): DocumentFragment {
  const fragment = document.createDocumentFragment();
  const lines = value.split(/\r\n|\r|\n/);
  for (let index = 0; index < lines.length; index += 1) {
    const row = document.createElement('span');
    row.className = 'agent-ui-client-code-row';
    row.classList.toggle('agent-ui-client-code-row-without-number', !lineNumbers);
    if (lineNumbers) {
      const number = document.createElement('span');
      number.className = 'agent-ui-client-code-line-number';
      number.textContent = String(index + 1);
      row.append(number);
    }
    const code = document.createElement('span');
    code.className = 'agent-ui-client-code-content hljs';
    code.innerHTML = highlightCode(lines[index], language) || ' ';
    row.append(code);
    fragment.append(row);
  }
  return fragment;
}

function createCodeEditor(container: HTMLElement, options: ClientToolCodeEditorOptions = {}): ClientToolCodeEditorView {
  let value = options.value ?? '';
  let disposed = false;
  let viewport: HTMLElement | undefined;
  const themeObserver = observeTheme(container, options.theme);
  const language = resolveLanguage(options.language, options.fileName);

  container.classList.add('agent-ui-client-tool-editor', 'agent-ui-client-tool-code-editor');
  applyContainerSize(container, options.height, 240);

  const render = () => {
    if (disposed) return;
    const nextViewport = document.createElement('div');
    nextViewport.className = 'agent-ui-client-code-scroll';
    nextViewport.tabIndex = 0;
    const pre = document.createElement('pre');
    pre.className = 'agent-ui-client-code-pre';
    const code = document.createElement('code');
    code.className = 'agent-ui-client-code-lines hljs';
    code.append(createCodeRows(value, language, options.lineNumbers !== false));
    pre.append(code);
    nextViewport.append(pre);
    container.replaceChildren(nextViewport);
    viewport = nextViewport;
  };

  render();
  return {
    ready: Promise.resolve(),
    getValue: () => value,
    setValue(nextValue) {
      value = nextValue;
      render();
    },
    focus: () => viewport?.focus(),
    layout: () => undefined,
    dispose() {
      disposed = true;
      themeObserver?.disconnect();
      container.replaceChildren();
      viewport = undefined;
    },
  };
}

function parseDiff(value: string): { rows: DiffRow[]; fileName?: string } {
  const rows: DiffRow[] = [];
  const patches = parsePatch(value);

  for (const patch of patches) {
    let previousNewEnd: number | undefined;
    for (const hunk of patch.hunks) {
      if (previousNewEnd !== undefined) {
        const hiddenCount = Math.max(0, hunk.newStart - previousNewEnd);
        if (hiddenCount > 0) rows.push({ kind: 'gap', count: hiddenCount });
      }

      let oldLine = hunk.oldStart;
      let newLine = hunk.newStart;
      for (const line of hunk.lines) {
        const marker = line[0];
        const code = line.slice(1);
        if (marker === '\\') continue;
        if (marker === '-') {
          rows.push({ kind: 'delete', lineNumber: oldLine, marker: '-', code });
          oldLine += 1;
          continue;
        }
        if (marker === '+') {
          rows.push({ kind: 'insert', lineNumber: newLine, marker: '+', code });
          newLine += 1;
          continue;
        }
        rows.push({ kind: 'context', lineNumber: newLine, marker: '', code });
        oldLine += 1;
        newLine += 1;
      }
      previousNewEnd = newLine;
    }
  }

  const firstPatch = patches[0];
  return {
    rows,
    fileName: firstPatch?.newFileName || firstPatch?.oldFileName,
  };
}

function createDiffEditor(container: HTMLElement, options: ClientToolDiffEditorOptions): ClientToolDiffEditorView {
  let value = options.value;
  let disposed = false;
  let viewport: HTMLElement | undefined;
  const themeObserver = observeTheme(container, options.theme);

  container.classList.add('agent-ui-client-tool-editor', 'agent-ui-client-tool-diff-editor');
  applyContainerSize(container, options.height, 320);

  const render = () => {
    if (disposed) return;
    const parsedDiff = parseDiff(value);
    const language = resolveLanguage(options.language, options.fileName ?? parsedDiff.fileName);
    let addedLines = 0;
    let removedLines = 0;
    const nextViewport = document.createElement('div');
    nextViewport.className = 'agent-ui-client-diff-scroll';
    nextViewport.tabIndex = 0;
    const pre = document.createElement('pre');
    pre.className = 'agent-ui-client-diff-pre';
    const lines = document.createElement('code');
    lines.className = 'agent-ui-client-diff-lines hljs';

    for (const row of parsedDiff.rows) {
      if (row.kind === 'gap') {
        const gap = document.createElement('span');
        gap.className = 'agent-ui-client-diff-gap';
        gap.textContent = `${row.count} unmodified lines`;
        lines.append(gap);
        continue;
      }

      if (row.kind === 'insert') addedLines += 1;
      if (row.kind === 'delete') removedLines += 1;
      const element = document.createElement('span');
      element.className = `agent-ui-client-diff-row agent-ui-client-diff-row-${row.kind}`;

      const number = document.createElement('span');
      number.className = 'agent-ui-client-diff-line-number';
      number.textContent = String(row.lineNumber);

      const marker = document.createElement('span');
      marker.className = 'agent-ui-client-diff-marker';
      marker.textContent = row.marker;

      const code = document.createElement('span');
      code.className = 'agent-ui-client-diff-code hljs';
      code.innerHTML = highlightCode(row.code, language) || ' ';

      element.append(number, marker, code);
      lines.append(element);
    }

    pre.append(lines);
    nextViewport.append(pre);
    container.replaceChildren(nextViewport);
    viewport = nextViewport;
    options.onDiffChange?.(Object.freeze({ addedLines, removedLines }));
  };

  render();
  return {
    ready: Promise.resolve(),
    getValue: () => value,
    setValue(nextValue) {
      value = nextValue;
      render();
    },
    focus: () => viewport?.focus(),
    layout: () => undefined,
    dispose() {
      disposed = true;
      themeObserver?.disconnect();
      container.replaceChildren();
      viewport = undefined;
    },
  };
}

function createPatch(options: ClientToolDiffPatchOptions): string {
  return createTwoFilesPatch(
    options.oldFileName ?? 'original',
    options.newFileName ?? options.oldFileName ?? 'modified',
    options.original,
    options.modified,
    options.oldHeader,
    options.newHeader,
    { context: options.contextLines ?? 3 },
  );
}

export const clientToolUI: ClientToolUIPrimitives = Object.freeze({
  codeEditor: Object.freeze({ create: createCodeEditor }),
  diffEditor: Object.freeze({ createPatch, create: createDiffEditor }),
});
