import { registerSQLClientTools } from './sql-client-tools.js';

window.REACT_AGENT_HOST_CONFIG = window.REACT_AGENT_HOST_CONFIG || undefined;

const $ = (id) => document.getElementById(id);
const container = $('agent-root');
const configBar = $('config-bar');
const callerKeyInput = $('caller-key-input');
const routeValuesInput = $('route-values-input');
const controlContextInput = $('control-context-input');
const llmContextInput = $('llm-context-input');
const moreCapabilitiesPanel = $('more-capabilities-panel');
const toggleMoreCapabilitiesButton = $('toggle-more-capabilities');
const toggleSDKThemeButton = $('toggle-sdk-theme');
const applyButton = $('apply-config');
const copyCallerButton = $('copy-caller');
const deleteCallerButton = $('delete-caller');
const copyCallerModalMask = $('copy-caller-modal-mask');
const copySourceCallerInput = $('copy-source-caller');
const copyTargetCallerInput = $('copy-target-caller');
const copyTargetNameInput = $('copy-target-name');
const copyTargetPlatformInput = $('copy-target-platform');
const copyTargetDescriptionInput = $('copy-target-description');
const copyCallerModalState = $('copy-caller-modal-state');
const copyCallerModalClose = $('copy-caller-modal-close');
const copyCallerModalCancel = $('copy-caller-modal-cancel');
const copyCallerModalConfirm = $('copy-caller-modal-confirm');
const deleteCallerModalMask = $('delete-caller-modal-mask');
const deleteCallerConfirmLabel = $('delete-caller-confirm-label');
const deleteCallerConfirmInput = $('delete-caller-confirm-input');
const deleteCallerModalState = $('delete-caller-modal-state');
const deleteCallerModalClose = $('delete-caller-modal-close');
const deleteCallerModalCancel = $('delete-caller-modal-cancel');
const deleteCallerModalConfirm = $('delete-caller-modal-confirm');
const inputAPIDemoValue = $('input-api-demo-value');
const inputAPIFillButton = $('input-api-fill');
const inputAPISubmitButton = $('input-api-submit');
const inputAPIDemoState = $('input-api-demo-state');
const openReplayLink = $('open-replay');
const replayModalMask = $('replay-modal-mask');
const replayScopeInput = $('replay-scope-input');
const replayModalState = $('replay-modal-state');
const replayModalClose = $('replay-modal-close');
const replayModalCancel = $('replay-modal-cancel');
const replayModalConfirm = $('replay-modal-confirm');
const baseUrl = `${location.origin}/react-base-service/react`;
const managementBaseUrl = `${location.origin}/react-base-service`;
let currentHandle = null;
let disposeSQLClientTools = null;
let sdkTheme = window.REACT_AGENT_HOST_CONFIG?.theme === 'dark' ? 'dark' : 'light';
const feedbackByRunId = {};

const loadMockAnalysisThemeOptions = async () => {
  await new Promise((resolve) => setTimeout(resolve, 500));
  return [
    {
      id: 'budget-2026-core',
      label: '2026 核心业务预算',
      children: [
        { id: 'scope-revenue', label: '收入与利润', payload: { budgetId: 202601, scopeId: 101 } },
        { id: 'scope-retention', label: '用户留存', payload: { budgetId: 202601, scopeId: 102 } },
      ],
    },
    {
      id: 'budget-2026-growth',
      label: '2026 增长专项预算',
      selectable: false,
      children: [
        { id: 'scope-acquisition', label: '获客转化', payload: { budgetId: 202602, scopeId: 201 } },
        { id: 'scope-disabled', label: '实验主题（暂不可用）', disabled: true, payload: { budgetId: 202602, scopeId: 202 } },
      ],
    },
  ];
};

const formatMockTreeSelectionLabel = (selected) => {
  const firstLabel = selected[0]?.path.map((item) => item.label).join(' / ') ?? '';
  const preview = Array.from(firstLabel).slice(0, 7).join('');
  return selected.length > 1 ? `${preview} 等${selected.length}个` : preview;
};

const formatMockTreeSelectionNodes = (selected) => {
  const nodeIds = new Set();
  return selected
    .flatMap(({ path }) => path)
    .filter((node) => {
      if (nodeIds.has(node.id)) return false;
      nodeIds.add(node.id);
      return true;
    })
    .map((node) => node.label)
    .join(',');
};

