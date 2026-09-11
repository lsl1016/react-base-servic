import {
    createEmptyHistoryState,
    registerHistory,
} from '@lexical/history';
import { registerPlainText } from '@lexical/plain-text';
import {
    $createLineBreakNode,
    $createParagraphNode,
    $createTextNode,
    $getRoot,
    $getSelection,
    $isElementNode,
    $isLineBreakNode,
    $isNodeSelection,
    $isRangeSelection,
    $isTextNode,
    CLEAR_HISTORY_COMMAND,
    COMMAND_PRIORITY_HIGH,
    COMMAND_PRIORITY_LOW,
    COPY_COMMAND,
    createEditor,
    CUT_COMMAND,
    HISTORY_PUSH_TAG,
    KEY_BACKSPACE_COMMAND,
    KEY_DELETE_COMMAND,
    KEY_ENTER_COMMAND,
    KEY_ESCAPE_COMMAND,
    KEY_TAB_COMMAND,
    PASTE_COMMAND,
    type LexicalEditor,
    type LexicalNode,
} from 'lexical';
import { createEffect, createMemo, createSignal, onCleanup, onMount, Show } from 'solid-js';
import { InputPartsView } from './InputPartsView';
import { $createShortcutNode, $isShortcutNode, ShortcutNode } from './ShortcutNode';
import { SlashCommandPlugin } from './SlashCommandPlugin';
import type { AgentInputPart, AgentQuickInsertItem, AgentQuickInsertShortcutItem } from './types';
import { parseAgentInputText, serializeAgentInputParts } from './types';

export interface RichInputEditorProps {
  placeholder: string;
  disabled?: boolean;
  resetKey: number;
  draft?: {
    key: number;
    parts: AgentInputPart[];
  };
  quickInsertItems?: AgentQuickInsertItem[];
  autocompleteItems?: AgentInputPart[][];
  onSubmit: () => void;
  onChange: (parts: AgentInputPart[], isEmpty: boolean, isComposing: boolean) => void;
  onQuickInsertSelect?: (item: AgentQuickInsertShortcutItem) => void;
}

function readNodeParts(node: LexicalNode): AgentInputPart[] {
  if ($isShortcutNode(node)) {
    return [node.toInputPart()];
  }

  if ($isTextNode(node)) {
    const text = node.getTextContent();
    return text ? [{ type: 'text', text }] : [];
  }

  if ($isLineBreakNode(node)) {
    return [{ type: 'text', text: '\n' }];
  }

  if ($isElementNode(node)) {
    return node.getChildren().flatMap(readNodeParts);
  }

  return [];
}

function readEditorParts(): AgentInputPart[] {
  const root = $getRoot();
  return root.getChildren().flatMap((child, index, children) => {
    const parts = readNodeParts(child);
    if (index < children.length - 1) {
      return [...parts, { type: 'text' as const, text: '\n' }];
    }
    return parts;
  });
}

function isPartsEmpty(parts: AgentInputPart[]): boolean {
  return parts.every((part) => {
    if (part.type === 'shortcut') return false;
    return !part.text.trim();
  });
}

type CompletionToken = {
  text: string;
  part: AgentInputPart;
};

