import { defineConfig, devices } from '@playwright/test';

const port = Number(process.env.POSTIK_E2E_PORT ?? 8090);

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: `http://localhost:${port}`,
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'bash e2e/serve.sh',
    url: `http://localhost:${port}/readyz`,
    reuseExistingServer: false,
    timeout: 120_000,
    env: { POSTIK_E2E_PORT: String(port) },
  },
});
