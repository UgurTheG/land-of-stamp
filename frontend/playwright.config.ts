import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: 'http://localhost:5173',
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'on-first-retry',
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
  // The frontend dev server (vite) must be running and proxying to the backend.
  webServer: [
    {
      command: 'cd ../backend && DB_PATH=:memory: JWT_SECRET=e2e-test-secret TEST_SEED=true go run .',
      port: 8080,
      reuseExistingServer: true,
      timeout: 15_000,
    },
    {
      command: 'npm run dev -- --port 5173',
      port: 5173,
      reuseExistingServer: true,
      timeout: 15_000,
    },
  ],
});

