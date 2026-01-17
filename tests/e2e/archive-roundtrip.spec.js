const { test, expect } = require('@playwright/test');
const { buildUser, register, createFinalizedCardFromModal, expectToast } = require('./helpers');

async function clickAction(page, label) {
  await page.getByRole('button', { name: 'Actions' }).click();
  const menu = page.locator('.dropdown-menu--visible');
  await expect(menu).toBeVisible();
  const pattern = new RegExp(`^\\s*${label}\\s*$`);
  await menu.locator('.dropdown-item').filter({ hasText: pattern }).click();
}

test('cards can be archived and unarchived from dashboard', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'arch');
  await register(page, user);

  const title = `Archive Roundtrip ${Date.now()}`;
  const year = new Date().getFullYear();
  await createFinalizedCardFromModal(page, { title, year });

  await page.goto('/dashboard');
  const preview = page.locator('.dashboard-card-preview').filter({ hasText: title });
  await expect(preview).toBeVisible();
  await expect(preview.locator('.archive-badge')).toHaveCount(0);

  await preview.locator('.dashboard-card-checkbox').check();
  await clickAction(page, 'Archive');
  await expectToast(page, 'archived');
  await expect(preview.locator('.archive-badge')).toBeVisible();

  await preview.locator('.dashboard-card-checkbox').check();
  await clickAction(page, 'Unarchive');
  await expectToast(page, 'unarchived');
  await expect(preview.locator('.archive-badge')).toHaveCount(0);

  await preview.locator('a').first().click();
  await expect(page.locator('.bingo-grid--archive')).toHaveCount(0);
  await expect(page.locator('.finalized-card-view')).toBeVisible();
});
