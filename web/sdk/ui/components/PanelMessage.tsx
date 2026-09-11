import { Show } from "solid-js";

export interface PanelMessageProps {
  message?: string;
}

export function PanelMessage(props: PanelMessageProps) {
  return (
    <Show when={props.message?.trim()}>
      {(message) => (
        <div class="agent-ui-panel-message" role="alert">
          {message()}
        </div>
      )}
    </Show>
  );
}

export default PanelMessage;
