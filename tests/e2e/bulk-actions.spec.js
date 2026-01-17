const { test, expect } = require('@playwright/test');
const fs = require('fs/promises');
const {
  buildUser,
  register,
  createFinalizedCardFromModal,
  expectToast,
  ensureSelectedCount,
} = require('./helpers');

async function clickAction(page, label) {
  await page.getByRole('button', { name: 'Actions' }).click();
  const menu = page.locator('.dropdown-menu--visible');
  await expect(menu).toBeVisible();
  await menu.locator('.dropdown-item').filter({ hasText: label }).click();
}

test('bulk actions update visibility, export, and delete cards', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'bulk');
  await register(page, user);

  const year = new Date().getFullYear();
  await createFinalizedCardFromModal(page, { title: 'Bulk Card One', year });
  await createFinalizedCardFromModal(page, { title: 'Bulk Card Two', year: year + 1 });

  await page.goto('/dashboard');
  await expect(page.locator('.dashboard-card-preview')).toHaveCount(2);
  await ensureSelectedCount(page, 2);

  await clickAction(page, 'Make Private');
  await expectToast(page, 'cards updated');
  await expect(page.locator('.visibility-badge--private')).toHaveCount(2);

  await ensureSelectedCount(page, 2);
  const downloadPromise = page.waitForEvent('download');
  await clickAction(page, 'Export Cards');
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toMatch(/^yearofbingo_export_\d{4}-\d{2}-\d{2}\.zip$/);
  const exportPath = testInfo.outputPath('cards-export.zip');
  await download.saveAs(exportPath);
  const buffer = await fs.readFile(exportPath);
  expect(buffer.slice(0, 2).toString('utf8')).toBe('PK');
  expect(buffer.length).toBeGreaterThan(100);

  await ensureSelectedCount(page, 2);
  page.once('dialog', (dialog) => dialog.accept());
  await clickAction(page, 'Delete');
  await expectToast(page, 'deleted');
  await expect(page.locator('#cards-list')).toContainText('No cards yet');
});
