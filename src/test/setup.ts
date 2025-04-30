import '@testing-library/jest-dom';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

// Runs a cleanup after each test case
afterEach(() => {
  cleanup();
});

console.log('Test setup loaded successfully');

// Add globals since we enabled globals in vite.config.ts
(global as any).IS_TEST_ENV = true; 