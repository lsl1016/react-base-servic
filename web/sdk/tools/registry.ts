/**
 * 客户端工具注册表
 *
 * 管理所有客户端工具的注册和查找。
 * 按工具名（name）和别名（aliases）索引，支持单个/批量注册。
 *
 * AgentClient 内部持有此注册表，当收到 client_tool_use_start 事件时，
 * ClientToolExecutor 通过服务端下发的可执行标识查找对应工具并执行。
 */
import type { ClientTool } from './types';

export class ClientToolRegistry {
  private tools = new Map<string, ClientTool>();
  private registeredTools = new Set<ClientTool>();

  register(tool: ClientTool): this {
    this.registeredTools.add(tool);
    this.tools.set(tool.name, tool);
    for (const alias of tool.aliases ?? []) {
      this.tools.set(alias, tool);
    }
    return this;
  }

  registerMany(tools: ClientTool[]): this {
    for (const tool of tools) {
      this.register(tool);
    }
    return this;
  }

  get(name: string): ClientTool | undefined {
    return this.tools.get(name);
  }

  getFirst(names: Array<string | undefined>): ClientTool | undefined {
    for (const name of names) {
      if (!name) continue;
      const tool = this.tools.get(name);
      if (tool) return tool;
    }
    return undefined;
  }

  has(name: string): boolean {
    return this.tools.has(name);
  }

  getAll(): ClientTool[] {
    return [...this.registeredTools.values()];
  }

  get size(): number {
    return this.registeredTools.size;
  }
}