const quickInsertItems = [
  {
    id: 'analysis-theme-select',
    label: '/按分析主题选表',
    group: '分析主题选表',
    description: 'Mock：按预算和分析主题选择数据范围',
    kind: 'tree-select',
    picker: {
      multiple: true,
      leafOnly: true,
      loadOptions: loadMockAnalysisThemeOptions,
      toShortcut: (selected) => {
        return {
        id: `analysis-theme-${selected.map(({ node }) => `${node.payload.budgetId}-${node.payload.scopeId}`).join('_')}`,
        label: `主题：${formatMockTreeSelectionLabel(selected)}`,
        group: '分析主题选表',
        description: '已选择的 Mock 分析主题',
        data: {
          tag: 'user_select_analysis_theme',
          proto: {
           system_remider: '当用户当前选择了一些分析主题',
           selections: formatMockTreeSelectionNodes(selected)
          },
        },
      }
      },
      onConfirm: (selected) => console.info('[playground] tree-select confirmed', selected),
      onCancel: (selected) => console.info('[playground] tree-select cancelled', selected),
    },
  },
   {
    id: 'user-input-table-name',
    label: '/t快捷选表:',
    group: 'Skills',
    description: '输入AI能理解的库名表名，eg: default.yike_notice_action_hour',
    data: { tag: 'skill', proto: { system_remider: '当用户使用此命令表示想要快捷选表，后续会使用自然语言跟随表名' } },
  },
  {
    id: 'skills-create',
    label: '/创建技能',
    group: 'Skills',
    description: '你可以使用此技能来创建一个技能。',
    data: { tag: 'skill', proto: { system_remider: '当用户使用此命令时，调用createSkill来为他的要求创建skill。' } },
  },
  ...Array.from({ length: 10 }, (_, index) => ({
    id: `skills-${index}`,
    label: `/蓝鲸技能包-${index}`,
    group: 'Skills',
    description: `当用户提到“蓝鲸技能包-${index}”时使用。`,
    data: { tag: 'skill', proto: { path: `/a/b/c/蓝鲸技能包-${index}.md` } },
  })),
  ...Array.from({ length: 10 }, (_, index) => ({
    id: `commands-${index}`,
    label: `/命令${index}`,
    group: 'Commands',
    description: `当用户提到“${index}”时使用。`,
    data: { tag: 'command', proto: { path: `/a/b/c/${index}.md` } },
  })),
];

const parseRouteValues = (value) => value
  .split(',')
  .map((item) => item.trim())
  .filter(Boolean);

const parseJSONObject = (value, fieldName) => {
  const text = value.trim();
  if (!text) {
    return undefined;
  }

  const parsed = JSON.parse(text);
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error(`${fieldName} 必须是 JSON 对象`);
  }

  return parsed;
};

const resolveMountConfig = () => {
  const hostConfig = window.REACT_AGENT_HOST_CONFIG;
  if (hostConfig && hostConfig.callerKey) {
    return {
      callerKey: hostConfig.callerKey,
      routeValues: Array.isArray(hostConfig.routeValues) ? hostConfig.routeValues : [],
      controlContext: hostConfig.controlContext && typeof hostConfig.controlContext === 'object' && !Array.isArray(hostConfig.controlContext)
        ? hostConfig.controlContext
        : undefined,
      llmContext: hostConfig.llmContext && typeof hostConfig.llmContext === 'object' && !Array.isArray(hostConfig.llmContext)
        ? hostConfig.llmContext
        : undefined,
      editable: hostConfig.editable === true,
    };
  }

  return {
    callerKey: callerKeyInput.value.trim() || 'demo-app',
    routeValues: parseRouteValues(routeValuesInput.value),
    controlContext: parseJSONObject(controlContextInput.value, 'controlContext'),
    llmContext: parseJSONObject(llmContextInput.value, 'llmContext'),
    editable: true,
  };
};



const mountAgent = () => {
  const hostConfig = resolveMountConfig();

  if (currentHandle && typeof currentHandle.destroy === 'function') {
    disposeSQLClientTools?.();
    disposeSQLClientTools = null;
    currentHandle.destroy();
  }

  currentHandle = window.AgentWebSDK.mount(container, {
    baseUrl,
    callerKey: hostConfig.callerKey,
    routeValues: hostConfig.routeValues,
    controlContext: hostConfig.controlContext,
    llmContext: hostConfig.llmContext,
    modelKey: 'DeepSeek',
    modelVersion: 'deepseek-v4-pro',
    maxSteps: 25,
    theme: sdkTheme,
    title: 'ReAct Playground',
    showSidebar: true,
    placeholder: '输入 / 插入快捷节点，描述你的需求...',
    quickInsertItems,
    serializeInput: window.AgentWebSDK.serializeAgentInputParts,
  });

  disposeSQLClientTools = registerSQLClientTools(currentHandle.client);

  currentHandle?.ui?.setFeedback?.(feedbackByRunId);
  configBar.classList.toggle('rp-hidden', !hostConfig.editable);
  return management.reload();
};

const setInputAPIDemoState = (message, isError = false) => {
  inputAPIDemoState.textContent = message;
  inputAPIDemoState.classList.toggle('rp-input-api-state-error', isError);
};

const runInputAPIDemo = async (submit) => {
  if (!currentHandle || typeof currentHandle.fillInput !== 'function') {
    setInputAPIDemoState('当前 SDK 未提供 fillInput，请重新构建并刷新页面', true);
    return;
  }

  inputAPIFillButton.disabled = true;
  inputAPISubmitButton.disabled = true;
  setInputAPIDemoState(submit ? '正在调用 fillInput(..., { submit: true })...' : '正在预填输入框...');

  try {
    const result = await currentHandle.fillInput(inputAPIDemoValue.value, { submit });
    if (result.status === 'submitted') {
      setInputAPIDemoState('已通过 fillInput 立即发送');
      return;
    }
    if (result.status === 'filled') {
      setInputAPIDemoState(submit
        ? '当前 run 未结束，submit 已忽略，仅更新了输入框'
        : '已填充输入框，请在聊天框中确认后发送');
      return;
    }
    setInputAPIDemoState(`调用未执行：${result.reason}`, true);
  } catch (error) {
    setInputAPIDemoState(error?.message || 'fillInput 调用失败', true);
  } finally {
    inputAPIFillButton.disabled = false;
    inputAPISubmitButton.disabled = false;
  }
};

