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

test('anonymous card conflicts can keep the existing card', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'anonkeep');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page);
  await fillCardWithSuggestions(page);
  await finalizeCard(page);
  await logout(page);

  await page.goto('/#create');
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

test('anonymous card conflicts can replace the existing card', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'anonreplace');
  await register(page, user);

  const conflictYear = new Date().getFullYear() + 1;
  const conflictTitle = 'Replacement Card';
  await page.goto('/#create');
  await page.selectOption('#card-year', String(conflictYear));
  await page.fill('#card-title', conflictTitle);
  await page.getByRole('button', { name: 'Create Card' }).click();
  await page.fill('#item-input', 'Original Only Goal');
  await page.click('#add-btn');
  await page.locator('#fill-empty-btn').click();
  await finalizeCard(page);
  await logout(page);

  await page.goto('/#create');
  await page.selectOption('#card-year', String(conflictYear));
  await page.fill('#card-title', conflictTitle);
  await page.getByRole('button', { name: 'Create Card' }).click();
  await expect(page.locator('#item-input')).toBeVisible();
  await page.fill('#item-input', 'Anon Replacement Goal');
  await page.click('#add-btn');
  await page.locator('#fill-empty-btn').click();
  await page.locator('.editor-actions').getByRole('button', { name: /Finalize Card/i }).click();

  const modal = page.locator('#modal-overlay');
  await expect(modal).toHaveClass(/modal-overlay--visible/);
  await modal.getByRole('button', { name: 'I Already Have an Account' }).click();

  await page.fill('#finalize-login-email', user.email);
  await page.fill('#finalize-login-password', user.password);
  await page.getByRole('button', { name: 'Login & Save Card' }).click();

  await expect(page.locator('#modal-title')).toHaveText('Card Already Exists');
  page.once('dialog', (dialog) => dialog.accept());
  await page.getByRole('button', { name: 'Replace Existing Card' }).click();

  await expectToast(page, 'Card replaced and finalized');
  await expect(page.locator('.finalized-card-view')).toBeVisible();
  await expect(page.locator('.bingo-cell').filter({ hasText: 'Anon Replacement Goal' })).toBeVisible();
  await expect(page.locator('.bingo-cell').filter({ hasText: 'Original Only Goal' })).toHaveCount(0);
});
