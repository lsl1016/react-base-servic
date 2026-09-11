import { createEffect, createSignal, onCleanup, onMount, type JSX } from 'solid-js';
import { ScrollArea } from './ScrollArea';

export interface SmartScrollState {
  scrollTop: number;
  scrollHeight: number;
  clientHeight: number;
  distanceToTop: number;
  distanceToBottom: number;
  isAtTop: boolean;
  isAtBottom: boolean;
  isFollowing: boolean;
  isUserScrolling: boolean;
}

export interface SmartScrollHandle {
  getElement: () => HTMLDivElement | undefined;
  getState: () => SmartScrollState;
  scrollToBottom: (options?: ScrollToOptions) => void;
  scrollToTop: (options?: ScrollToOptions) => void;
  scrollTo: (top: number, options?: ScrollToOptions) => void;
  pauseFollow: () => void;
  resumeFollow: (options?: { scroll?: boolean }) => void;
}

export interface SmartScrollProps {
  children: JSX.Element;
  ref?: (handle: SmartScrollHandle) => void;
  class?: string;
  classList?: Record<string, boolean>;
  style?: JSX.CSSProperties;
  follow?: false | 'bottom';
  followKey?: unknown;
  active?: boolean;
  bottomThreshold?: number;
  reachThreshold?: number;
  onReachTop?: () => void;
  onReachBottom?: () => void;
  onStickChange?: (stuck: boolean) => void;
  onScrollStateChange?: (state: SmartScrollState) => void;
}

const EMPTY_STATE: SmartScrollState = {
  scrollTop: 0,
  scrollHeight: 0,
  clientHeight: 0,
  distanceToTop: 0,
  distanceToBottom: 0,
  isAtTop: true,
  isAtBottom: true,
  isFollowing: true,
  isUserScrolling: false,
};

function toScrollBehavior(options?: ScrollToOptions): ScrollBehavior | undefined {
  return options?.behavior;
}

