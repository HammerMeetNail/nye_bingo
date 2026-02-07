const { defineConfig, devices } = require('@playwright/test');
const os = require('os');

const isCI = !!process.env.CI;
const isHeadless = process.env.PLAYWRIGHT_HEADLESS !== 'false';
const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';
const outputDir = process.env.PLAYWRIGHT_OUTPUT_DIR || 'test-results';
const reportDir = process.env.PLAYWRIGHT_REPORT_DIR || 'playwright-report';
const parsePositiveInt = (raw, fallback) => {
  const parsed = Number.parseInt(raw || '', 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
};
const parseNonNegativeInt = (raw, fallback) => {
  const parsed = Number.parseInt(raw || '', 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
};
const testTimeout = parsePositiveInt(process.env.PLAYWRIGHT_TEST_TIMEOUT_MS, isCI ? 90000 : 60000);
const expectTimeout = parsePositiveInt(process.env.PLAYWRIGHT_EXPECT_TIMEOUT_MS, isCI ? 15000 : 10000);
const actionTimeout = parsePositiveInt(process.env.PLAYWRIGHT_ACTION_TIMEOUT_MS, isCI ? 15000 : 10000);
const navigationTimeout = parsePositiveInt(process.env.PLAYWRIGHT_NAVIGATION_TIMEOUT_MS, isCI ? 45000 : 30000);
const retries = parseNonNegativeInt(process.env.PLAYWRIGHT_RETRIES, isCI ? 2 : 0);
const defaultWorkers = (() => {
  if (isCI) return 1;
  const cpuCount = Array.isArray(os.cpus()) ? os.cpus().length : 1;
  return Math.max(1, Math.min(4, cpuCount - 1));
})();
const workers = (() => {
  const raw = process.env.PLAYWRIGHT_WORKERS;
  if (!raw) return defaultWorkers;
  if (raw === 'auto') return defaultWorkers;
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : defaultWorkers;
})();

module.exports = defineConfig({
  testDir: 'tests/e2e',
  timeout: testTimeout,
  retries,
  forbidOnly: isCI,
  expect: {
    timeout: expectTimeout,
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
    actionTimeout,
    navigationTimeout,
    acceptDownloads: true,
    screenshot: 'only-on-failure',
    video: isCI ? 'on-first-retry' : 'retain-on-failure',
    trace: isCI ? 'on-first-retry' : 'retain-on-failure',
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
