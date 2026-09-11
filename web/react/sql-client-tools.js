const INITIAL_SQL = String.raw`select  -- 订单日报示例
  o.order_date as order_date,
  o.region as region,
  count(distinct o.order_id) as order_count,
  count(distinct o.customer_id) as customer_count,
  round(sum(o.pay_amount), 2) as total_amount,
  round(sum(o.pay_amount) / count(distinct o.order_id), 2) as avg_order_amount
from demo_db.dwd_order_detail_dayinc o
where o.dt = '{@date}'
  and o.order_status in ('paid', 'shipped', 'done')
group by
  o.order_date,
  o.region
order by
  o.order_date desc,
  total_amount desc
`;

let currentSQL = INITIAL_SQL;
let lastReadSQL = INITIAL_SQL;
const pendingChanges = new Map();
const changeListeners = new Map();

const isObject = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);

const notifyChange = (toolUseId) => {
  for (const listener of changeListeners.get(toolUseId) ?? []) {
    listener();
  }
};

const subscribeChange = (toolUseId, listener) => {
  const listeners = changeListeners.get(toolUseId) ?? new Set();
  listeners.add(listener);
  changeListeners.set(toolUseId, listeners);
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0) {
      changeListeners.delete(toolUseId);
    }
  };
};

const finishChange = (change, result) => {
  if (!change || change.status !== 'pending') {
    return;
  }
  change.status = result.isError ? 'rejected' : 'accepted';
  change.message = String(result.content);
  notifyChange(change.toolUseId);
  change.resolve({
    ...result,
    meta: {
      version: 1,
      kind: 'diff',
      fileName: 'current.sql',
      language: 'sql',
      original: change.original,
      modified: change.modified,
    },
  });
};

const acceptChange = (toolUseId) => {
  const change = pendingChanges.get(toolUseId);
  if (!change || change.status !== 'pending') {
    return;
  }
  if (currentSQL !== change.base) {
    finishChange(change, {
      content: 'SQL 已在其他修改中更新，请重新调用 read_sql 后再提交 str_replace。',
      isError: true,
    });
    return;
  }

  currentSQL = change.modified;
  finishChange(change, {
    content: `SQL 修改已接受，当前最新 SQL 共 ${currentSQL.length} 个字符。`,
  });
};

const rejectChange = (toolUseId, message = '用户拒绝了本次 SQL 修改。') => {
  const change = pendingChanges.get(toolUseId);
  finishChange(change, { content: message, isError: true });
};

const countOccurrences = (source, search) => {
  let count = 0;
  let offset = 0;
  while (offset <= source.length - search.length) {
    const index = source.indexOf(search, offset);
    if (index === -1) break;
    count += 1;
    offset = index + search.length;
  }
  return count;
};

const createPendingChange = (input, context) => {
  if (!isObject(input)) {
    return { error: 'str_replace 参数必须是对象。' };
  }

  const { old_string: oldString, new_string: newString } = input;
  const unexpectedKeys = Object.keys(input).filter((key) => key !== 'old_string' && key !== 'new_string');
  if (unexpectedKeys.length > 0) {
    return { error: `str_replace 不支持参数：${unexpectedKeys.join(', ')}。` };
  }
  if (typeof oldString !== 'string' || oldString.length === 0) {
    return { error: 'old_string 必须是非空字符串。' };
  }
  if (typeof newString !== 'string') {
    return { error: 'new_string 必须是字符串。' };
  }

  const occurrences = countOccurrences(currentSQL, oldString);
  if (occurrences === 0) {
    return { error: 'old_string 未出现在当前 SQL 中，请先调用 read_sql 获取最新内容。' };
  }
  if (occurrences > 1) {
    return { error: 'old_string 在当前 SQL 中出现多次，请提供能够唯一定位的更完整片段。' };
  }

  const index = currentSQL.indexOf(oldString);
  const modified = `${currentSQL.slice(0, index)}${newString}${currentSQL.slice(index + oldString.length)}`;
  let resolve;
  const result = new Promise((resolveResult) => {
    resolve = resolveResult;
  });
  const change = {
    toolUseId: context.toolUseId,
    base: currentSQL,
    original: lastReadSQL,
    modified,
    oldString,
    newString,
    status: 'pending',
    message: '等待用户确认',
    resolve,
    result,
  };
  pendingChanges.set(context.toolUseId, change);
  notifyChange(context.toolUseId);
  return { change };
};

