import { defineConfig } from '@playwright/test';
import path from 'path';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: 'http://127.0.0.1:8080',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
  webServer: {
    command: `go run ${path.resolve(__dirname, '..', 'cmd', 'mpc_editor')}`,
    cwd: path.resolve(__dirname, '..'),
    port: 8080,
    reuseExistingServer: true,
    timeout: 30_000,
  },
});
