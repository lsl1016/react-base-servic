import { createComponent } from 'solid-js';
import { render } from 'solid-js/web';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ToolCallState } from '../runtime/types';
import { DisplayFiles, readDisplayFileArtifacts } from '../ui/components/DisplayFiles';

afterEach(() => {
  document.body.innerHTML = '';
  vi.restoreAllMocks();
});

const displayFilesCall = (artifacts: unknown[]): ToolCallState => ({
  toolUseId: 'display_1',
  toolName: 'displayFiles',
  input: { artifacts },
  status: 'done',
  result: '{"displayed":3}',
  executedBy: 'internal',
});

describe('DisplayFiles', () => {
  it('normalizes optional names and filters unsafe or incomplete artifacts', () => {
    const artifacts = readDisplayFileArtifacts(displayFilesCall([
      { type: 'IMAGE/PNG', uri: '/files/%E8%B6%8B%E5%8A%BF.png' },
      { type: 'text/html', name: '报告.html', uri: 'https://example.com/report' },
      { type: 'application/pdf', uri: 'javascript:alert(1)' },
      { type: '', uri: '/files/invalid' },
    ]));

    expect(artifacts).toEqual([
      { type: 'image/png', name: '趋势.png', uri: '/files/%E8%B6%8B%E5%8A%BF.png' },
      { type: 'text/html', name: '报告.html', uri: 'https://example.com/report' },
    ]);
  });


  it('reads persisted python_exec artifacts from tool meta without exposing URLs to the model result', () => {
    const artifacts = readDisplayFileArtifacts({
      toolUseId: 'python_1',
      toolName: 'python_exec',
      input: { python: 'print({})' },
      status: 'done',
      result: '{"artifacts":[{"type":"image/png","name":"趋势.png","bytes":1024}]}',
      meta: {
        artifacts: [{ type: 'image/png', name: '趋势.png', uri: '/artifact/image-1' }],
      },
      executedBy: 'internal',
    });

    expect(artifacts).toEqual([
      { type: 'image/png', name: '趋势.png', uri: '/artifact/image-1' },
    ]);
  });

  it('renders HTML with preview only and other files with download only', () => {
    const open = vi.spyOn(window, 'open').mockReturnValue(null);
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(
      () => createComponent(DisplayFiles, {
        toolCall: displayFilesCall([
          { type: 'image/png', name: '趋势一.png', uri: '/artifact/image-1' },
          { type: 'image/jpeg', name: '趋势二.jpg', uri: '/artifact/image-2' },
          { type: 'application/pdf', name: '分析报告.pdf', uri: '/artifact/report' },
          { type: 'text/html', name: '交互报告.html', uri: '/artifact/html' },
          { type: 'application/json', name: '结果.json', uri: '/artifact/json' },
        ]),
      }),
      host,
    );

    expect(host.querySelectorAll('.agent-ui-display-files-image-item')).toHaveLength(2);
    expect(host.querySelector('img')?.getAttribute('src')).toBe('/artifact/image-1');
    expect(host.querySelectorAll('.agent-ui-display-files-card')).toHaveLength(3);
    expect(host.textContent).toContain('分析报告.pdf');
    expect(host.textContent).toContain('交互报告.html');
    expect(host.textContent).toContain('结果.json');

    const pdfCard = host.querySelector('[data-file-kind="pdf"]') as HTMLElement;
    const htmlCard = host.querySelector('[data-file-kind="html"]') as HTMLElement;
    const jsonCard = host.querySelector('[data-file-kind="json"]') as HTMLElement;
    expect(pdfCard.querySelectorAll('button')).toHaveLength(1);
    expect(pdfCard.querySelector('button')?.textContent).toContain('下载');
    expect(htmlCard.querySelectorAll('button')).toHaveLength(1);
    expect(htmlCard.querySelector('button')?.textContent).toContain('预览');
    expect(htmlCard.textContent).not.toContain('下载');
    expect(jsonCard.querySelectorAll('button')).toHaveLength(1);
    expect(jsonCard.querySelector('button')?.textContent).toContain('下载');
    expect(jsonCard.textContent).not.toContain('预览');
    expect(host.querySelectorAll('.agent-ui-display-files-card-actions button')).toHaveLength(3);

    (htmlCard.querySelector('button') as HTMLButtonElement).click();
    expect(open).toHaveBeenCalledWith('/artifact/html', '_blank', 'noopener,noreferrer');

    (pdfCard.querySelector('button') as HTMLButtonElement).click();
    expect(open).toHaveBeenLastCalledWith('/artifact/report', '_blank', 'noopener,noreferrer');

    (jsonCard.querySelector('button') as HTMLButtonElement).click();
    expect(open).toHaveBeenLastCalledWith('/artifact/json', '_blank', 'noopener,noreferrer');

    dispose();
  });

  it('replaces a broken image with a stable fallback and disables preview', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(
      () => createComponent(DisplayFiles, {
        toolCall: displayFilesCall([
          { type: 'image/png', name: '损坏图片.png', uri: '/artifact/broken-image' },
        ]),
      }),
      host,
    );

    const image = host.querySelector('img') as HTMLImageElement;
    image.dispatchEvent(new Event('error'));

    const button = host.querySelector('.agent-ui-display-files-image-button') as HTMLButtonElement;
    const fallback = host.querySelector('.agent-ui-display-files-image-fallback') as HTMLElement;
    expect(host.querySelector('img')).toBeNull();
    expect(fallback).not.toBeNull();
    expect(fallback.textContent).toContain('图片加载失败');
    expect(fallback.querySelector('svg')).not.toBeNull();
    expect(button.disabled).toBe(true);

    button.click();
    expect(document.body.querySelector('[role="dialog"]')).toBeNull();
    dispose();
  });

  it('uses the full lightbox stage for a single image', () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(
      () => createComponent(DisplayFiles, {
        toolCall: displayFilesCall([
          { type: 'image/png', name: '单张图片.png', uri: '/artifact/single-image' },
        ]),
      }),
      host,
    );

    (host.querySelector('.agent-ui-display-files-image-button') as HTMLButtonElement).click();
    const stage = document.body.querySelector('.agent-ui-display-files-lightbox-stage') as HTMLElement;
    expect(stage.classList.contains('is-single')).toBe(true);
    expect(stage.querySelectorAll('.agent-ui-display-files-lightbox-nav')).toHaveLength(0);
    const previewImage = stage.querySelector('img') as HTMLImageElement;
    expect(previewImage.getAttribute('src')).toBe('/artifact/single-image');

    previewImage.click();
    expect(document.body.querySelector('[role="dialog"]')).not.toBeNull();

    stage.click();
    expect(document.body.querySelector('[role="dialog"]')).toBeNull();

    dispose();
  });

  it('opens an image lightbox with zoom and previous/next navigation', () => {
    const open = vi.spyOn(window, 'open').mockReturnValue(null);
    const host = document.createElement('div');
    document.body.appendChild(host);
    const dispose = render(
      () => createComponent(DisplayFiles, {
        toolCall: displayFilesCall([
          { type: 'image/png', name: '第一张.png', uri: '/artifact/image-1' },
          { type: 'image/png', name: '第二张.png', uri: '/artifact/image-2' },
        ]),
      }),
      host,
    );

    (host.querySelector('.agent-ui-display-files-image-button') as HTMLButtonElement).click();
    const dialog = document.body.querySelector('[role="dialog"]') as HTMLElement;
    expect(dialog).not.toBeNull();
    expect(dialog.querySelector('.agent-ui-display-files-lightbox-stage')?.classList.contains('is-single')).toBe(false);
    expect(dialog.querySelector('img')?.getAttribute('alt')).toBe('第一张.png');
    expect(dialog.textContent).toContain('1 / 2');

    (dialog.querySelector('[aria-label="下一张"]') as HTMLButtonElement).click();
    expect(dialog.querySelector('img')?.getAttribute('alt')).toBe('第二张.png');
    expect(dialog.textContent).toContain('2 / 2');

    (dialog.querySelector('[aria-label="放大"]') as HTMLButtonElement).click();
    expect(dialog.textContent).toContain('125%');

    (dialog.querySelector('[aria-label="向左旋转"]') as HTMLButtonElement).click();
    const previewImage = dialog.querySelector('img') as HTMLImageElement;
    expect(previewImage.style.transform).toBe('rotate(-90deg) scale(1.25)');
    expect(previewImage.classList.contains('is-sideways')).toBe(true);

    (dialog.querySelector('[aria-label="下载"]') as HTMLButtonElement).click();
    expect(open).toHaveBeenCalledWith('/artifact/image-2', '_blank', 'noopener,noreferrer');

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft' }));
    expect(dialog.querySelector('img')?.getAttribute('alt')).toBe('第一张.png');
    expect((dialog.querySelector('img') as HTMLImageElement).style.transform).toBe('rotate(0deg) scale(1)');

    (dialog.querySelector('[aria-label="关闭"]') as HTMLButtonElement).click();
    expect(document.body.querySelector('[role="dialog"]')).toBeNull();
    dispose();
  });
});
