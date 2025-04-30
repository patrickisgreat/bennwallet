import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

const inCI = !!process.env.CI       // GitHub Actions sets this automatically

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: true,

    // be explicit that we’re tweaking the threads pool
    pool: 'threads',

    poolOptions: {
      threads: {
        // keep min ≤ max no matter where we run
        minThreads: 1,
        maxThreads: inCI ? 2 : 8,   // CI keeps things tiny, local uses all the juice
      },
    },
  },
})