function tokenizeTextPart(text: string): CompletionToken[] {
  const tokens: CompletionToken[] = [];
  const punctuationPattern = /[。！？!?；;，,、.：:（）()【】\[\]{}《》<>“”"‘’'`…—\-~·@#$%^&*_+=|\\/]/;
  let buffer = '';

  const pushBuffer = () => {
    if (!buffer) return;
    tokens.push({ text: buffer, part: { type: 'text', text: buffer } });
    buffer = '';
  };

  for (const char of text) {
    if (char === ' ' || char === '\n' || char === '\t') {
      pushBuffer();
      tokens.push({ text: char, part: { type: 'text', text: char } });
      continue;
    }

    buffer += char;
    if (punctuationPattern.test(char)) {
      pushBuffer();
    }
  }

  pushBuffer();
  return tokens;
}

function tokenizeParts(parts: AgentInputPart[]): CompletionToken[] {
  return parts.flatMap((part) => {
    if (part.type === 'shortcut') {
      return [{ text: part.label, part }];
    }

    return tokenizeTextPart(part.text);
  });
}

function mergeTextTokensToParts(tokens: CompletionToken[]): AgentInputPart[] {
  const parts: AgentInputPart[] = [];
  for (const token of tokens) {
    const part = token.part;
    if (part.type === 'shortcut') {
      parts.push(part);
      continue;
    }

    const last = parts[parts.length - 1];
    if (last?.type === 'text') {
      last.text += part.text;
    } else {
      parts.push({ type: 'text', text: part.text });
    }
  }

  return parts;
}

function isWhitespaceToken(token: CompletionToken): boolean {
  return /^\s+$/.test(token.text);
}

function getCompletionSuffixFromTokenStart(inputTokens: CompletionToken[], candidateTokens: CompletionToken[], startIndex: number): AgentInputPart[] {
  if (startIndex + inputTokens.length > candidateTokens.length) return [];

  for (let index = 0; index < inputTokens.length; index += 1) {
    const inputToken = inputTokens[index];
    const candidateToken = candidateTokens[startIndex + index];
    if (!candidateToken) return [];

    const isLastInputToken = index === inputTokens.length - 1;
    const canUsePartialTextMatch = isLastInputToken && inputToken.part.type === 'text' && candidateToken.part.type === 'text';

    if (canUsePartialTextMatch && candidateToken.text.startsWith(inputToken.text) && candidateToken.text !== inputToken.text) {
      const suffix = candidateToken.text.slice(inputToken.text.length);
      const suffixTokens: CompletionToken[] = [
        {
          text: suffix,
          part: { type: 'text' as const, text: suffix },
        },
        ...candidateTokens.slice(startIndex + index + 1),
      ].filter((token) => token.text.length > 0);
      return mergeTextTokensToParts(suffixTokens);
    }

    if (inputToken.text !== candidateToken.text) return [];
  }

  return mergeTextTokensToParts(candidateTokens.slice(startIndex + inputTokens.length));
}

function getTokenizedCompletionSuffixParts(inputParts: AgentInputPart[], candidateParts: AgentInputPart[]): AgentInputPart[] {
  const inputTokens = tokenizeParts(inputParts);
  const candidateTokens = tokenizeParts(candidateParts);
  if (!inputTokens.length) return [];

  for (let startIndex = 0; startIndex < candidateTokens.length; startIndex += 1) {
    const startToken = candidateTokens[startIndex];
    if (!startToken || isWhitespaceToken(startToken)) continue;

    const suffixParts = getCompletionSuffixFromTokenStart(inputTokens, candidateTokens, startIndex);
    if (suffixParts.length) return suffixParts;
  }

  return [];
}

function hasSlashTextPart(parts: AgentInputPart[]): boolean {
  return parts.some((part) => part.type === 'text' && part.text.includes('/'));
}

function canShowCompletions(): boolean {
  const selection = $getSelection();
  return $isRangeSelection(selection) && selection.isCollapsed();
}

function resetEditor(editor: LexicalEditor) {
  editor.update(() => {
    const root = $getRoot();
    root.clear();
    root.append($createParagraphNode());
  });
  editor.dispatchCommand(CLEAR_HISTORY_COMMAND, undefined);
}

function hasShortcutPart(parts: AgentInputPart[]): boolean {
  return parts.some((part) => part.type === 'shortcut');
}

function getStructuredSelectionText(): string | null {
  const selection = $getSelection();
  if (!$isRangeSelection(selection) && !$isNodeSelection(selection)) return null;

  const parts = selection.extract().flatMap(readNodeParts);
  if (!hasShortcutPart(parts)) return null;

  return serializeAgentInputParts(parts) || null;
}

function removeCurrentSelection(): boolean {
  const selection = $getSelection();
  if ($isNodeSelection(selection)) {
    selection.deleteNodes();
    return true;
  }

  if ($isRangeSelection(selection) && !selection.isCollapsed()) {
    selection.removeText();
    return true;
  }

  return false;
}

function appendTextNodes(nodes: LexicalNode[], text: string) {
  text.split('\n').forEach((chunk, index) => {
    if (index > 0) {
      nodes.push($createLineBreakNode());
    }
    if (chunk) {
      nodes.push($createTextNode(chunk));
    }
  });
}

function createNodesFromParts(parts: AgentInputPart[]): LexicalNode[] {
  const nodes: LexicalNode[] = [];
  parts.forEach((part) => {
    if (part.type === 'text') {
      appendTextNodes(nodes, part.text);
      return;
    }

    nodes.push($createShortcutNode({
      id: part.id,
      label: part.label,
      group: part.group,
      description: part.description,
      data: part.data,
    }));
  });
  return nodes;
}

function setEditorParts(editor: LexicalEditor, parts: AgentInputPart[]) {
  editor.update(() => {
    const root = $getRoot();
    const paragraph = $createParagraphNode();
    const nodes = createNodesFromParts(parts);

    root.clear();
    paragraph.append(...nodes);
    root.append(paragraph);
    paragraph.selectEnd();
  }, { tag: HISTORY_PUSH_TAG });
  editor.focus();
}

function removeSelectedShortcutNodes(): boolean {
  const selection = $getSelection();
  if (!$isNodeSelection(selection)) return false;

  const shortcutNodes = selection.getNodes().filter($isShortcutNode);
  if (!shortcutNodes.length) return false;

  shortcutNodes.forEach((node) => node.remove());
  return true;
}

function getAdjacentShortcutNode(direction: 'backward' | 'forward'): ShortcutNode | null {
  const selection = $getSelection();
  if (!$isRangeSelection(selection) || !selection.isCollapsed()) return null;

  const anchor = selection.anchor;
  const node = anchor.getNode();
  let candidate: LexicalNode | null | undefined;

  if (anchor.type === 'text' && $isTextNode(node)) {
    if (direction === 'backward') {
      candidate = anchor.offset === 0 ? node.getPreviousSibling() : null;
    } else {
      candidate = anchor.offset === node.getTextContent().length ? node.getNextSibling() : null;
    }
  } else if (anchor.type === 'element' && $isElementNode(node)) {
    const children = node.getChildren();
    candidate = direction === 'backward' ? children[anchor.offset - 1] : children[anchor.offset];
  }

  return $isShortcutNode(candidate) ? candidate : null;
}

function removeAdjacentShortcutNode(direction: 'backward' | 'forward'): boolean {
  const shortcutNode = getAdjacentShortcutNode(direction);
  if (!shortcutNode) return false;

  shortcutNode.remove();
  return true;
}

export function RichInputEditor(props: RichInputEditorProps) {
  let rootRef: HTMLDivElement | undefined;
  const [editor, setEditor] = createSignal<LexicalEditor>();
  const [isEmpty, setIsEmpty] = createSignal(true);
  const [inputParts, setInputParts] = createSignal<AgentInputPart[]>([]);
  const [completionOpen, setCompletionOpen] = createSignal(false);

  const isSlashMenuVisible = () => typeof document !== 'undefined' && Boolean(document.querySelector('.agent-ui-slash-menu'));

  const inlineCompletion = createMemo(() => {
    const currentParts = inputParts();
    if (!completionOpen() || isPartsEmpty(currentParts) || hasSlashTextPart(currentParts) || isSlashMenuVisible()) return undefined;

    for (const parts of props.autocompleteItems ?? []) {
      const suffixParts = getTokenizedCompletionSuffixParts(currentParts, parts);
      if (suffixParts.length) {
        return { parts, suffixParts };
      }
    }

    return undefined;
  });

  const inlineCompletionSuffixParts = () => inlineCompletion()?.suffixParts ?? [];
  const hasInlineCompletion = () => inlineCompletionSuffixParts().length > 0;

  const closeCompletions = () => {
    setCompletionOpen(false);
  };

  const acceptCompletion = () => {
    const lexicalEditor = editor();
    const completion = inlineCompletion();
    if (!lexicalEditor || !completion) return false;

    setEditorParts(lexicalEditor, [...inputParts(), ...completion.suffixParts]);
    closeCompletions();
    return true;
  };

  const focusEditor = () => {
    if (!props.disabled) {
      editor()?.focus();
    }
  };

  onMount(() => {
    const lexicalEditor = createEditor({
      namespace: 'AgentWebSdkInput',
      nodes: [ShortcutNode],
      theme: {
        paragraph: 'agent-ui-editor-paragraph',
      },
      onError(error) {
        throw error;
      },
    });

    lexicalEditor.setRootElement(rootRef ?? null);
    lexicalEditor.setEditable(!props.disabled);
    resetEditor(lexicalEditor);

    const unregisterPlainText = registerPlainText(lexicalEditor);
    const unregisterHistory = registerHistory(lexicalEditor, createEmptyHistoryState(), 300);

    const unregisterUpdate = lexicalEditor.registerUpdateListener(({ editorState }) => {
      editorState.read(() => {
        const parts = readEditorParts();
        const empty = isPartsEmpty(parts);
        setIsEmpty(empty);
        setInputParts(parts);
        setCompletionOpen(!props.disabled && !empty && canShowCompletions());
        props.onChange(parts, empty, lexicalEditor.isComposing());
      });
    });

    const unregisterEnter = lexicalEditor.registerCommand(
      KEY_ENTER_COMMAND,
      (event) => {
        if (event?.shiftKey) return false;
        event?.preventDefault();
        closeCompletions();
        props.onSubmit();
        return true;
      },
      COMMAND_PRIORITY_LOW
    );

    const unregisterTab = lexicalEditor.registerCommand(
      KEY_TAB_COMMAND,
      (event) => {
        if (!inlineCompletion()) return false;
        event?.preventDefault();
        return acceptCompletion();
      },
      COMMAND_PRIORITY_HIGH
    );

    const unregisterEscape = lexicalEditor.registerCommand(
      KEY_ESCAPE_COMMAND,
      (event) => {
        if (!completionOpen()) return false;
        event?.preventDefault();
        closeCompletions();
        return true;
      },
      COMMAND_PRIORITY_HIGH
    );

    const unregisterBackspace = lexicalEditor.registerCommand(
      KEY_BACKSPACE_COMMAND,
      (event) => {
        if (removeSelectedShortcutNodes() || removeAdjacentShortcutNode('backward')) {
          event?.preventDefault();
          return true;
        }
        return false;
      },
      COMMAND_PRIORITY_HIGH
    );

    const unregisterDelete = lexicalEditor.registerCommand(
      KEY_DELETE_COMMAND,
      (event) => {
        if (removeSelectedShortcutNodes() || removeAdjacentShortcutNode('forward')) {
          event?.preventDefault();
          return true;
        }
        return false;
      },
      COMMAND_PRIORITY_HIGH
    );

    const unregisterCopy = lexicalEditor.registerCommand(
      COPY_COMMAND,
      (event) => {
        if (props.disabled || !(event instanceof ClipboardEvent)) return false;

        const text = getStructuredSelectionText();
        if (!text) return false;

        event.preventDefault();
        event.clipboardData?.setData('text/plain', text);
        return true;
      },
      COMMAND_PRIORITY_HIGH
    );

    const unregisterCut = lexicalEditor.registerCommand(
      CUT_COMMAND,
      (event) => {
        if (props.disabled || !(event instanceof ClipboardEvent)) return false;

        const text = getStructuredSelectionText();
        if (!text) return false;

        event.preventDefault();
        event.clipboardData?.setData('text/plain', text);
        removeCurrentSelection();
        return true;
      },
      COMMAND_PRIORITY_HIGH
    );

    const unregisterPaste = lexicalEditor.registerCommand(
      PASTE_COMMAND,
      (event) => {
        if (props.disabled || !(event instanceof ClipboardEvent)) return false;
        const text = event.clipboardData?.getData('text/plain') ?? '';
        const parsedParts = parseAgentInputText(text, props.quickInsertItems);
        if (!hasShortcutPart(parsedParts)) return false;

        const selection = $getSelection();
        if (!$isRangeSelection(selection)) return false;

        const nodes = createNodesFromParts(parsedParts);
        if (!nodes.length) return false;

        event?.preventDefault();
        selection.insertNodes(nodes);
        return true;
      },
      COMMAND_PRIORITY_HIGH
    );

    setEditor(lexicalEditor);

    onCleanup(() => {
      unregisterPlainText();
      unregisterHistory();
      unregisterUpdate();
      unregisterEnter();
      unregisterTab();
      unregisterEscape();
      unregisterBackspace();
      unregisterDelete();
      unregisterCopy();
      unregisterCut();
      unregisterPaste();
      lexicalEditor.setRootElement(null);
    });
  });

  createEffect(() => {
    if (props.disabled) {
      closeCompletions();
    }
    editor()?.setEditable(!props.disabled);
  });

  createEffect(() => {
    const lexicalEditor = editor();
    props.resetKey;
    if (lexicalEditor) {
      closeCompletions();
      resetEditor(lexicalEditor);
    }
  });

  createEffect(() => {
    const lexicalEditor = editor();
    const draft = props.draft;
    if (lexicalEditor && draft) {
      closeCompletions();
      setEditorParts(lexicalEditor, draft.parts);
    }
  });

  return (
    <div class="agent-ui-editor-shell" onMouseDown={focusEditor}>
      <div
        ref={rootRef}
        class="agent-ui-editor-root agent-ui-input"
        contentEditable={!props.disabled}
        role="textbox"
        aria-label={props.placeholder}
        aria-multiline="true"
        tabIndex={props.disabled ? -1 : 0}
      />
      <Show when={isEmpty()}>
        <div class="agent-ui-editor-placeholder">{props.placeholder}</div>
      </Show>
      <Show when={!isEmpty() && hasInlineCompletion()}>
        <div class="agent-ui-editor-inline-completion">
          <span class="agent-ui-editor-inline-completion-prefix">
            <InputPartsView parts={inputParts()} fallback="" quickInsertItems={props.quickInsertItems} />
          </span>
          <span class="agent-ui-editor-inline-completion-suggestion">
            <InputPartsView parts={inlineCompletionSuffixParts()} fallback="" quickInsertItems={props.quickInsertItems} />
          </span>
        </div>
      </Show>
      <Show when={editor() && rootRef && (props.quickInsertItems?.length ?? 0) > 0}>
        <SlashCommandPlugin
          editor={editor()!}
          rootElement={rootRef!}
          items={props.quickInsertItems ?? []}
          onInsert={(items) => items.forEach((item) => props.onQuickInsertSelect?.(item))}
        />
      </Show>
    </div>
  );
}
