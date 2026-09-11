import { createComponent } from 'solid-js';
import { render } from 'solid-js/web';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { InputArea } from '../ui/components/InputArea';

afterEach(() => {
  document.body.innerHTML = '';
});

describe('InputArea attachments', () => {
  it('enables the attachment upload entry by default', () => {
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(InputArea, {
        isRunning: false,
        onSend: vi.fn(),
        onCancel: vi.fn(),
        onUploadAttachment: vi.fn(),
      }),
      host,
    );

    expect(host.querySelector('[aria-label="上传附件"]')).not.toBeNull();
    expect(host.querySelector('input[type="file"]')?.getAttribute('accept')).toBe('.csv,.md,.txt');
    dispose();
  });

  it('hides the attachment upload UI when attachmentUpload is false', () => {
    const host = document.createElement('div');
    const dispose = render(
      () => createComponent(InputArea, {
        isRunning: false,
        attachmentUpload: false,
        onSend: vi.fn(),
        onCancel: vi.fn(),
        onUploadAttachment: vi.fn(),
      }),
      host,
    );

    expect(host.querySelector('[aria-label="上传附件"]')).toBeNull();
    expect(host.querySelector('input[type="file"]')).toBeNull();
    expect(host.querySelector('.agent-ui-input-attachments-container')).toBeNull();
    dispose();
  });

  it('uploads selected files, renders cards, and supports removal', async () => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const upload = vi.fn(async (file: File) => ({
      fileId: 'file_1',
      fileName: file.name,
      ext: 'md',
      size: file.size,
      uploadedAt: '2026-07-13 10:00:00',
    }));
    const onAttachmentUploadSuccess = vi.fn();
    const dispose = render(
      () => createComponent(InputArea, {
        isRunning: false,
        onSend: vi.fn(),
        onCancel: vi.fn(),
        onUploadAttachment: upload,
        onAttachmentUploadSuccess,
      }),
      host,
    );

    const actions = host.querySelector('.agent-ui-input-actions');
    expect(actions?.firstElementChild?.classList.contains('agent-ui-input-actions-left')).toBe(true);
    expect(actions?.lastElementChild?.classList.contains('agent-ui-input-actions-right')).toBe(true);

    const input = host.querySelector<HTMLInputElement>('input[type="file"]')!;
    const file = new File(['# context'], 'context.md', { type: 'text/markdown' });
    Object.defineProperty(input, 'files', { configurable: true, value: [file] });
    input.dispatchEvent(new Event('change', { bubbles: true }));

    await vi.waitFor(() => expect(upload).toHaveBeenCalledWith(file));
    await vi.waitFor(() => expect(onAttachmentUploadSuccess).toHaveBeenCalledWith(file, {
      fileId: 'file_1',
      fileName: 'context.md',
      ext: 'md',
      size: file.size,
      uploadedAt: '2026-07-13 10:00:00',
    }));
    await vi.waitFor(() => expect(host.textContent).toContain('context.md'));
    expect(host.querySelector('.agent-ui-input-attachment')).not.toBeNull();
    expect(host.querySelector('.agent-ui-input-attachments-container')?.nextElementSibling?.classList
      .contains('agent-ui-input-wrapper')).toBe(true);

    host.querySelector<HTMLButtonElement>('[aria-label="删除附件 context.md"]')?.click();
    expect(host.querySelector('.agent-ui-input-attachment')).toBeNull();
    dispose();
  });

  it('supports csv, md, and txt files and rejects other extensions', async () => {
    const host = document.createElement('div');
    const upload = vi.fn(async (file: File) => ({
      fileId: file.name,
      fileName: file.name,
      ext: file.name.split('.').pop()!,
      size: file.size,
      uploadedAt: '2026-07-14 10:00:00',
    }));
    const onAttachmentUploadError = vi.fn();
    const dispose = render(
      () => createComponent(InputArea, {
        isRunning: false,
        onSend: vi.fn(),
        onCancel: vi.fn(),
        onUploadAttachment: upload,
        onAttachmentUploadError,
      }),
      host,
    );

    const input = host.querySelector<HTMLInputElement>('input[type="file"]')!;
    const supportedFiles = [
      new File(['a,b'], 'data.CSV'),
      new File(['# title'], 'readme.md'),
      new File(['notes'], 'notes.txt'),
    ];
    Object.defineProperty(input, 'files', { configurable: true, value: supportedFiles });
    input.dispatchEvent(new Event('change', { bubbles: true }));
    await vi.waitFor(() => expect(upload).toHaveBeenCalledTimes(3));

    const unsupportedFile = new File(['{}'], 'data.json');
    Object.defineProperty(input, 'files', { configurable: true, value: [unsupportedFile] });
    input.dispatchEvent(new Event('change', { bubbles: true }));

    expect(upload).toHaveBeenCalledTimes(3);
    expect(onAttachmentUploadError).toHaveBeenLastCalledWith('仅支持 CSV、MD、TXT 文件');
    dispose();
  });

  it('rejects a selection that would exceed five files', () => {
    const host = document.createElement('div');
    const upload = vi.fn();
    const onAttachmentUploadError = vi.fn();
    const dispose = render(
      () => createComponent(InputArea, {
        isRunning: false,
        onSend: vi.fn(),
        onCancel: vi.fn(),
        onUploadAttachment: upload,
        onAttachmentUploadError,
      }),
      host,
    );

    const input = host.querySelector<HTMLInputElement>('input[type="file"]')!;
    const files = Array.from({ length: 6 }, (_, index) => new File(['x'], `${index}.txt`));
    Object.defineProperty(input, 'files', { configurable: true, value: files });
    input.dispatchEvent(new Event('change', { bubbles: true }));

    expect(upload).not.toHaveBeenCalled();
    expect(onAttachmentUploadError).toHaveBeenLastCalledWith('最多上传 5 个文件');
    expect(host.querySelector('.agent-ui-input-attachment')).toBeNull();
    dispose();
  });

  it('rejects a single file larger than 50MB', () => {
    const host = document.createElement('div');
    const upload = vi.fn();
    const onAttachmentUploadError = vi.fn();
    const dispose = render(
      () => createComponent(InputArea, {
        isRunning: false,
        onSend: vi.fn(),
        onCancel: vi.fn(),
        onUploadAttachment: upload,
        onAttachmentUploadError,
      }),
      host,
    );

    const input = host.querySelector<HTMLInputElement>('input[type="file"]')!;
    const file = new File(['x'], 'large.txt');
    Object.defineProperty(file, 'size', { configurable: true, value: 50 * 1024 * 1024 + 1 });
    Object.defineProperty(input, 'files', { configurable: true, value: [file] });
    input.dispatchEvent(new Event('change', { bubbles: true }));

    expect(upload).not.toHaveBeenCalled();
    expect(onAttachmentUploadError).toHaveBeenLastCalledWith('单个文件大小不能超过 50MB');
    dispose();
  });

  it('rejects files when their cumulative size would exceed 50MB', async () => {
    const host = document.createElement('div');
    const upload = vi.fn(async (file: File) => ({
      fileId: file.name,
      fileName: file.name,
      ext: 'txt',
      size: file.size,
      uploadedAt: '2026-07-14 10:00:00',
    }));
    const onAttachmentUploadError = vi.fn();
    const dispose = render(
      () => createComponent(InputArea, {
        isRunning: false,
        onSend: vi.fn(),
        onCancel: vi.fn(),
        onUploadAttachment: upload,
        onAttachmentUploadError,
      }),
      host,
    );

    const input = host.querySelector<HTMLInputElement>('input[type="file"]')!;
    const firstFile = new File(['x'], 'first.txt');
    Object.defineProperty(firstFile, 'size', { configurable: true, value: 30 * 1024 * 1024 });
    Object.defineProperty(input, 'files', { configurable: true, value: [firstFile] });
    input.dispatchEvent(new Event('change', { bubbles: true }));
    await vi.waitFor(() => expect(upload).toHaveBeenCalledTimes(1));

    const secondFile = new File(['x'], 'second.txt');
    Object.defineProperty(secondFile, 'size', { configurable: true, value: 21 * 1024 * 1024 });
    Object.defineProperty(input, 'files', { configurable: true, value: [secondFile] });
    input.dispatchEvent(new Event('change', { bubbles: true }));

    expect(upload).toHaveBeenCalledTimes(1);
    expect(onAttachmentUploadError).toHaveBeenLastCalledWith('文件总大小不能超过 50MB');
    expect(host.textContent).not.toContain('second.txt');
    dispose();
  });

  it('reports upload request failures', async () => {
    const host = document.createElement('div');
    const onAttachmentUploadError = vi.fn();
    const dispose = render(
      () => createComponent(InputArea, {
        isRunning: false,
        onSend: vi.fn(),
        onCancel: vi.fn(),
        onUploadAttachment: vi.fn(async () => {
          throw new Error('服务暂不可用');
        }),
        onAttachmentUploadError,
      }),
      host,
    );

    const input = host.querySelector<HTMLInputElement>('input[type="file"]')!;
    Object.defineProperty(input, 'files', {
      configurable: true,
      value: [new File(['context'], 'context.txt')],
    });
    input.dispatchEvent(new Event('change', { bubbles: true }));

    await vi.waitFor(() => expect(onAttachmentUploadError).toHaveBeenLastCalledWith('服务暂不可用'));
    dispose();
  });
});
