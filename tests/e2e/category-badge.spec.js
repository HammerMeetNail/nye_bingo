const { test, expect } = require('@playwright/test');
const { buildUser, register, expectToast } = require('./helpers');

test('card category badge persists on dashboard and editor', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'cat');
  await register(page, user);

  const title = `Travel Badge ${Date.now()}`;
  await page.selectOption('#card-category', 'travel');
  await page.fill('#card-title', title);
  await page.locator('#create-card-form button[type="submit"]').click();
  await expectToast(page, 'created');

  await expect(page.locator('.category-badge')).toContainText(/Travel/i);

  await page.goto('/dashboard');
  const preview = page.locator('.dashboard-card-preview').filter({ hasText: title });
  await expect(preview).toBeVisible();
  await expect(preview.locator('.category-badge')).toContainText(/Travel/i);

  await page.reload();
  const previewAfter = page.locator('.dashboard-card-preview').filter({ hasText: title });
  await expect(previewAfter).toBeVisible();
  await expect(previewAfter.locator('.category-badge')).toContainText(/Travel/i);

  await previewAfter.locator('a').first().click();
  await expect(page.locator('#item-input')).toBeVisible();
  await expect(page.locator('.category-badge')).toContainText(/Travel/i);
});

