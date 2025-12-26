const { defineConfig, devices } = require('@playwright/test');

const isHeadless = process.env.PLAYWRIGHT_HEADLESS !== 'false';
const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';
const outputDir = process.env.PLAYWRIGHT_OUTPUT_DIR || 'test-results';
const reportDir = process.env.PLAYWRIGHT_REPORT_DIR || 'playwright-report';
const workers = (() => {
  const raw = process.env.PLAYWRIGHT_WORKERS;
  if (!raw) return 1;
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1;
})();

module.exports = defineConfig({
  testDir: 'tests/e2e',
  timeout: 60000,
  expect: {
    timeout: 10000,
  },
  fullyParallel: false,
  workers,
  outputDir,
  reporter: [
    ['list'],
    ['html', { open: 'never', outputFolder: reportDir }],
  ],
  use: {
    baseURL,
    headless: isHeadless,
    actionTimeout: 10000,
    navigationTimeout: 30000,
    acceptDownloads: true,
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },
  ],
});