const applySDKTheme = (theme) => {
  sdkTheme = theme === 'dark' ? 'dark' : 'light';
  currentHandle?.setTheme?.(sdkTheme);
  const dark = sdkTheme === 'dark';
  const label = dark ? '切换到亮色主题' : '切换到暗色主题';
  toggleSDKThemeButton.setAttribute('aria-pressed', dark ? 'true' : 'false');
  toggleSDKThemeButton.setAttribute('aria-label', label);
  toggleSDKThemeButton.title = label;
  toggleSDKThemeButton.querySelector('[aria-hidden="true"]').textContent = dark ? '☀' : '☾';
};

const parseReplayScope = () => {
  let scope;
  try {
    scope = JSON.parse(replayScopeInput.value.trim());
  } catch {
    throw new Error('请输入合法的 JSON');
  }

  if (!scope || Array.isArray(scope) || typeof scope !== 'object') {
    throw new Error('会话定位信息必须是 JSON 对象');
  }

  const sessionId = typeof scope.sessionId === 'string' ? scope.sessionId.trim() : '';
  const callerKey = typeof scope.callerKey === 'string' ? scope.callerKey.trim() : '';
  if (!sessionId) throw new Error('sessionId 不能为空');
  if (!callerKey) throw new Error('callerKey 不能为空');
  if (!Array.isArray(scope.routeValues) || scope.routeValues.some((item) => typeof item !== 'string')) {
    throw new Error('routeValues 必须是字符串数组');
  }

  return { sessionId, callerKey, routeValues: scope.routeValues };
};

const buildReplayURL = (scope) => {
  const replayURL = new URL('/react-base-service/react/replay', location.origin);
  replayURL.searchParams.set('sessionId', scope.sessionId);
  replayURL.searchParams.set('callerKey', scope.callerKey);
  replayURL.searchParams.set('routeValues', JSON.stringify(scope.routeValues));
  replayURL.searchParams.set('theme', sdkTheme);
  return replayURL;
};

const closeReplayModal = () => {
  replayModalMask.classList.remove('rp-visible');
  replayModalState.textContent = '';
};

const openReplayModal = () => {
  replayModalState.textContent = '';
  replayModalMask.classList.add('rp-visible');
  replayScopeInput.focus();
};

const confirmReplay = () => {
  try {
    const replayURL = buildReplayURL(parseReplayScope());
    window.open(replayURL.href, '_blank', 'noopener,noreferrer');
    closeReplayModal();
  } catch (error) {
    replayModalState.textContent = error?.message || '无法打开重放页面';
    replayScopeInput.focus();
  }
};

const normalizeEnvelope = (payload) => {
  const code = payload.errno ?? payload.errNo ?? payload.code ?? 0;
  if (code !== 0) {
    throw new Error(payload.errmsg ?? payload.errMsg ?? payload.message ?? '请求失败');
  }
  return payload.data;
};