const strReplaceRenderer = {
  mount(container, initialContext) {
    let context = initialContext;
    let editor;
    let editorLoading = false;
    let editorReady = false;
    let disposed = false;

    const card = document.createElement('section');
    card.className = 'rp-sql-diff';

    const header = document.createElement('header');
    header.className = 'rp-sql-diff-header';

    const titleWrap = document.createElement('div');
    titleWrap.className = 'rp-sql-diff-title-wrap';

    const title = document.createElement('strong');
    title.className = 'rp-sql-diff-title';
    title.textContent = 'current.sql';

    const addedSummary = document.createElement('span');
    addedSummary.className = 'rp-sql-diff-summary rp-sql-diff-summary-added';
    addedSummary.textContent = '+0';

    const removedSummary = document.createElement('span');
    removedSummary.className = 'rp-sql-diff-summary rp-sql-diff-summary-removed';
    removedSummary.textContent = '-0';

    const status = document.createElement('span');
    status.className = 'rp-sql-diff-status';

    titleWrap.append(title, addedSummary, removedSummary);
    header.append(titleWrap, status);

    const editorHost = document.createElement('div');
    editorHost.className = 'rp-sql-diff-editor';

    const footer = document.createElement('footer');
    footer.className = 'rp-sql-diff-footer';

    const hint = document.createElement('span');
    hint.className = 'rp-sql-diff-hint';

    const actions = document.createElement('div');
    actions.className = 'rp-sql-diff-actions';

    const rejectButton = document.createElement('button');
    rejectButton.className = 'rp-sql-diff-button rp-sql-diff-reject';
    rejectButton.type = 'button';
    rejectButton.textContent = '拒绝';

    const acceptButton = document.createElement('button');
    acceptButton.className = 'rp-sql-diff-button rp-sql-diff-accept';
    acceptButton.type = 'button';
    acceptButton.textContent = '接受';

    actions.append(rejectButton, acceptButton);
    footer.append(hint, actions);
    card.append(header, editorHost, footer);
    container.replaceChildren(card);

    const render = () => {
      const liveChange = pendingChanges.get(context.toolCall.toolUseId);
      const meta = context.toolCall.meta;
      const replayChange = !liveChange && isObject(meta)
        && typeof meta.original === 'string' && typeof meta.modified === 'string'
        ? {
            original: meta.original,
            modified: meta.modified,
            fileName: typeof meta.fileName === 'string' ? meta.fileName : 'current.sql',
            language: typeof meta.language === 'string' ? meta.language : 'sql',
          }
        : undefined;
      const displayChange = liveChange ?? replayChange;
      const isPending = liveChange?.status === 'pending';

      title.textContent = displayChange?.fileName ?? 'current.sql';

      if (displayChange && !editor && !editorLoading) {
        editorLoading = true;
        const patch = context.ui.diffEditor.createPatch({
          original: displayChange.original,
          modified: displayChange.modified,
          oldFileName: displayChange.fileName ?? 'current.sql',
          newFileName: displayChange.fileName ?? 'current.sql',
          contextLines: 3,
        });
        editor = context.ui.diffEditor.create(editorHost, {
          value: patch,
          fileName: displayChange.fileName ?? 'current.sql',
          height: 150,
          onDiffChange(diffSummary) {
            addedSummary.textContent = `+${diffSummary.addedLines}`;
            removedSummary.textContent = `-${diffSummary.removedLines}`;
          },
        });
        editor.ready.then(() => {
          if (disposed) return;
          editorReady = true;
          render();
        }).catch((error) => {
          if (disposed) return;
          editorHost.textContent = `Diff 编辑器加载失败：${error instanceof Error ? error.message : String(error)}`;
          editorHost.classList.add('rp-sql-diff-editor-error');
          hint.textContent = '无法预览变更';
          render();
        });
      }

      const replayStatus = context.toolCall.status === 'done' && !context.toolCall.isError
        ? '已接受'
        : context.toolCall.status === 'error' || context.toolCall.isError
          ? '执行失败'
          : context.toolCall.status === 'cancelled'
            ? '已取消'
            : '等待工具执行';
      status.textContent = liveChange?.message ?? replayStatus;
      status.dataset.state = liveChange?.status ?? context.toolCall.status;
      hint.textContent = liveChange
        ? liveChange.status === 'pending'
          ? (editorReady ? '确认后才会更新当前 SQL' : '正在加载 Diff 编辑器')
          : liveChange.message
        : context.toolCall.result || (replayChange ? '历史变更仅供查看' : '没有可恢复的变更内容');
      rejectButton.disabled = !isPending;
      acceptButton.disabled = !isPending || !editorReady;
      footer.classList.toggle('rp-sql-diff-footer-complete', !isPending);
    };

    const unsubscribe = subscribeChange(initialContext.toolCall.toolUseId, render);
    rejectButton.addEventListener('click', () => rejectChange(context.toolCall.toolUseId));
    acceptButton.addEventListener('click', () => acceptChange(context.toolCall.toolUseId));
    render();

    return {
      update(nextContext) {
        context = nextContext;
        render();
      },
      unmount() {
        disposed = true;
        unsubscribe();
        editor?.dispose();
        container.replaceChildren();
      },
    };
  },
};

