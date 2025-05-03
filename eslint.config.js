import js from '@eslint/js';
import globals from 'globals';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import tseslint from 'typescript-eslint';

// Create a sanitized version of browser globals
const browserGlobals = { ...globals.browser };

// Fix any globals with trailing spaces
// This specifically addresses the "AudioWorkletGlobalScope " issue
if ('AudioWorkletGlobalScope ' in browserGlobals) {
  const value = browserGlobals['AudioWorkletGlobalScope '];
  delete browserGlobals['AudioWorkletGlobalScope '];
  browserGlobals['AudioWorkletGlobalScope'] = value;
}

export default tseslint.config(
  { ignores: ['dist'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2020,
      globals: browserGlobals,
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
    },
  }
);
