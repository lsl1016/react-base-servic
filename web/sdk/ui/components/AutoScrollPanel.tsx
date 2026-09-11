/**
 * 可滚动区域：自定义滚动条、底部渐隐遮罩，流式内容时自动滚到底（用户上滑后暂停）。
 */

import { createEffect, type JSX } from 'solid-js';

export interface AutoScrollPanelProps {
  class?: string;
  /** 内容变化时递增或变更，用于触发跟随滚动 */
  followKey?: unknown;
  maxHeight?: number | string;
  children: JSX.Element;
}

export function AutoScrollPanel(props: AutoScrollPanelProps) {
  let scrollRef: HTMLDivElement | undefined;
  let autoScroll = true;

  createEffect(() => {
    props.followKey;
    if (autoScroll && scrollRef) {
      scrollRef.scrollTop = scrollRef.scrollHeight;
    }
  });

  const handleScroll = () => {
    if (!scrollRef) return;
    const distanceFromBottom = scrollRef.scrollHeight - scrollRef.scrollTop - scrollRef.clientHeight;
    autoScroll = distanceFromBottom <= 10;
  };

  const maxHeight = () => {
    const value = props.maxHeight;
    if (value === undefined) return undefined;
    return typeof value === 'number' ? `${value}px` : value;
  };

  return (
    <div
      class={`agent-ui-auto-scroll-panel${props.class ? ` ${props.class}` : ''}`}
      style={maxHeight() ? { 'max-height': maxHeight() } : undefined}
      ref={scrollRef}
      onScroll={handleScroll}
    >
      {props.children}
    </div>
  );
}
