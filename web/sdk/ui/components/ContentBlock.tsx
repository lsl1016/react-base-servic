/**
 * ContentBlock - 助手回答内容渲染
 *
 * 使用 marked 库渲染 Markdown 内容。
 * 支持流式内容（逐步更新）。
 */

import { Marked, Renderer, type Tokens } from "marked";
import morphdom from "morphdom";
import { Index, Show, createEffect, createMemo, createSignal, onCleanup } from "solid-js";
import { render } from "solid-js/web";
import IconMingcuteAiLine from "~icons/mingcute/ai-line";
import {
  CodeBlock,
  createCodeBlockData,
  type CodeBlockCopyData,
  type CodeBlockData,
  type CodeBlockSelectionCopyData,
} from "./CodeBlock";

export interface ContentBlockProps {
  /** 内容文本（可能是流式更新中的） */
  content: string;
  /** 内容是否已完整 */
  complete: boolean;
  /** 代码块复制状态的内存持久化键 */
  copyMemoryKey?: string;
  /** 点击快捷下一步 */
  onNextButtonClick?: (message: string, buttonIndex: number) => void;
  /** 点击复制代码块 */
  onCodeCopy?: (data: CodeBlockCopyData) => void;
  /** 复制代码块中的选区 */
  onCodeSelectionCopy?: (data: CodeBlockSelectionCopyData) => void;
}

type ContentSegment =
  | {
      type: "markdown";
      content: string;
      html: string;
      codeBlocks: CodeBlockData[];
    }
  | {
      type: "next-button";
      content: string;
      buttonIndex: number;
    };

const escapeHtml = (value: string) => value
  .replace(/&/g, "&amp;")
  .replace(/</g, "&lt;")
  .replace(/>/g, "&gt;")
  .replace(/"/g, "&quot;")
  .replace(/'/g, "&#39;");

const hideTrailingIncompleteSpanTag = (content: string) => {
  return content.replace(/<\/?s(?:p(?:a(?:n\b[^>]*)?)?)?$/i, "");
};

const hideTrailingIncompleteNextButton = (content: string) => {
  const index = content.lastIndexOf("[");
  if (index < 0) return content;

  const tail = content.slice(index);
  if (tail.includes("]") || /[\r\n]/.test(tail)) return content;

  const marker = "[next-button:";
  const normalizedTail = tail.toLowerCase();
  if (marker.startsWith(normalizedTail) || normalizedTail.startsWith(marker)) {
    return content.slice(0, index);
  }
  return content;
};

const renderSafeInlineHtml = (html: string) => {
  if (/^<span\b[^>]*>$/i.test(html)) {
    return html;
  }
  if (/^<\/span\s*>$/i.test(html)) {
    return "</span>";
  }
  if (/^<br\s*\/?>$/i.test(html)) {
    return "<br>";
  }
  return null;
};

const defaultRenderer = new Renderer();

const forceBlankAnchorTargets = (container: HTMLElement) => {
  for (const anchor of container.querySelectorAll("a")) {
    anchor.setAttribute("target", "_blank");

    const rel = new Set((anchor.getAttribute("rel") ?? "").split(/\s+/).filter(Boolean));
    rel.add("noopener");
    rel.add("noreferrer");
    anchor.setAttribute("rel", Array.from(rel).join(" "));
  }
};

const createMarkdownParser = (codeBlocks?: CodeBlockData[]) => new Marked({
  breaks: true,
  gfm: true,
  renderer: {
    link(token: Tokens.Link) {
      const html = defaultRenderer.link.call(this, token);
      return html.replace(/^<a /, '<a target="_blank" rel="noopener noreferrer" ');
    },
    html({ text }: Tokens.HTML | Tokens.Tag) {
      return renderSafeInlineHtml(text) ?? text;
    },
    code({ text, lang }: Tokens.Code) {
      const codeBlock = createCodeBlockData(text, lang);
      if (codeBlocks) {
        const index = codeBlocks.push(codeBlock) - 1;
        return `<div data-agent-ui-code-block="${index}"></div>`;
      }

      const languageClass = codeBlock.language ? ` language-${codeBlock.language}` : "";
      return `<pre><code class="hljs${languageClass}">${codeBlock.highlightedHtml}\n</code></pre>`;
    },
  },
});

const NEXT_BUTTON_PATTERN = /\[next-button:([^\]\r\n]+)\]/g;

export const renderMarkdown = (content: string) => {
  try {
    return createMarkdownParser().parse(hideTrailingIncompleteSpanTag(content)) as string;
  } catch (error) {
    console.error("Failed to parse markdown:", error);
    return escapeHtml(content);
  }
};

const renderMarkdownSegment = (content: string) => {
  const codeBlocks: CodeBlockData[] = [];
  try {
    const html = createMarkdownParser(codeBlocks).parse(hideTrailingIncompleteSpanTag(content)) as string;
    return { html, codeBlocks };
  } catch (error) {
    console.error("Failed to parse markdown:", error);
    return { html: escapeHtml(content), codeBlocks };
  }
};

export const parseContentSegments = (content: string): ContentSegment[] => {
  const segments: ContentSegment[] = [];
  content = hideTrailingIncompleteNextButton(content);
  let lastIndex = 0;
  let buttonIndex = 0;

  for (const match of content.matchAll(NEXT_BUTTON_PATTERN)) {
    const matchedText = match[0];
    const label = match[1]?.trim();
    const index = match.index ?? 0;

    const markdown = content.slice(lastIndex, index);
    if (markdown.trim()) {
      const rendered = renderMarkdownSegment(markdown);
      segments.push({
        type: "markdown",
        content: markdown,
        ...rendered,
      });
    }

    if (label) {
      segments.push({
        type: "next-button",
        content: label,
        buttonIndex,
      });
      buttonIndex += 1;
    }

    lastIndex = index + matchedText.length;
  }

  const rest = content.slice(lastIndex);
  if (rest) {
    const rendered = renderMarkdownSegment(rest);
    segments.push({
      type: "markdown",
      content: rest,
      ...rendered,
    });
  }

  return segments;
};

const appendStreamingCursor = (container: HTMLElement) => {
  const cursor = document.createElement("span");
  cursor.className = "agent-ui-streaming-cursor";
  cursor.setAttribute("aria-hidden", "true");

  const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT);
  let lastTextNode: Text | undefined;
  while (walker.nextNode()) {
    const node = walker.currentNode as Text;
    if (node.data.trim()) lastTextNode = node;
  }

  if (!lastTextNode?.parentElement) {
    container.append(cursor);
    return;
  }

  const inlineTags = new Set(["A", "CODE", "DEL", "EM", "SPAN", "STRONG"]);
  let target = lastTextNode.parentElement;
  while (target.parentElement !== container && inlineTags.has(target.tagName)) {
    target = target.parentElement!;
  }
  target.append(cursor);
};

