import '@testing-library/jest-dom';
import { vi } from 'vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

// Mock Firebase modules
vi.mock('../firebase/firebase');

// Mock window.matchMedia
interface MediaQueryList {
  matches: boolean;
  media: string;
  onchange: ((this: MediaQueryList, ev: MediaQueryListEvent) => void) | null;
  addListener: (listener: (this: MediaQueryList, ev: MediaQueryListEvent) => void) => void;
  removeListener: (listener: (this: MediaQueryList, ev: MediaQueryListEvent) => void) => void;
  addEventListener: <K extends keyof MediaQueryListEventMap>(
    type: K,
    listener: (this: MediaQueryList, ev: MediaQueryListEventMap[K]) => void
  ) => void;
  removeEventListener: <K extends keyof MediaQueryListEventMap>(
    type: K,
    listener: (this: MediaQueryList, ev: MediaQueryListEventMap[K]) => void
  ) => void;
  dispatchEvent: (event: Event) => boolean;
}

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation(
    (query: string): MediaQueryList => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })
  ),
});

// Clean up after each test
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

console.log('Test setup loaded successfully');

// Add globals since we enabled globals in vite.config.ts
// Define the type for the global object
declare global {
  // eslint-disable-next-line no-var
  var IS_TEST_ENV: boolean;
}

// Set the global variable
global.IS_TEST_ENV = true;
