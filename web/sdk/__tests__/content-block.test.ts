import copy from 'copy-to-clipboard';
import { createComponent, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Step } from '../runtime/types';
import { AssistantTurnBody } from '../ui/components/AssistantTurnBody';
import { ContentBlock, parseContentSegments, renderMarkdown } from '../ui/components/ContentBlock';

vi.mock('copy-to-clipboard', () => ({ default: vi.fn(() => true) }));

afterEach(() => {
  document.body.innerHTML = '';
  vi.clearAllMocks();
});

describe('renderMarkdown', () => {
  it('renders safe color spans used by streamed model output', () => {
    const html = renderMarkdown('<span style="color:#a56f04">技能加载</span> 蓝鲸图表配置');

    expect(html).toContain('<span style="color:#a56f04">技能加载</span>');
  });

  it('supports broader span attributes directly', () => {
    const html = renderMarkdown('<span class="status" style="color: #a56f04; font-weight: 600;">技能加载</span>');

    expect(html).toContain('<span class="status" style="color: #a56f04; font-weight: 600;">技能加载</span>');
  });

  it('hides trailing incomplete span tags while streaming', () => {
    expect(renderMarkdown('<span style="color:#a56f04"')).not.toContain('&lt;span');
    expect(renderMarkdown('<span style="color:#a56f04">技能加载</spa')).toContain('<span style="color:#a56f04">技能加载');
    expect(renderMarkdown('<span style="color:#a56f04">技能加载</spa')).not.toContain('&lt;/spa');
  });

  it('keeps non-whitelisted raw HTML as the innerHTML fallback', () => {
    const html = renderMarkdown('hello <div>abc</div>');

    expect(html).toContain('<div>abc</div>');
    expect(html).not.toContain('&lt;div&gt;abc&lt;/div&gt;');
  });

  it('keeps fenced HTML as code instead of raw DOM', () => {
    const html = renderMarkdown('```html\n<div>abc</div>\n```');

    expect(html).toContain('<pre><code class="hljs language-html">');
    expect(html).toContain('class="hljs-tag"');
    expect(html).not.toContain('<div>abc</div>');
  });

  it('highlights fenced code using the declared language', () => {
    const html = renderMarkdown('```typescript\nconst answer: number = 42;\n```');

    expect(html).toContain('<pre><code class="hljs language-typescript">');
    expect(html).toContain('<span class="hljs-keyword">const</span>');
    expect(html).toContain('<span class="hljs-number">42</span>');
  });

  it('escapes fenced code when the language is unknown', () => {
    const html = renderMarkdown('```unknown\n<script>alert(1)</script>\n```');

    expect(html).toContain('<pre><code class="hljs language-unknown">');
    expect(html).toContain('&lt;script&gt;alert(1)&lt;/script&gt;');
    expect(html).not.toContain('<script>');
  });

  it('opens markdown links in a new tab', () => {
    const html = renderMarkdown('[OpenAI](https://openai.com)');

    expect(html).toContain('<a target="_blank" rel="noopener noreferrer" href="https://openai.com">OpenAI</a>');
  });
});

describe('parseContentSegments', () => {
  it('hides trailing incomplete next-button markers while streaming', () => {
    expect(parseContentSegments('正文 [next-button:继续').length).toBe(1);
    expect(parseContentSegments('正文 [next-button:继续')[0]).toMatchObject({
      type: 'markdown',
      content: '正文 ',
    });
    expect(parseContentSegments('[n')).toEqual([]);
  });

  it('renders complete next-button markers as button segments', () => {
    expect(parseContentSegments('正文 [next-button:继续]')).toMatchObject([
      { type: 'markdown', content: '正文 ' },
      { type: 'next-button', content: '继续', buttonIndex: 0 },
    ]);
  });

  it('assigns button indexes in display order', () => {
    expect(parseContentSegments('[next-button:继续]\n[next-button:换一个]')).toMatchObject([
      { type: 'next-button', content: '继续', buttonIndex: 0 },
      { type: 'next-button', content: '换一个', buttonIndex: 1 },
    ]);
  });
});

