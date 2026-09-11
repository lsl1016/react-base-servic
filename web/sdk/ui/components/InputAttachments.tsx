import { For, Show } from "solid-js";
import IconIconoirAttachment from "~icons/iconoir/attachment";
import IconMdiClose from "~icons/mdi/close";
import type { ChatFileUpload } from "../../protocol/types";

export interface InputAttachment {
  id: number;
  fileName: string;
  size: number;
  status: "uploading" | "uploaded" | "error";
  uploaded?: ChatFileUpload;
  error?: string;
}

export interface InputAttachmentsProps {
  attachments: InputAttachment[];
  disabled?: boolean;
  onRemove: (id: number) => void;
}

function formatFileSize(size: number): string {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${Math.max(1, Math.round(size / 1024))} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

export function InputAttachments(props: InputAttachmentsProps) {
  return (
    <Show when={props.attachments.length > 0}>
      <div class="agent-ui-input-attachments" aria-label="已选择的附件">
        <For each={props.attachments}>
          {(attachment) => (
            <div class="agent-ui-input-attachment" classList={{ "agent-ui-input-attachment-error": attachment.status === "error" }}>
              <span class="agent-ui-input-attachment-icon"><IconIconoirAttachment width="17" height="17" /></span>
              <span class="agent-ui-input-attachment-info">
                <span class="agent-ui-input-attachment-name" title={attachment.fileName}>{attachment.fileName}</span>
                <span class="agent-ui-input-attachment-meta" title={attachment.error}>
                  {attachment.status === "uploading"
                    ? "上传中..."
                    : attachment.status === "error"
                      ? attachment.error || "上传失败"
                      : formatFileSize(attachment.size)}
                </span>
              </span>
              <button
                type="button"
                class="agent-ui-input-attachment-remove"
                onClick={() => props.onRemove(attachment.id)}
                disabled={props.disabled}
                title="删除附件"
                aria-label={`删除附件 ${attachment.fileName}`}
              >
                <IconMdiClose width="15" height="15" />
              </button>
            </div>
          )}
        </For>
      </div>
    </Show>
  );
}

export default InputAttachments;
