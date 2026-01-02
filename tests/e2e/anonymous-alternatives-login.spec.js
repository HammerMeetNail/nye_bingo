const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  logout,
  expectToast,
} = require('./helpers');

test('anonymous user can log in to save a card', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'anonlogin');
  await register(page, user);
  await logout(page);

  await page.goto('/#create');
  await page.fill('#card-title', 'Anon Login Card');
  await page.selectOption('#card-grid-size', '3');
  await page.fill('#card-header', 'ABC');
  await page.getByRole('button', { name: 'Create Card' }).click();

  await expect(page.locator('#item-input')).toBeVisible();
  await page.locator('#fill-empty-btn').click();
  await expect(page.locator('.editor-actions').getByRole('button', { name: /Finalize Card/i })).toBeEnabled();
  await page.locator('.editor-actions').getByRole('button', { name: /Finalize Card/i }).click();

  const modal = page.locator('#modal-overlay');
  await expect(modal).toHaveClass(/modal-overlay--visible/);
  await modal.getByRole('button', { name: 'I Already Have an Account' }).click();

  await page.fill('#finalize-login-email', user.email);
  await page.fill('#finalize-login-password', user.password);
  await page.getByRole('button', { name: 'Login & Save Card' }).click();

  await expect(page.locator('.finalized-card-view')).toBeVisible();
  await expectToast(page, 'Card saved and finalized');
});

