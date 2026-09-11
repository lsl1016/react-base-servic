import { describe, expect, it } from 'vitest';
import { parseAgentInputText, serializeAgentInputParts, type AgentInputPart, type AgentQuickInsertItem } from '../ui/editor/types';

const quickInsertItems: AgentQuickInsertItem[] = [
  {
    id: 'field-gmv',
    label: '/gmv',
    group: 'Fields',
    description: 'GMV 字段',
    data: {
      tag: 'field',
      proto: { id: 'gmv' },
    },
  },
  {
    id: 'field-city',
    label: '/city',
    group: 'Fields',
    description: '城市字段',
    data: {
      tag: 'field',
      proto: { id: 'city' },
    },
  },
];

const shortcutParts: Extract<AgentInputPart, { type: 'shortcut' }>[] = quickInsertItems.map((item) => ({
  type: 'shortcut',
  ...item,
}));

describe('editor input serialization', () => {
  it('serializes text and shortcut parts as protocol tags', () => {
    const parts: AgentInputPart[] = [
      { type: 'text', text: '帮我分析 ' },
      shortcutParts[0],
      { type: 'text', text: ' 的字段选择逻辑' },
    ];

    expect(serializeAgentInputParts(parts)).toBe('帮我分析 <field id="gmv">/gmv</field> 的字段选择逻辑');
  });

  it('serializes proto values as tag attributes', () => {
    const parts: AgentInputPart[] = [
      { type: 'text', text: '打开 ' },
      {
        type: 'shortcut',
        id: 'skill-canvas',
        label: '/canvas',
        group: 'Skills',
        description: 'A Cursor Canvas is a live React app',
        data: {
          tag: 'skill',
          proto: { path: '/a/b/c/canvas.md', enabled: true, order: 1 },
        },
      },
    ];

    expect(serializeAgentInputParts(parts)).toBe('打开 <skill enabled="true" order="1" path="/a/b/c/canvas.md">/canvas</skill>');
  });

  it('parses serialized text back into shortcut parts', () => {
    expect(parseAgentInputText('分析 <field id="gmv">/gmv</field> 和 <field id="city">/city</field>', quickInsertItems)).toEqual([
      { type: 'text', text: '分析 ' },
      shortcutParts[0],
      { type: 'text', text: ' 和 ' },
      shortcutParts[1],
    ]);
  });

  it('restores dynamic shortcuts that are not in the initial quick item list', () => {
    const serialized = '<analysis_theme budget_id="1" scope_id="2">预算一 / 主题二</analysis_theme>';
    const parts = parseAgentInputText(serialized);

    expect(parts).toEqual([{
      type: 'shortcut',
      id: 'serialized:analysis_theme?budget_id=1&scope_id=2',
      label: '预算一 / 主题二',
      group: 'analysis_theme',
      description: '预算一 / 主题二',
      data: {
        tag: 'analysis_theme',
        proto: { budget_id: '1', scope_id: '2' },
      },
    }]);
    expect(serializeAgentInputParts(parts)).toBe(serialized);
  });
});
