import type { AgentInputPart } from './editor/types';

export type AgentInputValue = string | AgentInputPart[];

export interface FillInputOptions {
  /** false 时仅替换输入草稿，true 时立即发送；默认 false。 */
  submit?: boolean;
}

export type FillInputResult =
  | { status: 'filled' }
  | { status: 'submitted' }
  | {
      status: 'rejected';
      reason: 'empty' | 'disconnected' | 'unmounted';
    };

export interface AgentInputCommand {
  fillInput(input: AgentInputValue, options?: FillInputOptions): Promise<FillInputResult>;
}