export function SmartScroll(props: SmartScrollProps) {
  let containerRef: HTMLDivElement | undefined;
  let contentRef: HTMLDivElement | undefined;
  let frameId: number | undefined;
  let userScrollTimer: ReturnType<typeof setTimeout> | undefined;
  let resizeObserver: ResizeObserver | undefined;
  let isFollowing = props.follow === 'bottom';
  let isUserScrolling = false;
  let lastStickValue: boolean | undefined;
  let lastReachTop = false;
  let lastReachBottom = false;
  let isMounted = false;

  const [state, setState] = createSignal<SmartScrollState>({
    ...EMPTY_STATE,
    isFollowing,
  });

  const bottomThreshold = () => props.bottomThreshold ?? 80;
  const reachThreshold = () => props.reachThreshold ?? 80;
  const canFollow = () => props.follow === 'bottom';

  const readState = (): SmartScrollState => {
    const element = containerRef;
    if (!element) {
      return {
        ...EMPTY_STATE,
        isFollowing,
        isUserScrolling,
      };
    }

    const scrollTop = element.scrollTop;
    const scrollHeight = element.scrollHeight;
    const clientHeight = element.clientHeight;
    const distanceToBottom = Math.max(0, scrollHeight - scrollTop - clientHeight);
    const distanceToTop = Math.max(0, scrollTop);
    const isAtBottom = distanceToBottom <= bottomThreshold();
    const isAtTop = distanceToTop <= reachThreshold();

    return {
      scrollTop,
      scrollHeight,
      clientHeight,
      distanceToTop,
      distanceToBottom,
      isAtTop,
      isAtBottom,
      isFollowing: canFollow() && isFollowing,
      isUserScrolling,
    };
  };

  const emitState = () => {
    const nextState = readState();
    setState(nextState);
    props.onScrollStateChange?.(nextState);

    if (lastStickValue !== nextState.isFollowing) {
      lastStickValue = nextState.isFollowing;
      props.onStickChange?.(nextState.isFollowing);
    }

    const topReached = nextState.distanceToTop <= reachThreshold();
    if (topReached && !lastReachTop) {
      props.onReachTop?.();
    }
    lastReachTop = topReached;

    const bottomReached = nextState.distanceToBottom <= reachThreshold();
    if (bottomReached && !lastReachBottom) {
      props.onReachBottom?.();
    }
    lastReachBottom = bottomReached;
  };

  const schedule = (task: () => void) => {
    if (frameId) {
      cancelAnimationFrame(frameId);
    }

    frameId = requestAnimationFrame(() => {
      frameId = undefined;
      task();
    });
  };

  const scheduleMeasure = () => {
    schedule(() => emitState());
  };

  const resetResizeObserver = () => {
    resizeObserver?.disconnect();
    resizeObserver = undefined;

    if (!isMounted || typeof ResizeObserver === 'undefined') return;

    resizeObserver = new ResizeObserver(() => {
      followBottom();
    });

    if (contentRef) {
      resizeObserver.observe(contentRef);
    }
    if (containerRef) {
      resizeObserver.observe(containerRef);
    }
  };

  const handleViewportRef = (element: HTMLDivElement | undefined) => {
    containerRef = element;
    resetResizeObserver();
    followBottom();
  };

  const handleContentRef = (element: HTMLDivElement | undefined) => {
    contentRef = element;
    resetResizeObserver();
    followBottom();
  };

  const scrollToPosition = (top: number, options?: ScrollToOptions) => {
    const element = containerRef;
    if (!element) return;

    const behavior = toScrollBehavior(options);
    if (behavior) {
      element.scrollTo({ ...options, top });
    } else {
      element.scrollTop = top;
    }
    scheduleMeasure();
  };

  const scrollToBottom = (options?: ScrollToOptions) => {
    const element = containerRef;
    if (!element) return;

    scrollToPosition(element.scrollHeight, options);
  };

  const followBottom = () => {
    if (!canFollow() || !isFollowing) {
      scheduleMeasure();
      return;
    }

    schedule(() => scrollToBottom());
  };

  const pauseFollow = () => {
    isFollowing = false;
    scheduleMeasure();
  };

  const resumeFollow = (options?: { scroll?: boolean }) => {
    isFollowing = canFollow();
    if (options?.scroll ?? true) {
      scrollToBottom();
      return;
    }
    scheduleMeasure();
  };

  const handleScroll = () => {
    const current = readState();
    if (canFollow()) {
      isFollowing = current.distanceToBottom <= bottomThreshold();
    }

    isUserScrolling = true;
    if (userScrollTimer) {
      clearTimeout(userScrollTimer);
    }
    userScrollTimer = setTimeout(() => {
      isUserScrolling = false;
      scheduleMeasure();
    }, 120);

    emitState();
  };

  const handle: SmartScrollHandle = {
    getElement: () => containerRef,
    getState: () => state(),
    scrollToBottom,
    scrollToTop: (options) => scrollToPosition(0, options),
    scrollTo: (top, options) => scrollToPosition(top, options),
    pauseFollow,
    resumeFollow,
  };

  onMount(() => {
    isMounted = true;
    props.ref?.(handle);
    resetResizeObserver();
    emitState();
    followBottom();
  });

  createEffect(() => {
    props.followKey;
    props.active;
    followBottom();
  });

  onCleanup(() => {
    isMounted = false;
    if (frameId) {
      cancelAnimationFrame(frameId);
    }
    if (userScrollTimer) {
      clearTimeout(userScrollTimer);
    }
    resizeObserver?.disconnect();
  });

  return (
    <ScrollArea
      class={[
        props.class,
        ...Object.entries(props.classList ?? {})
          .filter(([, active]) => active)
          .map(([className]) => className),
      ].filter(Boolean).join(' ')}
      size="md"
      contentClass="agent-ui-smart-scroll-content"
      viewportRef={handleViewportRef}
      contentRef={handleContentRef}
      onScroll={handleScroll}
    >
      {props.children}
    </ScrollArea>
  );
}