import { describe, expect, it } from 'vitest';
import { SDK_VERSION, SDK_JS_FILE, SDK_CSS_FILE } from '../build/constants';

describe('build constants', () => {
  it('uses the required versioned browser asset names', () => {
    expect(SDK_VERSION).toBe('0.0.1');
    expect(SDK_JS_FILE).toBe('0.0.1.js');
    expect(SDK_CSS_FILE).toBe('0.0.1.css');
  });
});
