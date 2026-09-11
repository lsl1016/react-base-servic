import copy from "copy-to-clipboard";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import css from "highlight.js/lib/languages/css";
import go from "highlight.js/lib/languages/go";
import java from "highlight.js/lib/languages/java";
import javascript from "highlight.js/lib/languages/javascript";
import json from "highlight.js/lib/languages/json";
import markdown from "highlight.js/lib/languages/markdown";
import python from "highlight.js/lib/languages/python";
import sql from "highlight.js/lib/languages/sql";
import typescript from "highlight.js/lib/languages/typescript";
import xml from "highlight.js/lib/languages/xml";
import yaml from "highlight.js/lib/languages/yaml";
import { Show, createEffect, createMemo, createSignal, onCleanup } from "solid-js";
import { Portal } from "solid-js/web";
import IconAkarIconsCheck from "~icons/akar-icons/check";
import { useAgentUITheme } from "../theme";
import { AutoScrollPanel } from "./AutoScrollPanel";

const highlightLanguages = {
  bash,
  css,
  go,
  java,
  javascript,
  json,
  markdown,
  python,
  sql,
  typescript,
  xml,
  yaml,
};

for (const [name, language] of Object.entries(highlightLanguages)) {
  hljs.registerLanguage(name, language);
}

const escapeHtml = (value: string) => value
  .replace(/&/g, "&amp;")
  .replace(/</g, "&lt;")
  .replace(/>/g, "&gt;")
  .replace(/"/g, "&quot;")
  .replace(/'/g, "&#39;");

export interface CodeBlockData {
  code: string;
  language: string;
  highlightedHtml: string;
}

export type CodeBlockCopyData = {
  content: string;
  language: string;
};

export type CodeBlockSelectionCopyData = CodeBlockCopyData & {
  copyMethod: 'keyboard' | 'contextmenu';
};

export type CodeBlockProps = CodeBlockData & {
  copyMemoryKey?: string;
  onCopy?: (data: CodeBlockCopyData) => void;
  onSelectionCopy?: (data: CodeBlockSelectionCopyData) => void;
};

interface CodeBlockMemoryState {
  copied: boolean;
  copiedAt: number;
}

const codeBlockMemory = new Map<string, CodeBlockMemoryState>();

const countCodeLines = (code: string) => code.length === 0
  ? 0
  : code.split(/\r\n|\r|\n/).length;

export const createCodeBlockData = (code: string, info?: string): CodeBlockData => {
  const candidate = info?.match(/^\S+/)?.[0]?.toLowerCase() ?? "";
  const language = /^[a-z0-9_+#.-]+$/i.test(candidate) ? candidate : "";
  const highlightedHtml = language && hljs.getLanguage(language)
    ? hljs.highlight(code, { language, ignoreIllegals: true }).value
    : escapeHtml(code);

  return { code, language, highlightedHtml };
};

export function CodeBlock(props: CodeBlockProps) {
  const [fullscreen, setFullscreen] = createSignal(false);
  const [copied, setCopied] = createSignal(
    props.copyMemoryKey ? codeBlockMemory.get(props.copyMemoryKey)?.copied ?? false : false,
  );
  const lineCount = createMemo(() => countCodeLines(props.code));
  const theme = useAgentUITheme();
  let lastContextMenuAt = 0;

  const handleCopy = () => {
    if (!copy(props.code)) return;
    if (props.copyMemoryKey) {
      codeBlockMemory.set(props.copyMemoryKey, {
        copied: true,
        copiedAt: Date.now(),
      });
    }
    setCopied(true);
    props.onCopy?.({ content: props.code, language: props.language });
  };

  const handleSelectionCopy = () => {
    const content = window.getSelection()?.toString() ?? '';
    if (!content) return;
    props.onSelectionCopy?.({
      content,
      language: props.language,
      copyMethod: Date.now() - lastContextMenuAt <= 15_000 ? 'contextmenu' : 'keyboard',
    });
  };

  createEffect(() => {
    const memoryKey = props.copyMemoryKey;
    void props.code;
    setCopied(memoryKey ? codeBlockMemory.get(memoryKey)?.copied ?? false : false);
  });

  createEffect(() => {
    if (!fullscreen()) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setFullscreen(false);
    };
    document.addEventListener("keydown", handleKeyDown);
    onCleanup(() => document.removeEventListener("keydown", handleKeyDown));
  });

  const renderBlock = (isFullscreen: boolean) => (
    <section
      class="agent-ui-code-block"
      classList={{ "agent-ui-code-block-fullscreen": isFullscreen }}
      data-agent-ui-theme={theme()}
      aria-label={`${props.language || "text"}代码块`}
      onContextMenu={() => {
        lastContextMenuAt = Date.now();
      }}
      onCopy={handleSelectionCopy}
    >
      <header class="agent-ui-code-block-toolbar">
        <div class="agent-ui-code-block-meta">
          {/* <span class="agent-ui-code-block-additions">示例代码</span> */}
          <span class="agent-ui-code-block-language">{props.language || "text"}</span>
        
        </div>
        <div class="agent-ui-code-block-actions">
          <button
            type="button"
            class="agent-ui-code-block-action agent-ui-code-block-copy-action"
            classList={{ "is-active": copied() }}
            title={copied() ? "已复制" : "复制"}
            aria-label={copied() ? "已复制" : "复制"}
            onClick={handleCopy}
          >
            <Show when={copied()}>
              <IconAkarIconsCheck width="14" height="14" aria-hidden="true" />
            </Show>
            {copied() ? "已复制" : "复制"}
          </button>
          <button
            type="button"
            class="agent-ui-code-block-action agent-ui-code-block-fullscreen-action"
            classList={{ "is-active": isFullscreen }}
            title={isFullscreen ? "取消全屏" : "全屏"}
            aria-label={isFullscreen ? "取消全屏" : "全屏"}
            aria-pressed={isFullscreen}
            onClick={() => setFullscreen(!isFullscreen)}
          >
            {isFullscreen ? "取消全屏" : "全屏"}
          </button>
        </div>
      </header>
      <AutoScrollPanel class="agent-ui-code-block-scroll" followKey={props.code}>
        <pre class="agent-ui-code-block-pre"><code class="hljs" innerHTML={props.highlightedHtml} /></pre>
      </AutoScrollPanel>
    </section>
  );

  return (
    <Show when={fullscreen()} fallback={renderBlock(false)}>
      <Portal>{renderBlock(true)}</Portal>
    </Show>
  );
}
