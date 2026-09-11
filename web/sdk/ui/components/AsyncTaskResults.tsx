import { For, Show } from "solid-js";
import IconMdiAlertCircleOutline from "~icons/mdi/alert-circle-outline";
import IconMdiBellOutline from "~icons/mdi/bell-outline";
import IconMdiCheckCircleOutline from "~icons/mdi/check-circle-outline";
import IconMdiClose from "~icons/mdi/close";
import type { AsyncTaskItem } from "../../session/types";

export interface AsyncTaskNotice {
  status: "succeeded" | "failed";
  title: string;
  detail?: string;
}

export interface AsyncTaskResultsProps {
  tasks: readonly AsyncTaskItem[];
  open: boolean;
  notice?: AsyncTaskNotice;
  onToggle: () => void;
  onClose: () => void;
  onDismissNotice: () => void;
  onLocate?: (toolUseId: string) => void;
}

function formatCompletedAt(task: AsyncTaskItem): string {
  const value = task.completedAt || task.updatedAt;
  if (!value) return "";
  return value.length >= 16 ? value.slice(5, 16) : value;
}

function taskTitle(task: AsyncTaskItem): string {
  return task.toolName?.trim() || "异步任务";
}

export function AsyncTaskResults(props: AsyncTaskResultsProps) {
  return (
    <>
      <Show when={props.tasks.length > 0}>
        <button
          class="agent-ui-header-icon-btn agent-ui-async-task-trigger"
          classList={{ "agent-ui-header-icon-btn-active": props.open }}
          title={`任务结果（${props.tasks.length}）`}
          aria-label={`任务结果，共 ${props.tasks.length} 条`}
          aria-expanded={props.open}
          onClick={props.onToggle}
        >
          <IconMdiBellOutline width="18" height="18" />
          <span class="agent-ui-async-task-badge">{props.tasks.length > 99 ? "99+" : props.tasks.length}</span>
        </button>
      </Show>

      <Show when={props.open && props.tasks.length > 0}>
        <div class="agent-ui-async-task-overlay" onClick={props.onClose}>
          <section
            class="agent-ui-async-task-popover"
            role="dialog"
            aria-label="任务结果"
            onClick={(event) => event.stopPropagation()}
          >
            <header class="agent-ui-async-task-popover-header">
              <div>
                <div class="agent-ui-async-task-popover-title">任务结果</div>
                <div class="agent-ui-async-task-popover-subtitle">当前会话共 {props.tasks.length} 条</div>
              </div>
              <button class="agent-ui-header-icon-btn" title="关闭任务结果" onClick={props.onClose}>
                <IconMdiClose width="17" height="17" />
              </button>
            </header>

            <div class="agent-ui-async-task-list">
              <For each={props.tasks}>
                {(task) => {
                  const failed = () => task.executionStatus === "failed";
                  return (
                    <article
                      class="agent-ui-async-task-item"
                      classList={{ "agent-ui-async-task-item-failed": failed() }}
                      data-status={task.executionStatus}
                    >
                      <div class="agent-ui-async-task-status-icon" aria-hidden="true">
                        <Show
                          when={!failed()}
                          fallback={<IconMdiAlertCircleOutline width="19" height="19" />}
                        >
                          <IconMdiCheckCircleOutline width="19" height="19" />
                        </Show>
                      </div>
                      <div class="agent-ui-async-task-item-content">
                        <div class="agent-ui-async-task-item-title-row">
                          <span class="agent-ui-async-task-item-title">{taskTitle(task)}</span>
                          <Show when={task.scope === "plan_step"}>
                            <span class="agent-ui-async-task-scope-tag">计划任务</span>
                          </Show>
                        </div>
                        <div class="agent-ui-async-task-item-meta">
                          <span>{failed() ? "执行失败" : "已完成"}</span>
                          <Show when={formatCompletedAt(task)}>
                            <span aria-hidden="true">·</span>
                            <time>{formatCompletedAt(task)}</time>
                          </Show>
                        </div>
                        <Show when={failed() && task.errorMessage}>
                          <div class="agent-ui-async-task-error" title={task.errorMessage}>
                            {task.errorMessage}
                          </div>
                        </Show>
                        <Show when={props.onLocate}>
                          <button
                            class="agent-ui-async-task-locate"
                            onClick={() => props.onLocate?.(task.toolUseId)}
                          >
                            定位到对话
                          </button>
                        </Show>
                      </div>
                    </article>
                  );
                }}
              </For>
            </div>
          </section>
        </div>
      </Show>

      <Show when={props.notice}>
        {(notice) => (
          <div class="agent-ui-async-task-toast" data-status={notice().status} role="status">
            <div class="agent-ui-async-task-toast-icon" aria-hidden="true">
              <Show
                when={notice().status === "succeeded"}
                fallback={<IconMdiAlertCircleOutline width="19" height="19" />}
              >
                <IconMdiCheckCircleOutline width="19" height="19" />
              </Show>
            </div>
            <div class="agent-ui-async-task-toast-content">
              <div class="agent-ui-async-task-toast-title">{notice().title}</div>
              <Show when={notice().detail}>
                <div class="agent-ui-async-task-toast-detail">{notice().detail}</div>
              </Show>
            </div>
            <button class="agent-ui-async-task-toast-close" title="关闭通知" onClick={props.onDismissNotice}>
              <IconMdiClose width="16" height="16" />
            </button>
          </div>
        )}
      </Show>
    </>
  );
}
