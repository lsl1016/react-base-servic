import { describe, expect, it } from 'vitest';
import packageJson from '../package.json';

describe('package contract', () => {
  it('publishes library and browser entrypoints consistently', () => {
    expect(packageJson.main).toBe('./dist/index.js');
    expect(packageJson.module).toBe('./dist/index.js');
    expect(packageJson.types).toBe('./dist/index.d.ts');
    expect(packageJson.exports['.']).toEqual({
      types: './dist/index.d.ts',
      import: './dist/index.js',
    });
    expect(packageJson.exports['./browser']).toEqual({
      types: './dist/browser.d.ts',
      import: './dist/browser.js',
    });
    expect(packageJson.exports['./style.css']).toBe('./dist/style.css');
  });

  it('uses the lightweight diff renderer without Monaco', () => {
    expect(packageJson.dependencies.diff).toBeDefined();
    expect(packageJson.dependencies['highlight.js']).toBeDefined();
    expect(packageJson.dependencies).not.toHaveProperty('monaco-editor');
  });
});
