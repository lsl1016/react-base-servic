import { For, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';
import IconLucideChevronLeft from '~icons/lucide/chevron-left';
import IconLucideChevronRight from '~icons/lucide/chevron-right';
import IconLucideDownload from '~icons/lucide/download';
import IconLucideEye from '~icons/lucide/eye';
import IconLucideFile from '~icons/lucide/file';
import IconLucideFileArchive from '~icons/lucide/file-archive';
import IconLucideFileAudio from '~icons/lucide/file-audio';
import IconLucideFileCode2 from '~icons/lucide/file-code-2';
import IconLucideFileJson from '~icons/lucide/file-json';
import IconLucideFileSpreadsheet from '~icons/lucide/file-spreadsheet';
import IconLucideFileText from '~icons/lucide/file-text';
import IconLucideFileVideo from '~icons/lucide/file-video';
import IconLucidePresentation from '~icons/lucide/presentation';
import IconLucideRotateCcw from '~icons/lucide/rotate-ccw';
import IconLucideX from '~icons/lucide/x';
import IconLucideZoomIn from '~icons/lucide/zoom-in';
import IconLucideZoomOut from '~icons/lucide/zoom-out';
import IconMaterialSymbolsImageOutlineRounded from '~icons/material-symbols/image-outline-rounded';
import type { ToolCallState } from '../../runtime/types';
import { useAgentUITheme } from '../theme';

export interface DisplayFileArtifact {
  type: string;
  name: string;
  uri: string;
}

export interface DisplayFilesProps {
  toolCall: ToolCallState;
}

type DisplayFileKind = 'html' | 'json' | 'pdf' | 'spreadsheet' | 'document'
  | 'presentation' | 'archive' | 'audio' | 'video' | 'text' | 'file';

const artifactCandidates = (toolCall: ToolCallState): unknown[] => {
  if (Array.isArray(toolCall.input?.artifacts)) return toolCall.input.artifacts;
  return Array.isArray(toolCall.meta?.artifacts) ? toolCall.meta.artifacts : [];
};

export const hasDisplayFileArtifacts = (toolCall: ToolCallState): boolean => (
  artifactCandidates(toolCall).length > 0
);

const isSafeArtifactUri = (uri: string) => {
  const value = uri.trim();
  return value.startsWith('/')
    || value.startsWith('./')
    || value.startsWith('../')
    || /^https?:\/\//i.test(value);
};

const inferArtifactName = (uri: string, index: number) => {
  const candidate = uri.split(/[?#]/, 1)[0].split('/').filter(Boolean).pop();
  if (!candidate) return `文件 ${index + 1}`;
  try {
    return decodeURIComponent(candidate);
  } catch {
    return candidate;
  }
};

export function readDisplayFileArtifacts(toolCall: ToolCallState): DisplayFileArtifact[] {
  return artifactCandidates(toolCall).flatMap((candidate, index) => {
    if (!candidate || typeof candidate !== 'object') return [];
    const record = candidate as Record<string, unknown>;
    const type = typeof record.type === 'string' ? record.type.trim().toLowerCase() : '';
    const uri = typeof record.uri === 'string' ? record.uri.trim() : '';
    if (!type || !uri || !isSafeArtifactUri(uri)) return [];
    const providedName = typeof record.name === 'string' ? record.name.trim() : '';
    return [{ type, uri, name: providedName || inferArtifactName(uri, index) }];
  });
}

const displayFileKind = (artifact: DisplayFileArtifact): DisplayFileKind => {
  const type = artifact.type.split(';', 1)[0];
  if (type === 'text/html' || type === 'application/xhtml+xml') return 'html';
  if (type === 'application/json' || type === 'text/json' || type.endsWith('+json')) return 'json';
  if (type === 'application/pdf') return 'pdf';
  if (type === 'text/csv' || type.includes('excel') || type.includes('spreadsheet')) return 'spreadsheet';
  if (type.includes('word') || type === 'application/rtf') return 'document';
  if (type.includes('powerpoint') || type.includes('presentation')) return 'presentation';
  if (type.includes('zip') || type.includes('gzip') || type.includes('compressed')
    || type.includes('archive') || type.includes('tar') || type.includes('rar')) return 'archive';
  if (type.startsWith('audio/')) return 'audio';
  if (type.startsWith('video/')) return 'video';
  if (type.startsWith('text/')) return 'text';
  return 'file';
};

function DisplayFileIcon(props: { kind: DisplayFileKind }) {
  switch (props.kind) {
    case 'html':
      return <IconLucideFileCode2 width="22" height="22" />;
    case 'json':
      return <IconLucideFileJson width="22" height="22" />;
    case 'spreadsheet':
      return <IconLucideFileSpreadsheet width="22" height="22" />;
    case 'archive':
      return <IconLucideFileArchive width="22" height="22" />;
    case 'audio':
      return <IconLucideFileAudio width="22" height="22" />;
    case 'video':
      return <IconLucideFileVideo width="22" height="22" />;
    case 'presentation':
      return <IconLucidePresentation width="22" height="22" />;
    case 'pdf':
    case 'document':
    case 'text':
      return <IconLucideFileText width="22" height="22" />;
    default:
      return <IconLucideFile width="22" height="22" />;
  }
}

const openFileInNewTab = (uri: string) => {
  window.open(uri, '_blank', 'noopener,noreferrer');
};

export function DisplayFiles(props: DisplayFilesProps) {
  const theme = useAgentUITheme();
  const artifacts = createMemo(() => readDisplayFileArtifacts(props.toolCall));
  const images = createMemo(() => artifacts().filter((artifact) => artifact.type.startsWith('image/')));
  const files = createMemo(() => artifacts().filter((artifact) => !artifact.type.startsWith('image/')));
  const [selectedImageIndex, setSelectedImageIndex] = createSignal<number | undefined>();
  const [scale, setScale] = createSignal(1);
  const [rotation, setRotation] = createSignal(0);

  const selectedImage = createMemo(() => {
    const index = selectedImageIndex();
    return index === undefined ? undefined : images()[index];
  });

  const closePreview = () => setSelectedImageIndex(undefined);
  const selectImage = (index: number) => {
    if (images().length === 0) return;
    setSelectedImageIndex((index + images().length) % images().length);
    setScale(1);
    setRotation(0);
  };
  const moveImage = (offset: number) => {
    const index = selectedImageIndex();
    if (index === undefined || images().length < 2) return;
    selectImage(index + offset);
  };
  const updateScale = (offset: number) => {
    setScale((value) => Math.min(3, Math.max(0.5, value + offset)));
  };

  createEffect(() => {
    if (!selectedImage()) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closePreview();
      if (event.key === 'ArrowLeft') moveImage(-1);
      if (event.key === 'ArrowRight') moveImage(1);
      if (event.key === '+' || event.key === '=') updateScale(0.25);
      if (event.key === '-') updateScale(-0.25);
    };
    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => document.removeEventListener('keydown', handleKeyDown));
  });

  return (
    <section class="agent-ui-display-files" aria-label="文件">
      <Show
        when={artifacts().length > 0}
        fallback={<div class="agent-ui-display-files-empty">没有可显示的文件</div>}
      >
        <Show when={images().length > 0}>
          <div class="agent-ui-display-files-images">
            <For each={images()}>
              {(artifact, index) => {
                const [loadFailed, setLoadFailed] = createSignal(false);
                return (
                  <figure class="agent-ui-display-files-image-item">
                    <button
                      type="button"
                      class="agent-ui-display-files-image-button"
                      title={loadFailed() ? `${artifact.name} 加载失败` : `放大 ${artifact.name}`}
                      aria-label={loadFailed() ? `${artifact.name} 加载失败` : `放大 ${artifact.name}`}
                      disabled={loadFailed()}
                      onClick={() => selectImage(index())}
                    >
                      <Show
                        when={!loadFailed()}
                        fallback={(
                          <span class="agent-ui-display-files-image-fallback" role="img" aria-label="图片加载失败">
                            <IconMaterialSymbolsImageOutlineRounded width="42" height="42" />
                            <span>图片加载失败</span>
                          </span>
                        )}
                      >
                        <img
                          src={artifact.uri}
                          alt={artifact.name}
                          loading="lazy"
                          onError={() => setLoadFailed(true)}
                        />
                      </Show>
                    </button>
                    <figcaption title={artifact.name}>{artifact.name}</figcaption>
                  </figure>
                );
              }}
            </For>
          </div>
        </Show>

        <Show when={files().length > 0}>
          <div class="agent-ui-display-files-list">
            <For each={files()}>
              {(artifact) => {
                const kind = displayFileKind(artifact);
                return (
                  <article class="agent-ui-display-files-card" data-file-kind={kind}>
                    <span class={`agent-ui-display-files-card-icon is-${kind}`} aria-hidden="true">
                      <DisplayFileIcon kind={kind} />
                    </span>
                    <div class="agent-ui-display-files-card-info">
                      <strong title={artifact.name}>{artifact.name}</strong>
                      <span>{artifact.type}</span>
                    </div>
                    <div class="agent-ui-display-files-card-actions">
                      <Show
                        when={kind === 'html'}
                        fallback={(
                          <button type="button" onClick={() => openFileInNewTab(artifact.uri)}>
                            <IconLucideDownload width="16" height="16" />
                            <span>下载</span>
                          </button>
                        )}
                      >
                        <button type="button" onClick={() => openFileInNewTab(artifact.uri)}>
                          <IconLucideEye width="16" height="16" />
                          <span>预览</span>
                        </button>
                      </Show>
                    </div>
                  </article>
                );
              }}
            </For>
          </div>
        </Show>
      </Show>

      <Show when={selectedImage()}>
        {(artifact) => (
          <Portal>
            <div
              class="agent-ui-display-files-lightbox"
              data-agent-ui-theme={theme()}
              role="dialog"
              aria-modal="true"
              aria-label={`预览 ${artifact().name}`}
              onClick={closePreview}
            >
              <header class="agent-ui-display-files-lightbox-header" onClick={(event) => event.stopPropagation()}>
                <div class="agent-ui-display-files-lightbox-title">
                  <strong>{artifact().name}</strong>
                  <Show when={images().length > 1}>
                    <span>{(selectedImageIndex() ?? 0) + 1} / {images().length}</span>
                  </Show>
                </div>
                <div class="agent-ui-display-files-lightbox-actions">
                  <button type="button" title="缩小" aria-label="缩小" disabled={scale() <= 0.5} onClick={() => updateScale(-0.25)}>
                    <IconLucideZoomOut width="19" height="19" />
                  </button>
                  <span class="agent-ui-display-files-lightbox-scale">{Math.round(scale() * 100)}%</span>
                  <button type="button" title="放大" aria-label="放大" disabled={scale() >= 3} onClick={() => updateScale(0.25)}>
                    <IconLucideZoomIn width="19" height="19" />
                  </button>
                  <button type="button" title="向左旋转" aria-label="向左旋转" onClick={() => setRotation((value) => value - 90)}>
                    <IconLucideRotateCcw width="18" height="18" />
                  </button>
                  <button type="button" title="下载" aria-label="下载" onClick={() => openFileInNewTab(artifact().uri)}>
                    <IconLucideDownload width="18" height="18" />
                  </button>
                  <button type="button" title="关闭" aria-label="关闭" onClick={closePreview}>
                    <IconLucideX width="20" height="20" />
                  </button>
                </div>
              </header>
              <div
                class="agent-ui-display-files-lightbox-stage"
                classList={{ 'is-single': images().length === 1 }}
              >
                <Show when={images().length > 1}>
                  <button
                    type="button"
                    class="agent-ui-display-files-lightbox-nav is-previous"
                    title="上一张"
                    aria-label="上一张"
                    onClick={(event) => {
                      event.stopPropagation();
                      moveImage(-1);
                    }}
                  >
                    <IconLucideChevronLeft width="28" height="28" />
                  </button>
                </Show>
                <div class="agent-ui-display-files-lightbox-image-scroll">
                  <img
                    src={artifact().uri}
                    alt={artifact().name}
                    classList={{ 'is-sideways': Math.abs(rotation() % 180) === 90 }}
                    style={{ transform: `rotate(${rotation()}deg) scale(${scale()})` }}
                    onClick={(event) => event.stopPropagation()}
                  />
                </div>
                <Show when={images().length > 1}>
                  <button
                    type="button"
                    class="agent-ui-display-files-lightbox-nav is-next"
                    title="下一张"
                    aria-label="下一张"
                    onClick={(event) => {
                      event.stopPropagation();
                      moveImage(1);
                    }}
                  >
                    <IconLucideChevronRight width="28" height="28" />
                  </button>
                </Show>
              </div>
            </div>
          </Portal>
        )}
      </Show>
    </section>
  );
}
