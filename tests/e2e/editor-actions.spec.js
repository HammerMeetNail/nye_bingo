const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
  expectToast,
} = require('./helpers');

test('draft items can be edited and removed', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'edit');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page, { title: 'Draft Card' });

  await page.fill('#item-input', 'Original Goal');
  await page.click('#add-btn');
  await expect(page.locator('.progress-text')).toContainText('1/24 items added');

  await page.locator('.bingo-cell').filter({ hasText: 'Original Goal' }).click();
  await expect(page.locator('#modal-title')).toHaveText('Edit Goal');
  await page.fill('textarea[id^="edit-item-content-"]', 'Updated Goal');
  await page.getByRole('button', { name: 'Save' }).click();
  await expectToast(page, 'Goal updated');

  await page.locator('.bingo-cell').filter({ hasText: 'Updated Goal' }).click();
  await page.getByRole('button', { name: 'Remove' }).click();
  await expectToast(page, 'Item removed');
  await expect(page.locator('.progress-text')).toContainText('0/24 items added');
});

test('finalized card visibility can be toggled', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'visibility');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page, { title: 'Visibility Card' });
  await fillCardWithSuggestions(page);
  await finalizeCard(page);

  const visibilityButton = page.locator('.visibility-toggle-btn');
  await expect(visibilityButton).toContainText('Visible');
  await visibilityButton.click();
  await expectToast(page, 'Card is now private');
  await expect(visibilityButton).toContainText('Private');
  await visibilityButton.click();
  await expectToast(page, 'Card is now visible to friends');
});
