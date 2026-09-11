import * as sdk from './index';
import type { AgentClientConfig } from './runtime/agent-client';
import type { MountAgentUIOptions } from './ui/mount';

export * from './index';

export interface BrowserMountOptions extends AgentClientConfig, MountAgentUIOptions {}

export function mount(container: HTMLElement, options: BrowserMountOptions) {
  const client = sdk.createAgentClient(options);
  const ui = sdk.mountAgentUI(container, client, options);
  let unmounted = false;
  let destroyed = false;

  client.connect();

  return {
    client,
    ui,
    setTheme: ui.setTheme,
    getTheme: ui.getTheme,
    fillInput: ui.fillInput,
    unmount() {
      if (unmounted) return;
      ui.unmount();
      unmounted = true;
    },
    destroy() {
      if (!unmounted) {
        ui.unmount();
        unmounted = true;
      }
      if (destroyed) return;
      client.disconnect();
      destroyed = true;
    },
  };
}

export const createClient = sdk.createAgentClient;

const api = {
  ...sdk,
  mount,
  createClient,
};

if (typeof window !== 'undefined') {
  const globalWindow = window as Window & { AgentWebSDK?: Record<string, unknown> };
  globalWindow.AgentWebSDK = {
    ...(globalWindow.AgentWebSDK ?? {}),
    ...api,
  };
}
