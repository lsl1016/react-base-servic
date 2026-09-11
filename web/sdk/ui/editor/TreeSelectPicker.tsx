import { createEffect, createMemo, createSignal, For, onCleanup, onMount, Show } from 'solid-js';
import IconMdiChevronDown from '~icons/mdi/chevron-down';
import IconMdiChevronRight from '~icons/mdi/chevron-right';
import { TreeCheckbox } from './TreeCheckbox';
import type {
  AgentQuickInsertTreeNode,
  AgentQuickInsertTreeSelectItem,
  AgentQuickInsertTreeSelection,
} from './types';

interface TreeEntry {
  key: string;
  node: AgentQuickInsertTreeNode;
  path: AgentQuickInsertTreeNode[];
  depth: number;
}

export interface TreeSelectPickerProps {
  item: AgentQuickInsertTreeSelectItem;
  initialOptions?: AgentQuickInsertTreeNode[];
  ref?: (handle: TreeSelectPickerHandle) => void;
  onOptionsLoaded?: (options: AgentQuickInsertTreeNode[]) => void;
  onConfirm: (selected: AgentQuickInsertTreeSelection[]) => void;
  onCancel: (selected: AgentQuickInsertTreeSelection[]) => void;
}

export interface TreeSelectPickerHandle {
  cancel: () => void;
}

function flattenVisibleNodes(
  nodes: AgentQuickInsertTreeNode[],
  expanded: Set<string>,
  path: AgentQuickInsertTreeNode[] = [],
  keyPrefix = ''
): TreeEntry[] {
  return nodes.flatMap((node, index) => {
    const key = keyPrefix ? `${keyPrefix}.${index}` : String(index);
    const nextPath = [...path, node];
    const entry: TreeEntry = { key, node, path: nextPath, depth: path.length };
    if (!node.children?.length || !expanded.has(key)) return [entry];
    return [entry, ...flattenVisibleNodes(node.children, expanded, nextPath, key)];
  });
}

function flattenFilteredNodes(
  nodes: AgentQuickInsertTreeNode[],
  query: string,
  path: AgentQuickInsertTreeNode[] = [],
  keyPrefix = ''
): { matched: boolean; entries: TreeEntry[] } {
  const entries: TreeEntry[] = [];
  let matched = false;

  nodes.forEach((node, index) => {
    const key = keyPrefix ? `${keyPrefix}.${index}` : String(index);
    const nextPath = [...path, node];
    const children = flattenFilteredNodes(node.children ?? [], query, nextPath, key);
    const nodeMatched = node.label.toLowerCase().includes(query);
    if (!nodeMatched && !children.matched) return;

    matched = true;
    entries.push({ key, node, path: nextPath, depth: path.length });
    entries.push(...children.entries);
  });

  return { matched, entries };
}

export function isTreeNodeSelectable(node: AgentQuickInsertTreeNode, leafOnly = false): boolean {
  return !node.disabled && node.selectable !== false && (!leafOnly || !node.children?.length);
}

function logTreePickerDebug(event: string, details: Record<string, unknown>, error = false) {
  const message = `[agent-ui][tree-select-debug] ${event} ${JSON.stringify(details)}`;
  if (error) console.error(message);
  else console.info(message);
}

function getSelectableLeafEntries(entry: TreeEntry): TreeEntry[] {
  if (entry.node.disabled) return [];
  if (!entry.node.children?.length) {
    return isTreeNodeSelectable(entry.node, true) ? [entry] : [];
  }
  return entry.node.children.flatMap((node, index) => getSelectableLeafEntries({
    key: `${entry.key}.${index}`,
    node,
    path: [...entry.path, node],
    depth: entry.depth + 1,
  }));
}

