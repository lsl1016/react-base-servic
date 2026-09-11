import type { ReactEvent } from '../protocol/types';
import type { AgentStatus } from '../runtime/types';

export interface WsRuntimeContext {
  runId: string | null;
  runStatus: AgentStatus;
}

/**
 * WsClient 配置项
 */
export interface WsClientOptions {
  /** 是否在断开后自动重连，默认 true */
  reconnect?: boolean;
  /** 心跳超时（ms），超过此时间未收到任何消息则主动关闭触发重连，默认 30000 */
  heartbeatTimeout?: number;
  /** 重连最大间隔（ms），指数退避的上限，默认 30000 */
  reconnectMaxDelay?: number;
  /** WebSocket 建立连接的最大等待时间（ms），超时后主动关闭并触发重连，默认 10000 */
  connectTimeout?: number;
  /** 断网宽限（ms）：offline 事件后保留连接等待网络恢复，超时才拆连接触发重连；0 表示不宽限立刻拆，默认 5000 */
  offlineGraceMs?: number;
}

/**
 * WsClient 事件回调
 */
export interface WsClientCallbacks {
  /** WebSocket 连接建立时触发 */
  onOpen?: () => void;
  /** WebSocket 连接关闭时触发（含关闭原因） */
  onClose?: (event: CloseEvent) => void;
  /** WebSocket 出错时触发 */
  onError?: (event: Event) => void;
  /** 收到服务端 ReactEvent 时触发（已 JSON 解析） */
  onEvent?: (event: ReactEvent) => void;
  /** 连接状态变化时触发（connected=true 为已连接，false 为断开） */
  onConnectionChange?: (connected: boolean) => void;
  /** 自动重连排队时触发，attempt 从 1 开始 */
  onReconnectAttempt?: (attempt: number) => void;
  /** 获取当前 Agent 运行上下文，供连接日志打印和上报 */
  getRuntimeContext?: () => WsRuntimeContext;
}
