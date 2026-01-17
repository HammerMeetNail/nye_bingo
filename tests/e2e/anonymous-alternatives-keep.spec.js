const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  logout,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
  expectToast,
} = require('./helpers');

test('anonymous card conflicts can keep the existing card', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'anonkeep');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page);
  await fillCardWithSuggestions(page);
  await finalizeCard(page);
  await logout(page);

  await page.goto('/create');
  await page.selectOption('#card-grid-size', '3');
  await page.getByRole('button', { name: 'Create Card' }).click();
  await expect(page.locator('#item-input')).toBeVisible();
  await page.locator('#fill-empty-btn').click();
  await page.locator('.editor-actions').getByRole('button', { name: /Finalize Card/i }).click();

  const modal = page.locator('#modal-overlay');
  await expect(modal).toHaveClass(/modal-overlay--visible/);
  await modal.getByRole('button', { name: 'I Already Have an Account' }).click();

  await page.fill('#finalize-login-email', user.email);
  await page.fill('#finalize-login-password', user.password);
  await page.getByRole('button', { name: 'Login & Save Card' }).click();

  await expect(page.locator('#modal-title')).toHaveText('Card Already Exists');
  await page.getByRole('button', { name: 'Keep Existing Card' }).click();

  await expectToast(page, 'Keeping your existing card');
  await expect(page.locator('.finalized-card-view')).toBeVisible();
});