const readSQLTool = {
  name: 'read_sql',
  description: '读取当前最新的 SQL 内容。修改 SQL 前请先调用 read_sql 获取最新版本，然后使用 str_replace 更新或重写 SQL。',
  riskLevel: 'read',
  requiresConfirmation: false,
  inputSchema: {
    type: 'object',
    properties: {},
    additionalProperties: false,
  },
  ui: {
    type: 'explore',
    options: {
      prefixText: '读取 SQL',
      defaultExpanded: false,
      showInput: false,
      showResult: true,
    },
  },
  async execute(input, context) {
    if (input != null && (!isObject(input) || Object.keys(input).length > 0)) {
      return { content: 'read_sql 不接受任何参数。', isError: true };
    }
    lastReadSQL = currentSQL;
    return {
      content: currentSQL,
      data: { toolUseId: context.toolUseId, length: currentSQL.length },
    };
  },
};

const strReplaceTool = {
  name: 'str_replace',
  description: '更新或重写当前 SQL。必须先使用 read_sql 读取最新 SQL，再用 old_string 精确指定唯一待替换内容，用 new_string 提供新内容；重写全文时 old_string 传入当前完整 SQL。',
  riskLevel: 'write',
  requiresConfirmation: true,
  uiPlacement: 'root',
  inputSchema: {
    type: 'object',
    additionalProperties: false,
    required: ['old_string', 'new_string'],
    properties: {
      old_string: {
        type: 'string',
        minLength: 1,
        description: '当前最新 SQL 中需要被替换的原始文本，必须能够唯一匹配。',
      },
      new_string: {
        type: 'string',
        description: '替换后的新文本；传空字符串表示删除 old_string。',
      },
    },
  },
  ui: {
    type: 'custom',
    renderer: strReplaceRenderer,
  },
  execute(input, context) {
    const pending = createPendingChange(input, context);
    if (pending.error) {
      return Promise.resolve({ content: pending.error, isError: true });
    }
    return pending.change.result;
  },
};

export const registerSQLClientTools = (client) => {
  if (!client || typeof client.registerTools !== 'function') {
    throw new TypeError('registerSQLClientTools 需要 AgentClient 实例。');
  }

  client.registerTools([readSQLTool, strReplaceTool]);

  return () => {
    for (const change of pendingChanges.values()) {
      if (change.status === 'pending') {
        rejectChange(change.toolUseId, '客户端工具已卸载，本次 SQL 修改未提交。');
      }
    }
  };
};
