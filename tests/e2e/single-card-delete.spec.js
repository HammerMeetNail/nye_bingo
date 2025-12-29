const { test, expect } = require('@playwright/test');
const { buildUser, register, expectToast } = require('./helpers');

test('single card can be deleted from dashboard', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'del');
  await register(page, user);

  const title = `Delete Me ${Date.now()}`;
  await page.fill('#card-title', title);
  await Promise.all([
    page.waitForResponse((response) => {
      try {
        const url = new URL(response.url());
        return url.pathname === '/api/cards'
          && response.request().method() === 'POST'
          && response.status() === 201;
      } catch {
        return false;
      }
    }),
    page.locator('#create-card-form button[type="submit"]').click(),
  ]);
  await expectToast(page, `${title} created!`);
  await expect(page.locator('#item-input')).toBeVisible();

  await Promise.all([
    page.waitForResponse((response) => {
      try {
        const url = new URL(response.url());
        return url.pathname === '/api/cards' && response.request().method() === 'GET';
      } catch {
        return false;
      }
    }),
    page.goto('/#dashboard'),
  ]);
  await expect(page.getByRole('heading', { name: 'My Bingo Cards' })).toBeVisible();
  const preview = page.locator('.dashboard-card-preview').filter({ hasText: title });
  await expect(preview).toBeVisible();

  page.once('dialog', async (dialog) => {
    expect(dialog.type()).toBe('confirm');
    expect(dialog.message()).toContain(title);
    await dialog.accept();
  });
  await preview.getByRole('button', { name: 'Delete card' }).click();

  await expectToast(page, 'Card deleted');
  await expect(page.locator('.dashboard-card-preview').filter({ hasText: title })).toHaveCount(0);
});