function MarkdownSegment(props: {
  html: string;
  codeBlocks: CodeBlockData[];
  streaming: boolean;
  copyMemoryKey?: string;
  onCodeCopy?: (data: CodeBlockCopyData) => void;
  onCodeSelectionCopy?: (data: CodeBlockSelectionCopyData) => void;
}) {
  let container: HTMLDivElement | undefined;
  const mountedCodeBlocks = new Map<number, {
    element: HTMLElement;
    setData: (data: CodeBlockData) => CodeBlockData;
    dispose: () => void;
  }>();

  const disposeMountedCodeBlocks = () => {
    for (const mounted of mountedCodeBlocks.values()) mounted.dispose();
    mountedCodeBlocks.clear();
  };

  createEffect(() => {
    if (!container) return;

    const target = document.createElement("div");
    target.innerHTML = props.html;
    forceBlankAnchorTargets(target);
    if (props.streaming) appendStreamingCursor(target);
    morphdom(container, target, {
      childrenOnly: true,
      getNodeKey(node) {
        if (!(node instanceof HTMLElement)) return undefined;
        const codeBlockIndex = node.dataset.agentUiCodeBlock;
        if (codeBlockIndex !== undefined) return `code-block-${codeBlockIndex}`;
        return node.id || undefined;
      },
      onBeforeElChildrenUpdated(fromElement) {
        return !fromElement.hasAttribute("data-agent-ui-code-block");
      },
    });

    const liveIndexes = new Set<number>();
    const placeholders = container.querySelectorAll<HTMLElement>("[data-agent-ui-code-block]");
    for (const placeholder of placeholders) {
      const index = Number(placeholder.dataset.agentUiCodeBlock);
      const codeBlock = props.codeBlocks[index];
      if (!Number.isInteger(index) || !codeBlock) continue;
      liveIndexes.add(index);

      const mounted = mountedCodeBlocks.get(index);
      if (mounted?.element === placeholder) {
        mounted.setData(codeBlock);
        continue;
      }

      mounted?.dispose();
      const [data, setData] = createSignal(codeBlock);
      const dispose = render(
        () => (
          <CodeBlock
            code={data().code}
            language={data().language}
            highlightedHtml={data().highlightedHtml}
            copyMemoryKey={props.copyMemoryKey ? `${props.copyMemoryKey}:code:${index}` : undefined}
            onCopy={props.onCodeCopy}
            onSelectionCopy={props.onCodeSelectionCopy}
          />
        ),
        placeholder,
      );
      mountedCodeBlocks.set(index, { element: placeholder, setData, dispose });
    }

    for (const [index, mounted] of mountedCodeBlocks) {
      if (liveIndexes.has(index)) continue;
      mounted.dispose();
      mountedCodeBlocks.delete(index);
    }
  });

  onCleanup(disposeMountedCodeBlocks);
  return <div class="content-html" ref={container} />;
}

export function ContentBlock(props: ContentBlockProps) {
  const segments = createMemo(() => parseContentSegments(props.content));
  const lastMarkdownIndex = createMemo(() => {
    const values = segments();
    for (let index = values.length - 1; index >= 0; index -= 1) {
      if (values[index].type === "markdown") return index;
    }
    return -1;
  });

  return (
    <div class="agent-ui-content-block agent-ui-markdown">
      <Index each={segments()}>
        {(segment, index) => {
          const markdown = createMemo(() => {
            const value = segment();
            return value.type === "markdown" ? value : undefined;
          });
          const nextButton = createMemo(() => {
            const value = segment();
            return value.type === "next-button" ? value : undefined;
          });
          return (
            <Show
              when={markdown()}
              fallback={(
                <Show when={nextButton()}>
                  {(button) => (
                    <button
                      type="button"
                      class="agent-ui-next-button"
                      disabled={!props.onNextButtonClick}
                      onClick={() => props.onNextButtonClick?.(button().content, button().buttonIndex)}
                    >
                      <IconMingcuteAiLine width="15" height="15" class="agent-ui-next-button-icon" />
                      <span>{button().content}</span>
                    </button>
                  )}
                </Show>
              )}
            >
              {(content) => (
                <MarkdownSegment
                  html={content().html}
                  codeBlocks={content().codeBlocks}
                  streaming={!props.complete && index === lastMarkdownIndex()}
                  copyMemoryKey={props.copyMemoryKey ? `${props.copyMemoryKey}:segment:${index}` : undefined}
                  onCodeCopy={props.onCodeCopy}
                  onCodeSelectionCopy={props.onCodeSelectionCopy}
                />
              )}
            </Show>
          );
        }}
      </Index>
    </div>
  );
}
