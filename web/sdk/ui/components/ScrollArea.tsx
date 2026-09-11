import type { OverlayScrollbars, PartialOptions } from 'overlayscrollbars';
import {
  OverlayScrollbarsComponent,
  type OverlayScrollbarsComponentRef,
} from 'overlayscrollbars-solid';
import 'overlayscrollbars/overlayscrollbars.css';
import { onCleanup, onMount, type JSX } from 'solid-js';

export type ScrollAreaSize = 'sm' | 'md' | 'lg';

export interface ScrollAreaHandle {
  getRootElement: () => HTMLDivElement | undefined;
  getViewportElement: () => HTMLDivElement | undefined;
  getInstance: () => OverlayScrollbars | undefined;
}

export interface ScrollAreaProps {
  children: JSX.Element;
  ref?: (handle: ScrollAreaHandle) => void;
  size?: ScrollAreaSize;
  class?: string;
  contentClass?: string;
  viewportRef?: (element: HTMLDivElement | undefined) => void;
  contentRef?: (element: HTMLDivElement | undefined) => void;
  onScroll?: (event: Event) => void;
}

const DEFAULT_OPTIONS: PartialOptions = {
  overflow: {
    x: 'hidden',
    y: 'scroll',
  },
  scrollbars: {
    theme: 'os-theme-agent-ui',
    visibility: 'auto',
    autoHide: 'leave',
    autoHideDelay: 120,
    dragScroll: true,
    clickScroll: true,
  },
};

export function ScrollArea(props: ScrollAreaProps) {
  let rootRef: OverlayScrollbarsComponentRef<'div'> | undefined;
  let viewportElement: HTMLDivElement | undefined;
  let contentElement: HTMLDivElement | undefined;
  let frameId: number | undefined;

  const size = () => props.size ?? 'md';

  const syncViewport = () => {
    const nextViewport = rootRef?.osInstance()?.elements().viewport as HTMLDivElement | undefined;
    if (nextViewport === viewportElement) return;

    viewportElement = nextViewport;
    props.viewportRef?.(viewportElement);
  };

  const scheduleSyncViewport = () => {
    if (frameId) {
      cancelAnimationFrame(frameId);
    }

    frameId = requestAnimationFrame(() => {
      frameId = undefined;
      syncViewport();
    });
  };

  const handle: ScrollAreaHandle = {
    getRootElement: () => (rootRef?.getElement() ?? undefined) as HTMLDivElement | undefined,
    getViewportElement: () => {
      syncViewport();
      return viewportElement;
    },
    getInstance: () => rootRef?.osInstance() ?? undefined,
  };

  const events = {
    initialized: () => {
      syncViewport();
    },
    updated: () => {
      syncViewport();
    },
    scroll: (_instance: OverlayScrollbars, event: Event) => {
      props.onScroll?.(event);
    },
    destroyed: () => {
      viewportElement = undefined;
      props.viewportRef?.(undefined);
    },
  };

  onMount(() => {
    props.ref?.(handle);
    scheduleSyncViewport();
  });

  onCleanup(() => {
    if (frameId) {
      cancelAnimationFrame(frameId);
    }
    props.viewportRef?.(undefined);
    props.contentRef?.(undefined);
  });

  return (
    <OverlayScrollbarsComponent
      ref={(ref) => {
        rootRef = ref;
        scheduleSyncViewport();
      }}
      class={[
        'agent-ui-scroll-area',
        `agent-ui-scroll-area-size-${size()}`,
        props.class,
      ].filter(Boolean).join(' ')}
      options={DEFAULT_OPTIONS}
      events={events}
      defer
    >
      <div
        ref={(element) => {
          contentElement = element;
          props.contentRef?.(contentElement);
        }}
        class={props.contentClass}
      >
        {props.children}
      </div>
    </OverlayScrollbarsComponent>
  );
}