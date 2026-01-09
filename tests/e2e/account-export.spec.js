const { test, expect } = require('@playwright/test');
const fs = require('fs/promises');
const { buildUser, register, expectToast } = require('./helpers');

test('account export downloads a zip', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'accountexport');
  await register(page, user);

  await page.goto('/#profile');
  const downloadPromise = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Download Export' }).click();
  const download = await downloadPromise;

  expect(download.suggestedFilename()).toMatch(/^yearofbingo_account_export_\d{4}-\d{2}-\d{2}\.zip$/);
  const exportPath = testInfo.outputPath('account-export.zip');
  await download.saveAs(exportPath);
  const buffer = await fs.readFile(exportPath);
  expect(buffer.slice(0, 2).toString('utf8')).toBe('PK');
  expect(buffer.length).toBeGreaterThan(100);

  await expectToast(page, 'Export downloaded');
});