describe('ContentBlock code blocks', () => {
  it('forces raw HTML links to open in a new tab', () => {
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(ContentBlock, {
        content: '<a href="https://openai.com" target="_self" rel="nofollow">OpenAI</a>',
        complete: true,
      }),
      host,
    );

    const link = host.querySelector('a');
    expect(link?.getAttribute('target')).toBe('_blank');
    expect(link?.getAttribute('rel')?.split(/\s+/)).toEqual(
      expect.arrayContaining(['nofollow', 'noopener', 'noreferrer']),
    );
    dispose();
  });

  it('shows a blinking cursor after streamed text until content_end', () => {
    const host = document.createElement('div');
    const [complete, setComplete] = createSignal(false);
    const dispose = render(
      () => createComponent(ContentBlock, {
        content: '正在生成 **回答**',
        get complete() {
          return complete();
        },
      }),
      host,
    );

    const cursor = host.querySelector('.agent-ui-streaming-cursor');
    expect(cursor).not.toBeNull();
    expect(cursor?.parentElement?.tagName).toBe('P');
    expect(cursor?.previousSibling?.textContent).toContain('回答');

    setComplete(true);
    expect(host.querySelector('.agent-ui-streaming-cursor')).toBeNull();
    dispose();
  });

  it('hides the streaming cursor when the active run ends without content_end', () => {
    const host = document.createElement('div');
    const [isRunning, setIsRunning] = createSignal(true);
    const step: Step = {
      index: 0,
      runId: 'run-1',
      role: 'assistant',
      thoughts: '',
      content: '未收到 content_end 的回答',
      toolCalls: [],
      thoughtComplete: true,
      contentStarted: true,
      contentComplete: false,
    };
    const dispose = render(
      () => createComponent(AssistantTurnBody, {
        items: [{ kind: 'step', step }],
        isActiveTurn: true,
        get isRunning() {
          return isRunning();
        },
      }),
      host,
    );

    expect(host.querySelector('.agent-ui-streaming-cursor')).not.toBeNull();
    setIsRunning(false);
    expect(host.querySelector('.agent-ui-streaming-cursor')).toBeNull();
    dispose();
  });

  it('preserves markdown DOM identity while streamed content grows', () => {
    const host = document.createElement('div');
    const [content, setContent] = createSignal('正在生成');
    const dispose = render(
      () => createComponent(ContentBlock, {
        get content() {
          return content();
        },
        complete: false,
      }),
      host,
    );

    const paragraph = host.querySelector('p');
    setContent('正在生成更多内容');

    expect(host.querySelector('p')).toBe(paragraph);
    expect(paragraph?.textContent).toBe('正在生成更多内容');
    dispose();
  });

  it('mounts fenced code as a SolidJS component and copies the raw code', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(
      () => createComponent(ContentBlock, {
        content: '```typescript\nconst answer: number = 42;\n```',
        complete: true,
      }),
      host,
    );

    expect(host.querySelector('.agent-ui-code-block')).not.toBeNull();
    expect(host.querySelector('.hljs-keyword')?.textContent).toBe('const');

    const copyButton = host.querySelector<HTMLButtonElement>('button[aria-label="复制"]');
    expect(copyButton).not.toBeNull();
    expect(copyButton?.textContent).toBe('复制');
    copyButton?.click();
    expect(copy).toHaveBeenCalledWith('const answer: number = 42;');
    expect(copyButton?.textContent).toBe('已复制');
    expect(copyButton?.getAttribute('aria-label')).toBe('已复制');
    expect(copyButton?.classList.contains('is-active')).toBe(true);
    expect(copyButton?.querySelector('svg')).not.toBeNull();
    dispose();
  });

  it('persists copied state in memory by related tool use ID', () => {
    const createItems = (toolUseId: string): { kind: 'step'; step: Step }[] => [
      {
        kind: 'step',
        step: {
          index: 0,
          runId: 'run-copy-memory',
          role: 'assistant',
          thoughts: '',
          content: '',
          toolCalls: [{
            toolUseId,
            toolName: 'sql_parse',
            input: {},
            status: 'done',
            result: 'ok',
            executedBy: 'server',
          }],
          thoughtComplete: true,
          contentStarted: false,
          contentComplete: false,
        },
      },
      {
        kind: 'step',
        step: {
          index: 1,
          runId: 'run-copy-memory',
          role: 'assistant',
          thoughts: '',
          content: '```sql\nselect 1;\n```',
          toolCalls: [],
          thoughtComplete: true,
          contentStarted: true,
          contentComplete: true,
        },
      },
    ];
    const mount = (host: HTMLDivElement, toolUseId: string) => render(
      () => createComponent(AssistantTurnBody, {
        items: createItems(toolUseId),
        isActiveTurn: false,
        isRunning: false,
      }),
      host,
    );

    const firstHost = document.createElement('div');
    document.body.appendChild(firstHost);
    const disposeFirst = mount(firstHost, 'tool-use-copy-memory-1');
    firstHost.querySelector<HTMLButtonElement>('button[aria-label="复制"]')?.click();
    expect(firstHost.querySelector('button[aria-label="已复制"] svg')).not.toBeNull();
    disposeFirst();

    const sameToolHost = document.createElement('div');
    document.body.appendChild(sameToolHost);
    const disposeSameTool = mount(sameToolHost, 'tool-use-copy-memory-1');
    expect(sameToolHost.querySelector('button[aria-label="已复制"] svg')).not.toBeNull();
    disposeSameTool();

    const differentToolHost = document.createElement('div');
    document.body.appendChild(differentToolHost);
    const disposeDifferentTool = mount(differentToolHost, 'tool-use-copy-memory-2');
    expect(differentToolHost.querySelector('button[aria-label="复制"]')).not.toBeNull();
    disposeDifferentTool();
  });

  it('reports code button and selected code copy actions', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const onCodeCopy = vi.fn();
    const onCodeSelectionCopy = vi.fn();
    const selection = vi.spyOn(window, 'getSelection').mockReturnValue({
      toString: () => 'select 1',
    } as Selection);
    const dispose = render(
      () => createComponent(ContentBlock, {
        content: '```sql\nselect 1;\n```',
        complete: true,
        onCodeCopy,
        onCodeSelectionCopy,
      }),
      host,
    );

    host.querySelector<HTMLButtonElement>('button[aria-label="复制"]')?.click();
    expect(onCodeCopy).toHaveBeenCalledWith({ content: 'select 1;', language: 'sql' });

    const codeBlock = host.querySelector<HTMLElement>('.agent-ui-code-block')!;
    codeBlock.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }));
    codeBlock.dispatchEvent(new Event('copy', { bubbles: true }));
    expect(onCodeSelectionCopy).toHaveBeenCalledWith({
      content: 'select 1',
      language: 'sql',
      copyMethod: 'contextmenu',
    });

    selection.mockRestore();
    dispose();
  });

  it('keeps the language label while content streams', () => {
    const host = document.createElement('div');
    const [content, setContent] = createSignal('```typescript\nconst first = 1;\nconst second = 2;\n```');
    const dispose = render(
      () => createComponent(ContentBlock, {
        get content() {
          return content();
        },
        complete: false,
      }),
      host,
    );

    const language = host.querySelector('.agent-ui-code-block-language');
    expect(language?.textContent).toBe('typescript');

    setContent('```typescript\nconst first = 1;\nconst second = 2;\nconst third = 3;\n```');
    expect(host.querySelector('.agent-ui-code-block-language')).toBe(language);
    expect(language?.textContent).toBe('typescript');
    dispose();
  });

  it('renders fullscreen through a fixed portal and exits with Escape', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(
      () => createComponent(ContentBlock, {
        content: '```sql\nselect 1;\n```',
        complete: true,
      }),
      host,
    );

    host.querySelector<HTMLButtonElement>('button[aria-label="全屏"]')?.click();
    expect(document.body.querySelector('.agent-ui-code-block-fullscreen')).not.toBeNull();
    const fullscreenButton = document.body.querySelector<HTMLButtonElement>('button[aria-label="取消全屏"]');
    expect(fullscreenButton).not.toBeNull();
    expect(fullscreenButton?.classList.contains('is-active')).toBe(true);

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(document.body.querySelector('.agent-ui-code-block-fullscreen')).toBeNull();
    expect(host.querySelector('button[aria-label="全屏"]')).not.toBeNull();
    dispose();
  });

  it('keeps fullscreen state and component identity while code streams', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const [content, setContent] = createSignal('```typescript\nconst answer = 4;\n```');
    const dispose = render(
      () => createComponent(ContentBlock, {
        get content() {
          return content();
        },
        complete: false,
      }),
      host,
    );

    host.querySelector<HTMLButtonElement>('button[aria-label="全屏"]')?.click();
    const fullscreenBlock = document.body.querySelector('.agent-ui-code-block-fullscreen');

    setContent('```typescript\nconst answer = 42;\n```');

    const updatedFullscreenBlock = document.body.querySelector('.agent-ui-code-block-fullscreen');
    expect(updatedFullscreenBlock).toBe(fullscreenBlock);
    expect(updatedFullscreenBlock?.textContent).toContain('42');

    updatedFullscreenBlock
      ?.querySelector<HTMLButtonElement>('button[aria-label="复制"]')
      ?.click();
    expect(copy).toHaveBeenCalledWith('const answer = 42;');
    dispose();
  });

  it('follows streamed code until the user scrolls away from the bottom', () => {
    const host = document.createElement('div');
    const [content, setContent] = createSignal('```text\nfirst line\n```');
    const dispose = render(
      () => createComponent(ContentBlock, {
        get content() {
          return content();
        },
        complete: false,
      }),
      host,
    );

    const scrollArea = host.querySelector<HTMLElement>('.agent-ui-code-block-scroll');
    expect(scrollArea).not.toBeNull();
    let scrollHeight = 200;
    Object.defineProperties(scrollArea!, {
      scrollHeight: { configurable: true, get: () => scrollHeight },
      clientHeight: { configurable: true, get: () => 100 },
    });

    setContent('```text\nfirst line\nsecond line\n```');
    expect(scrollArea?.scrollTop).toBe(200);

    scrollArea!.scrollTop = 40;
    scrollArea?.dispatchEvent(new Event('scroll'));
    scrollHeight = 300;
    setContent('```text\nfirst line\nsecond line\nthird line\n```');
    expect(scrollArea?.scrollTop).toBe(40);

    scrollArea!.scrollTop = 200;
    scrollArea?.dispatchEvent(new Event('scroll'));
    scrollHeight = 400;
    setContent('```text\nfirst line\nsecond line\nthird line\nfourth line\n```');
    expect(scrollArea?.scrollTop).toBe(400);
    dispose();
  });

  it('takes over fenced code nested inside a blockquote', () => {
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(ContentBlock, {
        content: '> ```python\n> print("ok")\n> ```',
        complete: true,
      }),
      host,
    );

    expect(host.querySelector('blockquote .agent-ui-code-block')).not.toBeNull();
    expect(host.querySelector('.hljs-built_in')?.textContent).toBe('print');
    dispose();
  });
});