export function TreeSelectPicker(props: TreeSelectPickerProps) {
  let filterInputRef: HTMLInputElement | undefined;
  const [options, setOptions] = createSignal<AgentQuickInsertTreeNode[]>();
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal<string>();
  const [expanded, setExpanded] = createSignal<Set<string>>(new Set<string>());
  const [selected, setSelected] = createSignal<Map<string, AgentQuickInsertTreeSelection>>(new Map());
  const [filterQuery, setFilterQuery] = createSignal('');

  const entries = createMemo(() => {
    const query = filterQuery().trim().toLowerCase();
    return query
      ? flattenFilteredNodes(options() ?? [], query).entries
      : flattenVisibleNodes(options() ?? [], expanded());
  });
  const selectedValues = () => Array.from(selected().entries())
    .sort(([keyA], [keyB]) => {
      const pathA = keyA.split('.').map(Number);
      const pathB = keyB.split('.').map(Number);
      const length = Math.max(pathA.length, pathB.length);
      for (let index = 0; index < length; index += 1) {
        const difference = (pathA[index] ?? -1) - (pathB[index] ?? -1);
        if (difference) return difference;
      }
      return 0;
    })
    .map(([, selection]) => selection);

  props.ref?.({
    cancel: () => props.onCancel(selectedValues()),
  });

  onMount(() => {
    logTreePickerDebug('picker-mounted', { itemId: props.item.id });
    const focusFilter = () => {
      filterInputRef?.focus({ preventScroll: true });
      logTreePickerDebug('filter-focus', {
        itemId: props.item.id,
        focused: document.activeElement === filterInputRef,
      });
    };
    queueMicrotask(focusFilter);
    const focusFrame = requestAnimationFrame(focusFilter);
    onCleanup(() => cancelAnimationFrame(focusFrame));
  });

  const load = () => {
    logTreePickerDebug('load-start', { itemId: props.item.id });
    setLoading(true);
    setError(undefined);
    void Promise.resolve().then(() => props.item.picker.loadOptions()).then((nodes) => {
      setOptions(nodes);
      logTreePickerDebug('load-success', {
        itemId: props.item.id,
        rootCount: nodes.length,
      });
      props.onOptionsLoaded?.(nodes);
      setLoading(false);
    }).catch((reason: unknown) => {
      logTreePickerDebug('load-error', {
        itemId: props.item.id,
        message: reason instanceof Error ? reason.message : String(reason),
      }, true);
      setError(reason instanceof Error ? reason.message : '加载失败');
      setLoading(false);
    });
  };

  createEffect(() => {
    props.item;
    const initialOptions = props.initialOptions;
    setOptions(initialOptions);
    setExpanded(new Set<string>());
    setSelected(new Map());
    setFilterQuery('');
    if (!initialOptions) load();
  });

  const toggleExpanded = (key: string) => {
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const toggleSelected = (entry: TreeEntry) => {
    const cascadedLeaves = props.item.picker.multiple
      && props.item.picker.leafOnly
      && entry.node.children?.length
      ? getSelectableLeafEntries(entry)
      : [];
    if (!cascadedLeaves.length && !isTreeNodeSelectable(entry.node, props.item.picker.leafOnly)) return;
    setSelected((current) => {
      const next = props.item.picker.multiple
        ? new Map(current)
        : new Map<string, AgentQuickInsertTreeSelection>();
      if (cascadedLeaves.length) {
        const allSelected = cascadedLeaves.every((leaf) => next.has(leaf.key));
        cascadedLeaves.forEach((leaf) => {
          if (allSelected) next.delete(leaf.key);
          else next.set(leaf.key, { node: leaf.node, path: leaf.path });
        });
      } else if (next.has(entry.key)) {
        next.delete(entry.key);
      } else {
        next.set(entry.key, { node: entry.node, path: entry.path });
      }
      return next;
    });
  };

  return (
    <div class="agent-ui-tree-picker" data-testid="tree-select-picker">
      <div class="agent-ui-tree-picker-filter">
        <input
          ref={filterInputRef}
          type="text"
          autofocus
          value={filterQuery()}
          placeholder="请输入要搜索的内容"
          aria-label="请输入要搜索的内容"
          onInput={(event) => setFilterQuery(event.currentTarget.value)}
        />
        <span class="agent-ui-tree-picker-count">已选{selected().size}</span>
      </div>

      <div class="agent-ui-tree-picker-list" role="tree" aria-multiselectable={props.item.picker.multiple}>
        <Show when={!loading()} fallback={<div class="agent-ui-tree-picker-state">加载中...</div>}>
          <Show
            when={!error()}
            fallback={
              <div class="agent-ui-tree-picker-state agent-ui-tree-picker-error">
                <span>{error()}</span>
                <button type="button" onClick={load}>重试</button>
              </div>
            }
          >
            <Show when={(options()?.length ?? 0) > 0} fallback={<div class="agent-ui-tree-picker-state">暂无可选项</div>}>
              <Show when={entries().length > 0} fallback={<div class="agent-ui-tree-picker-state">无匹配项</div>}>
              <For each={entries()}>
                {(entry) => {
                  const hasChildren = () => Boolean(entry.node.children?.length);
                  const selectable = () => isTreeNodeSelectable(entry.node, props.item.picker.leafOnly);
                  const cascadedLeaves = () => props.item.picker.multiple && props.item.picker.leafOnly
                    ? getSelectableLeafEntries(entry)
                    : [];
                  const checkable = () => selectable()
                    || (entry.node.selectable !== false && cascadedLeaves().length > 0);
                  const checked = () => {
                    const leaves = cascadedLeaves();
                    return leaves.length > 0
                      ? leaves.every((leaf) => selected().has(leaf.key))
                      : selected().has(entry.key);
                  };
                  const indeterminate = () => {
                    const leaves = cascadedLeaves();
                    if (!leaves.length) return false;
                    const selectedCount = leaves.filter((leaf) => selected().has(leaf.key)).length;
                    return selectedCount > 0 && selectedCount < leaves.length;
                  };
                  return (
                    <div
                      class="agent-ui-tree-picker-row"
                      classList={{ 'agent-ui-tree-picker-row-disabled': Boolean(entry.node.disabled) }}
                      style={{ 'padding-left': `${8 + entry.depth * 20}px` }}
                      role="treeitem"
                      aria-expanded={hasChildren() ? expanded().has(entry.key) : undefined}
                      aria-selected={checkable() ? checked() : undefined}
                    >
                      <Show
                        when={hasChildren()}
                        fallback={<span class="agent-ui-tree-picker-spacer" />}
                      >
                        <button
                          type="button"
                          class="agent-ui-tree-picker-expand"
                          aria-label={expanded().has(entry.key) ? '收起' : '展开'}
                          onClick={() => toggleExpanded(entry.key)}
                        >
                          <Show when={expanded().has(entry.key)} fallback={<IconMdiChevronRight width="16" height="16" />}>
                            <IconMdiChevronDown width="16" height="16" />
                          </Show>
                        </button>
                      </Show>
                      <Show when={checkable()}>
                        <TreeCheckbox
                          checked={checked()}
                          indeterminate={indeterminate()}
                          disabled={entry.node.disabled}
                          label={entry.node.label}
                          onToggle={() => toggleSelected(entry)}
                        />
                      </Show>
                      <button
                        type="button"
                        class="agent-ui-tree-picker-label"
                        disabled={entry.node.disabled}
                        title={entry.node.label}
                        onClick={() => hasChildren()
                          ? toggleExpanded(entry.key)
                          : toggleSelected(entry)}
                      >
                        {entry.node.label}
                      </button>
                    </div>
                  );
                }}
              </For>
              </Show>
            </Show>
          </Show>
        </Show>
      </div>

      <div class="agent-ui-tree-picker-actions">
        <button type="button" class="agent-ui-tree-picker-button" onClick={() => props.onCancel(selectedValues())}>
          取消
        </button>
        <button
          type="button"
          class="agent-ui-tree-picker-button agent-ui-tree-picker-confirm"
          disabled={selected().size === 0}
          onClick={() => props.onConfirm(selectedValues())}
        >
          确定
        </button>
      </div>
    </div>
  );
}
