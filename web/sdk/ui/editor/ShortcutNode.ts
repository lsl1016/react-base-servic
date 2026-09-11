import {
    $applyNodeReplacement,
    DecoratorNode,
    type EditorConfig,
    type LexicalEditor,
    type LexicalNode,
    type NodeKey,
    type SerializedLexicalNode,
    type Spread,
} from 'lexical';
import type { JSX } from 'solid-js';
import type { AgentInputPart, AgentQuickInsertData, AgentQuickInsertShortcutItem } from './types';

export type SerializedShortcutNode = Spread<
  {
    id: string;
    label: string;
    group: string;
    description: string;
    data: AgentQuickInsertData;
  },
  SerializedLexicalNode
>;

export class ShortcutNode extends DecoratorNode<JSX.Element | null> {
  __id: string;
  __label: string;
  __group: string;
  __description: string;
  __data: AgentQuickInsertData;

  static getType(): string {
    return 'shortcut';
  }

  static clone(node: ShortcutNode): ShortcutNode {
    return new ShortcutNode(
      {
        id: node.__id,
        label: node.__label,
        group: node.__group,
        description: node.__description,
        data: node.__data,
      },
      node.__key
    );
  }

  static importJSON(serializedNode: SerializedShortcutNode): ShortcutNode {
    return $createShortcutNode({
      id: serializedNode.id,
      label: serializedNode.label,
      group: serializedNode.group,
      description: serializedNode.description,
      data: serializedNode.data,
    });
  }

  constructor(item: AgentQuickInsertShortcutItem, key?: NodeKey) {
    super(key);
    this.__id = item.id;
    this.__label = item.label;
    this.__group = item.group;
    this.__description = item.description;
    this.__data = item.data;
  }

  createDOM(_config: EditorConfig, _editor: LexicalEditor): HTMLElement {
    const span = document.createElement('span');
    span.className = 'agent-ui-shortcut-node';
    span.contentEditable = 'false';
    span.dataset.shortcutId = this.__id;
    span.textContent = this.__label;
    return span;
  }

  updateDOM(prevNode: ShortcutNode, dom: HTMLElement): boolean {
    if (prevNode.__label !== this.__label) {
      dom.textContent = this.__label;
    }
    if (prevNode.__id !== this.__id) {
      dom.dataset.shortcutId = this.__id;
    }
    return false;
  }

  exportJSON(): SerializedShortcutNode {
    return {
      type: 'shortcut',
      version: 1,
      id: this.__id,
      label: this.__label,
      group: this.__group,
      description: this.__description,
      data: this.__data,
    };
  }

  getTextContent(): string {
    return this.__label;
  }

  isInline(): true {
    return true;
  }

  isIsolated(): true {
    return true;
  }

  isKeyboardSelectable(): true {
    return true;
  }

  decorate(): JSX.Element | null {
    return null;
  }

  toInputPart(): Extract<AgentInputPart, { type: 'shortcut' }> {
    return {
      type: 'shortcut',
      id: this.__id,
      label: this.__label,
      group: this.__group,
      description: this.__description,
      data: this.__data,
    };
  }
}

export function $createShortcutNode(item: AgentQuickInsertShortcutItem): ShortcutNode {
  return $applyNodeReplacement(new ShortcutNode(item));
}

export function $isShortcutNode(node: LexicalNode | null | undefined): node is ShortcutNode {
  return node instanceof ShortcutNode;
}
