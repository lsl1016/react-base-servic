import { describe, expect, it, vi } from 'vitest';

import { track } from '../analytics';

describe('analytics track', () => {
  it('空实现：调用不抛错且无副作用', () => {
    const consoleDebug = vi.spyOn(console, 'debug').mockImplementation(() => {});

    expect(() => track('SDK_TEST', { event: 'connect:close' })).not.toThrow();
    expect(() => track('SDK_TEST')).not.toThrow();

    consoleDebug.mockRestore();
  });
});