const post = async (path, body) => {
  const response = await fetch(`${managementBaseUrl}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(body),
  });
  if (!response.ok) throw new Error(`请求失败：${response.status}`);
  return normalizeEnvelope(await response.json());
};

const setOperationState = (element, message, isError = false) => {
  element.textContent = message || '';
  element.classList.toggle('rp-error', isError);
};

const closeCopyCallerModal = () => {
  copyCallerModalMask.classList.remove('rp-visible');
  setOperationState(copyCallerModalState, '');
};

const openCopyCallerModal = async () => {
  const sourceCallerKey = callerKeyInput.value.trim();
  if (!sourceCallerKey) {
    alert('请先填写源 callerKey');
    callerKeyInput.focus();
    return;
  }
  copySourceCallerInput.value = sourceCallerKey;
  copyTargetCallerInput.value = `${sourceCallerKey.slice(0, 27)}_copy`;
  copyTargetNameInput.value = `${sourceCallerKey} 副本`;
  copyTargetPlatformInput.value = '';
  copyTargetDescriptionInput.value = `从 ${sourceCallerKey} 复制`;
  setOperationState(copyCallerModalState, '');
  copyCallerModalMask.classList.add('rp-visible');
  copyTargetCallerInput.focus();
  copyTargetCallerInput.select();

  try {
    const callers = await post('/caller/list', {});
    const source = Array.isArray(callers) ? callers.find((item) => item.callerKey === sourceCallerKey) : null;
    if (source && copyCallerModalMask.classList.contains('rp-visible') && copySourceCallerInput.value === sourceCallerKey) {
      copyTargetNameInput.value = `${source.name || sourceCallerKey} 副本`;
      copyTargetPlatformInput.value = source.platform || '';
    }
  } catch {
    // 列表预填失败不阻塞复制，用户仍可手动填写目标信息。
  }
};

const confirmCopyCaller = async () => {
  const sourceCallerKey = copySourceCallerInput.value.trim();
  const targetCallerKey = copyTargetCallerInput.value.trim();
  const name = copyTargetNameInput.value.trim();
  const platform = copyTargetPlatformInput.value.trim();
  if (!targetCallerKey || !name || !platform) {
    setOperationState(copyCallerModalState, '目标 callerKey、名称和平台不能为空', true);
    return;
  }

  copyCallerModalConfirm.disabled = true;
  setOperationState(copyCallerModalState, '正在复制...');
  try {
    const result = await post('/caller/copy_config', {
      sourceCallerKey,
      targetCaller: {
        callerKey: targetCallerKey,
        name,
        description: copyTargetDescriptionInput.value,
        platform,
      },
    });
    closeCopyCallerModal();
    callerKeyInput.value = result.targetCallerKey;
    await remountAgent();
    const copied = result.copied || {};
    management.setState(`复制完成：Skill ${copied.skills || 0}，系统提示词 ${copied.systemPrompts || 0}，Tool ${copied.tools || 0}，Tool 策略 ${copied.toolUserPolicies || 0}，API Key ${copied.apiKeys || 0}`);
  } catch (error) {
    setOperationState(copyCallerModalState, error?.message || '复制失败', true);
  } finally {
    copyCallerModalConfirm.disabled = false;
  }
};

const closeDeleteCallerModal = () => {
  deleteCallerModalMask.classList.remove('rp-visible');
  deleteCallerConfirmInput.value = '';
  setOperationState(deleteCallerModalState, '');
};

const openDeleteCallerModal = () => {
  const callerKey = callerKeyInput.value.trim();
  if (!callerKey) {
    alert('请先填写要删除的 callerKey');
    callerKeyInput.focus();
    return;
  }
  deleteCallerConfirmLabel.textContent = `请输入 ${callerKey} 确认删除`;
  deleteCallerConfirmInput.value = '';
  setOperationState(deleteCallerModalState, '');
  deleteCallerModalMask.classList.add('rp-visible');
  deleteCallerConfirmInput.focus();
};

const clearDeletedCaller = () => {
  disposeSQLClientTools?.();
  disposeSQLClientTools = null;
  currentHandle?.destroy?.();
  currentHandle = null;
  callerKeyInput.value = '';
  routeValuesInput.value = '';
  container.innerHTML = '';
  management.items = [];
  management.renderTable();
  management.setState('Caller 已删除');
};

const confirmDeleteCaller = async () => {
  const callerKey = callerKeyInput.value.trim();
  if (deleteCallerConfirmInput.value.trim() !== callerKey) {
    setOperationState(deleteCallerModalState, `请输入完整的 ${callerKey}`, true);
    return;
  }

  deleteCallerModalConfirm.disabled = true;
  setOperationState(deleteCallerModalState, '正在删除...');
  try {
    await post('/caller/batch_delete', { callerKey });
    closeDeleteCallerModal();
    clearDeletedCaller();
  } catch (error) {
    setOperationState(deleteCallerModalState, error?.message || '删除失败', true);
  } finally {
    deleteCallerModalConfirm.disabled = false;
  }
};

const shortText = (value, max = 56) => {
  const text = typeof value === 'string' ? value : JSON.stringify(value ?? '');
  return text.length > max ? `${text.slice(0, max)}...` : text;
};

const escapeHtml = (value) => String(value ?? '')
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;');

const isEnabled = (item) => Number(item.status) === 1;
const isPlanEnabled = (item) => item.planEnabled === true || Number(item.planEnabled) === 1;

const getConfig = () => resolveMountConfig();

const resources = {
  caller: {
    title: 'Caller 管理',
    addText: '注册 Caller',
    idKey: 'callerKey',
    listPath: '/caller/list',
    createPath: '/caller/register',
    updatePath: '/caller/update',
    // 不提供 deletePath：Caller 删除连带其全部资源，统一走「更多能力」里的强确认入口。
    columns: [
      ['callerKey', 'callerKey'], ['name', '名称'], ['description', '描述'], ['platform', '平台'], ['createdAt', '创建时间'], ['planEnabled', 'Plan'], ['status', '状态'], ['actions', '操作'],
    ],
    empty: () => ({ callerKey: '', name: '', description: '', platform: '', planEnabled: false, status: 1 }),
    toDraft: (item) => ({ ...item }),
    toPayload: (draft) => ({
      callerKey: draft.callerKey,
      name: draft.name,
      description: draft.description,
      platform: draft.platform,
      planEnabled: isPlanEnabled(draft),
      status: Number(draft.status),
    }),
    fields: (mode) => mode === 'create'
      ? [['callerKey', 'callerKey'], ['name', '名称'], ['platform', '平台'], ['planEnabled', 'Plan', 'select'], ['description', '描述', 'textarea']]
      : [['callerKey', 'callerKey', 'readonly'], ['name', '名称'], ['platform', '平台'], ['planEnabled', 'Plan', 'select'], ['status', '状态', 'select'], ['description', '描述', 'textarea']],
  },
  tool: {
    title: '工具管理',
    addText: '新增工具',
    idKey: 'toolId',
    listPath: '/tool/list',
    createPath: '/tool/register',
    updatePath: '/tool/update',
    deletePath: '/tool/delete',
    deleteBody: (item) => ({ toolId: item.toolId }),
    columns: [
      ['toolId', '工具标识'], ['name', '工具名称'], ['description', '工具描述'], ['toolType', '工具类型'], ['config', '工具配置'], ['routeValues', '适用方'], ['status', '状态'], ['actions', '操作'],
    ],
    empty: () => ({ name: '', description: '', toolType: 'client', configText: '{}', status: 1 }),
    toDraft: (item) => ({ ...item, configText: JSON.stringify(item.config ?? {}, null, 2) }),
    toPayload: (draft, config) => ({
      toolId: draft.toolId,
      callerKey: config.callerKey,
      routeValues: config.routeValues,
      name: draft.name,
      description: draft.description,
      toolType: draft.toolType,
      config: JSON.parse(draft.configText || '{}'),
      status: Number(draft.status),
    }),
    fields: [
      ['name', '工具名称'], ['toolType', '工具类型'], ['status', '状态', 'select'], ['description', '工具描述', 'textarea'], ['configText', '工具配置 JSON', 'textarea'],
    ],
  },
  systemPrompt: {
    title: '系统提示词管理',
    addText: '新增提示词',
    idKey: 'id',
    listPath: '/system-prompt/list',
    createPath: '/system-prompt/register',
    updatePath: '/system-prompt/update',
    deletePath: '/system-prompt/delete',
    deleteBody: (item) => ({ id: item.id }),
    columns: [
      ['id', 'ID'], ['name', '名称'], ['content', '内容'], ['routeValues', '适用方'], ['status', '状态'], ['actions', '操作'],
    ],
    empty: () => ({ name: '', content: '', status: 1 }),
    toDraft: (item) => ({ ...item }),
    toPayload: (draft, config) => ({
      id: draft.id,
      callerKey: config.callerKey,
      routeValues: config.routeValues,
      name: draft.name,
      content: draft.content,
      status: Number(draft.status),
    }),
    fields: [
      ['name', '名称'], ['status', '状态', 'select'], ['content', '内容', 'textarea'],
    ],
  },
  skill: {
    title: 'skill 管理',
    addText: '新增 skill',
    idKey: 'skillId',
    listPath: '/skill/list',
    createPath: '/skill/create',
    updatePath: '/skill/update',
    deletePath: '/skill/delete',
    deleteBody: (item) => ({ skillId: item.skillId }),
    columns: [
      ['skillId', 'skill 标识'], ['name', '名称'], ['description', '描述'], ['triggerCondition', '触发条件'], ['isDefault', '默认'], ['routeValues', '适用方'], ['status', '状态'], ['actions', '操作'],
    ],
    empty: () => ({
      name: '', description: '', triggerCondition: '', forbiddenCondition: '', executionSteps: '', businessContext: '', promptSupplement: '', isDefault: 0, status: 1,
    }),
    toDraft: (item) => ({ ...item }),
    toPayload: (draft, config) => ({
      skillId: draft.skillId,
      callerKey: config.callerKey,
      routeValues: config.routeValues,
      name: draft.name,
      description: draft.description,
      triggerCondition: draft.triggerCondition,
      forbiddenCondition: draft.forbiddenCondition,
      executionSteps: draft.executionSteps,
      businessContext: draft.businessContext,
      promptSupplement: draft.promptSupplement,
      isDefault: Number(draft.isDefault),
      status: Number(draft.status),
    }),
    fields: [
      ['name', '名称'], ['status', '状态', 'select'], ['isDefault', '默认', 'defaultSelect'], ['description', '描述', 'textarea'], ['triggerCondition', '触发条件', 'textarea'], ['forbiddenCondition', '禁用条件', 'textarea'], ['executionSteps', '执行步骤', 'textarea'], ['businessContext', '业务上下文', 'textarea'], ['promptSupplement', '补充提示词', 'textarea'],
    ],
  },
  apiKey: {
    title: 'API Key 管理',
    addText: '新增 API Key',
    idKey: 'id',
    listPath: '/apikey/list',
    createPath: '/apikey/register',
    updatePath: '/apikey/update',
    deletePath: '/apikey/delete',
    deleteBody: (item) => ({ id: item.id }),
    columns: [
      ['id', 'ID'], ['name', '名称'], ['apiKeyDisplay', 'API Key'], ['routeValues', '适用方'], ['status', '状态'], ['actions', '操作'],
    ],
    empty: () => ({ name: '', apiKey: '', status: 1 }),
    toDraft: (item) => ({ ...item, apiKeyDisplay: item.apiKey || '******', apiKey: '' }),
    toPayload: (draft, config, mode) => {
      const payload = {
        id: draft.id,
        callerKey: config.callerKey,
        routeValues: config.routeValues,
        name: draft.name,
        status: Number(draft.status),
      };
      if (mode === 'create' || draft.apiKey) payload.apiKey = draft.apiKey;
      return payload;
    },
    fields: (mode) => mode === 'create'
      ? [['name', '名称'], ['apiKey', 'API Key', 'password']]
      : [['name', '名称'], ['status', '状态', 'select'], ['apiKey', '新 API Key', 'password']],
  },
  planTemplate: {
    title: '模板列表',
    tabText: '模板列表',
    itemName: '模板',
    addText: '新增模板',
    idKey: 'templateId',
    listPath: '/react/plan_template/list',
    detailPath: '/react/plan_template/detail',
    createPath: '/react/plan_template/create',
    updatePath: '/react/plan_template/update',
    columns: [
      ['templateId', '模板 ID'], ['description', '描述'], ['revision', 'Revision'], ['status', '状态'], ['updatedBy', '更新人'], ['updatedAt', '更新时间'], ['actions', '操作'],
    ],
    empty: () => ({
      templateId: '',
      status: 1,
      templateText: JSON.stringify({
        description: '请填写模板用途',
        inputs: {
          type: 'object',
          properties: {},
          additionalProperties: false,
        },
        steps: [{
          id: 'execute_task',
          order: 1,
          name: '执行任务',
          goal: '根据输入完成目标',
          depends_on: [],
          input: {},
          tool_names: ['*'],
          max_rounds: 20,
          timeout_seconds: 300,
          output_schema: {
            type: 'object',
            properties: { result: { type: 'string' } },
            required: ['result'],
            additionalProperties: false,
          },
        }],
        output: { result: '${steps.execute_task.result.result}' },
        output_schema: {
          type: 'object',
          properties: { result: { type: 'string' } },
          required: ['result'],
          additionalProperties: false,
        },
      }, null, 2),
    }),
    toDraft: (item) => ({ ...item }),
    loadDetail: async (item, config, resource) => {
      const detail = await post(resource.detailPath, {
        callerKey: config.callerKey,
        templateId: item.templateId,
      });
      return {
        ...detail,
        templateText: JSON.stringify(detail?.template ?? {}, null, 2),
      };
    },
    toPayload: (draft, config) => ({
      callerKey: config.callerKey,
      templateId: draft.templateId,
      revision: draft.revision,
      template: JSON.parse(draft.templateText || '{}'),
      status: Number(draft.status),
    }),
    fields: (mode) => mode === 'create'
      ? [['templateId', '模板 ID'], ['status', '状态', 'select'], ['templateText', '模板 JSON', 'codeTextarea']]
      : [['templateId', '模板 ID', 'readonly'], ['revision', 'Revision', 'readonly'], ['status', '状态', 'select'], ['templateText', '模板 JSON', 'codeTextarea']],
  },
};

const management = {
  type: 'tool',
  items: [],
  draft: null,
  mode: 'create',
  init() {
    $('management-tabs').innerHTML = Object.entries(resources).map(([key, resource]) => (
      `<button class="rp-button rp-management-tab" data-type="${key}" type="button">${resource.tabText ?? resource.title.replace('管理', '')}</button>`
    )).join('');
    $('management-tabs').addEventListener('click', (event) => {
      const button = event.target.closest('[data-type]');
      if (!button) return;
      this.type = button.dataset.type;
      this.renderShell();
      this.reload();
    });
    $('management-keyword').addEventListener('input', () => this.renderTable());
    $('management-status').addEventListener('change', () => this.renderTable());
    $('management-add').addEventListener('click', () => this.openCreate());
    $('management-refresh').addEventListener('click', () => this.reload());
    $('management-modal-close').addEventListener('click', () => this.closeModal());
    $('management-modal-cancel').addEventListener('click', () => this.closeModal());
    $('management-modal-save').addEventListener('click', () => this.save());
    $('management-body').addEventListener('click', (event) => this.handleTableClick(event));
    this.renderShell();
  },
  resource() { return resources[this.type]; },
  config() { return getConfig(); },
  setState(text, isError = false) {
    const el = $('management-state');
    el.textContent = text || '';
    el.classList.toggle('rp-hidden', !text);
    el.classList.toggle('rp-error', isError);
  },
  renderShell() {
    const resource = this.resource();
    $('management-title').textContent = resource.title;
    $('management-add').textContent = resource.addText;
    document.querySelectorAll('.rp-management-tab').forEach((tab) => tab.classList.toggle('rp-active', tab.dataset.type === this.type));
    $('management-head').innerHTML = `<tr class="rp-table-row">${resource.columns.map(([, label]) => `<th class="rp-table-cell rp-table-header-cell">${label}</th>`).join('')}</tr>`;
  },
  async reload() {
    const resource = this.resource();
    const config = this.config();
    this.setState('加载中...');
    try {
      const data = await post(resource.listPath, { callerKey: config.callerKey, routeValues: config.routeValues });
      this.items = Array.isArray(data) ? data.map(resource.toDraft) : [];
      this.setState('');
      this.renderTable();
    } catch (error) {
      this.items = [];
      this.setState(error.message || '加载失败', true);
      this.renderTable();
    }
  },
  filteredItems() {
    const keyword = $('management-keyword').value.trim().toLowerCase();
    const status = $('management-status').value;
    return this.items.filter((item) => {
      const text = JSON.stringify(item).toLowerCase();
      const statusMatched = status === 'all' || (status === 'enabled' ? isEnabled(item) : !isEnabled(item));
      return (!keyword || text.includes(keyword)) && statusMatched;
    });
  },
  renderValue(key, item) {
    if (key === 'actions') {
      const deleteButton = this.resource().deletePath
        ? '<button class="rp-button rp-link-btn rp-danger-link" data-action="delete" type="button">删除</button>'
        : '';
      return `<div class="rp-row-actions"><button class="rp-button rp-link-btn" data-action="edit" type="button">修改</button>${deleteButton}</div>`;
    }
    if (key === 'status') return `<button class="rp-button rp-switch ${isEnabled(item) ? 'rp-on' : ''}" data-action="toggle" type="button" title="${isEnabled(item) ? '启用' : '停用'}"></button>`;
    if (key === 'planEnabled') return `<button class="rp-button rp-switch ${isPlanEnabled(item) ? 'rp-on' : ''}" data-action="toggle-plan" type="button" title="Plan ${isPlanEnabled(item) ? '启用' : '停用'}"></button>`;
    if (key === 'routeValues') return escapeHtml(Array.isArray(item.routeValues) && item.routeValues.length ? item.routeValues.join(',') : '[]');
    if (key === 'config') return escapeHtml(shortText(item.configText ?? item.config, 64));
    return escapeHtml(shortText(item[key], 72));
  },
  renderTable() {
    const resource = this.resource();
    const rows = this.filteredItems();
    if (!rows.length) {
      $('management-body').innerHTML = `<tr class="rp-table-row"><td class="rp-table-cell rp-table-empty-cell" colspan="${resource.columns.length}">暂无数据</td></tr>`;
      return;
    }
    $('management-body').innerHTML = rows.map((item, index) => (
      `<tr class="rp-table-row" data-index="${index}">${resource.columns.map(([key]) => `<td class="rp-table-cell" title="${escapeHtml(typeof item[key] === 'object' ? JSON.stringify(item[key] ?? '') : item[key] ?? '')}">${this.renderValue(key, item)}</td>`).join('')}</tr>`
    )).join('');
    this.visibleItems = rows;
  },
  itemFromEvent(event) {
    const row = event.target.closest('tr[data-index]');
    if (!row) return null;
    return this.visibleItems[Number(row.dataset.index)];
  },
  handleTableClick(event) {
    const action = event.target.closest('[data-action]')?.dataset.action;
    if (!action) return;
    const item = this.itemFromEvent(event);
    if (!item) return;
    if (action === 'edit') this.openEdit(item);
    if (action === 'delete') this.remove(item);
    if (action === 'toggle') this.toggle(item);
    if (action === 'toggle-plan') this.togglePlan(item);
  },
  openCreate() {
    this.mode = 'create';
    this.draft = this.resource().empty();
    this.renderModal();
  },
  async openEdit(item) {
    const resourceType = this.type;
    const resource = this.resource();
    this.mode = 'edit';
    this.setState(resource.loadDetail ? '正在加载模板详情...' : '');
    try {
      const draft = resource.loadDetail
        ? await resource.loadDetail(item, this.config(), resource)
        : { ...item };
      if (this.type !== resourceType) return;
      this.draft = draft;
      this.setState('');
      this.renderModal();
    } catch (error) {
      if (this.type !== resourceType) return;
      this.draft = null;
      this.setState(error.message || '加载详情失败', true);
    }
  },
  closeModal() {
    this.draft = null;
    $('management-modal-mask').classList.remove('rp-visible');
  },
  renderModal() {
    const resource = this.resource();
    const fields = typeof resource.fields === 'function' ? resource.fields(this.mode) : resource.fields;
    $('management-modal-title').textContent = this.mode === 'create' ? resource.addText : `编辑${resource.itemName ?? resource.title.replace('管理', '')}`;
    $('management-modal-body').innerHTML = `<div class="rp-form-grid">${fields.map(([key, label, type]) => {
      const value = this.draft[key] ?? '';
      if (type === 'select') {
        return `<label class="rp-field"><span class="rp-field-label">${label}</span><select class="rp-control rp-control-size-default" data-field="${key}"><option value="1" ${Number(value) === 1 ? 'selected' : ''}>启用</option><option value="0" ${Number(value) === 0 ? 'selected' : ''}>停用</option></select></label>`;
      }
      if (type === 'defaultSelect') {
        return `<label class="rp-field"><span class="rp-field-label">${label}</span><select class="rp-control rp-control-size-default" data-field="${key}"><option value="0" ${Number(value) === 0 ? 'selected' : ''}>否</option><option value="1" ${Number(value) === 1 ? 'selected' : ''}>是</option></select></label>`;
      }
      if (type === 'textarea' || type === 'codeTextarea') {
        const codeClass = type === 'codeTextarea' ? ' rp-code-textarea' : '';
        const spellcheck = type === 'codeTextarea' ? ' spellcheck="false"' : '';
        return `<label class="rp-field rp-span-all"><span class="rp-field-label">${label}</span><textarea class="rp-control rp-textarea${codeClass}" data-field="${key}"${spellcheck}>${escapeHtml(value)}</textarea></label>`;
      }
      if (type === 'password') {
        const placeholder = this.mode === 'edit' ? '留空表示不修改' : '请输入 API Key';
        return `<label class="rp-field rp-span-all"><span class="rp-field-label">${label}</span><input type="password" class="rp-control rp-control-size-default" data-field="${key}" value="" placeholder="${placeholder}" autocomplete="new-password" /></label>`;
      }
      if (type === 'readonly') {
        return `<label class="rp-field"><span class="rp-field-label">${label}</span><input class="rp-control rp-control-size-default" value="${escapeHtml(value)}" readonly disabled /></label>`;
      }
      return `<label class="rp-field"><span class="rp-field-label">${label}</span><input class="rp-control rp-control-size-default" data-field="${key}" value="${escapeHtml(value)}" /></label>`;
    }).join('')}</div>`;
    $('management-modal-mask').classList.add('rp-visible');
  },
  syncDraftFromModal() {
    $('management-modal-body').querySelectorAll('[data-field]').forEach((field) => {
      this.draft[field.dataset.field] = field.value;
    });
  },
  async save() {
    const resource = this.resource();
    this.syncDraftFromModal();
    try {
      const config = this.config();
      const payload = resource.toPayload(this.draft, config, this.mode);
      const isEdit = this.mode === 'edit' && this.draft[resource.idKey];
      await post(isEdit ? resource.updatePath : resource.createPath, payload);
      this.closeModal();
      resource.afterSave?.();
      await this.reload();
    } catch (error) {
      this.setState(error.message || '保存失败', true);
    }
  },
  async remove(item) {
    if (!this.resource().deletePath) return;
    if (!confirm('确认删除吗？')) return;
    try {
      await post(this.resource().deletePath, this.resource().deleteBody(item));
      await this.reload();
    } catch (error) {
      this.setState(error.message || '删除失败', true);
    }
  },
  async toggle(item) {
    const resource = this.resource();
    try {
      const draft = resource.loadDetail
        ? await resource.loadDetail(item, this.config(), resource)
        : { ...item };
      const payload = resource.toPayload({ ...draft, status: isEnabled(item) ? 0 : 1 }, this.config());
      await post(resource.updatePath, payload);
      await this.reload();
    } catch (error) {
      this.setState(error.message || '切换失败', true);
    }
  },
  async togglePlan(item) {
    const resource = this.resource();
    try {
      const payload = resource.toPayload({ ...item, planEnabled: !isPlanEnabled(item) }, this.config());
      await post(resource.updatePath, payload);
      await this.reload();
    } catch (error) {
      this.setState(error.message || 'Plan 开关切换失败', true);
    }
  },
};

const initialConfig = window.REACT_AGENT_HOST_CONFIG;
if (initialConfig && initialConfig.callerKey) {
  callerKeyInput.value = initialConfig.callerKey;
  routeValuesInput.value = Array.isArray(initialConfig.routeValues)
    ? initialConfig.routeValues.join(',')
    : '';
  controlContextInput.value = initialConfig.controlContext && typeof initialConfig.controlContext === 'object' && !Array.isArray(initialConfig.controlContext)
    ? JSON.stringify(initialConfig.controlContext, null, 2)
    : '';
  llmContextInput.value = initialConfig.llmContext && typeof initialConfig.llmContext === 'object' && !Array.isArray(initialConfig.llmContext)
    ? JSON.stringify(initialConfig.llmContext, null, 2)
    : '';
}

const hasContextConfigValue = () => !!(controlContextInput.value.trim() || llmContextInput.value.trim());

const setMoreCapabilitiesExpanded = (expanded) => {
  moreCapabilitiesPanel.classList.toggle('rp-hidden', !expanded);
  toggleMoreCapabilitiesButton.setAttribute('aria-expanded', expanded ? 'true' : 'false');
  toggleMoreCapabilitiesButton.textContent = expanded ? '收起更多能力 ▴' : '更多能力 ▾';
};

setMoreCapabilitiesExpanded(hasContextConfigValue());

toggleMoreCapabilitiesButton.addEventListener('click', () => {
  setMoreCapabilitiesExpanded(moreCapabilitiesPanel.classList.contains('rp-hidden'));
});

toggleSDKThemeButton.addEventListener('click', () => {
  applySDKTheme(sdkTheme === 'dark' ? 'light' : 'dark');
});

applySDKTheme(sdkTheme);

const remountAgent = async () => {
  try {
    await mountAgent();
  } catch (error) {
    const message = error?.message || '配置解析失败';
    alert(message);
  }
};

applyButton.addEventListener('click', remountAgent);
copyCallerButton.addEventListener('click', openCopyCallerModal);
deleteCallerButton.addEventListener('click', openDeleteCallerModal);
copyCallerModalClose.addEventListener('click', closeCopyCallerModal);
copyCallerModalCancel.addEventListener('click', closeCopyCallerModal);
copyCallerModalConfirm.addEventListener('click', confirmCopyCaller);
copyCallerModalMask.addEventListener('click', (event) => {
  if (event.target === copyCallerModalMask) closeCopyCallerModal();
});
copyTargetDescriptionInput.addEventListener('keydown', (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') confirmCopyCaller();
});
deleteCallerModalClose.addEventListener('click', closeDeleteCallerModal);
deleteCallerModalCancel.addEventListener('click', closeDeleteCallerModal);
deleteCallerModalConfirm.addEventListener('click', confirmDeleteCaller);
deleteCallerModalMask.addEventListener('click', (event) => {
  if (event.target === deleteCallerModalMask) closeDeleteCallerModal();
});
deleteCallerConfirmInput.addEventListener('keydown', (event) => {
  if (event.key === 'Enter') confirmDeleteCaller();
});
openReplayLink.addEventListener('click', openReplayModal);
replayModalClose.addEventListener('click', closeReplayModal);
replayModalCancel.addEventListener('click', closeReplayModal);
replayModalConfirm.addEventListener('click', confirmReplay);
replayModalMask.addEventListener('click', (event) => {
  if (event.target === replayModalMask) closeReplayModal();
});
replayScopeInput.addEventListener('keydown', (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
    event.preventDefault();
    confirmReplay();
  }
});
replayScopeInput.addEventListener('input', () => {
  replayModalState.textContent = '';
});
inputAPIFillButton.addEventListener('click', () => runInputAPIDemo(false));
inputAPISubmitButton.addEventListener('click', () => runInputAPIDemo(true));
inputAPIDemoValue.addEventListener('keydown', (event) => {
  if (event.key !== 'Enter') return;
  event.preventDefault();
  runInputAPIDemo(event.metaKey || event.ctrlKey);
});
[routeValuesInput, callerKeyInput].forEach((input) => {
  input.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') remountAgent();
  });
});

management.init();
remountAgent();
