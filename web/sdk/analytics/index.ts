/**
 * SDK 内部埋点（仅对外导出 track）
 *
 * 本地无外部埋点平台依赖：track 为空操作，失败静默，不影响 SDK 主流程。
 */

/** 主动上报（当前为空操作，保留接口以便接入方替换） */
export function track(event: string, params?: Record<string, unknown>): void {
  if (typeof window === 'undefined') return;
  try {
    // no-op：未接入埋点平台
    void event;
    void params;
  } catch {
    // 埋点失败不影响 SDK 主流程。
  }
}
