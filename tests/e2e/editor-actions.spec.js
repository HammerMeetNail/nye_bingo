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

test('double-submit on add goal only creates one item', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'dblsave');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page, { title: 'Double Save' });

  await page.locator('.bingo-cell--empty').first().click();
  await expect(page.locator('#modal-title')).toHaveText('Add Goal');
  await page.fill('textarea[id^="edit-item-content-"]', 'Walk the neighborhood loop');

  let postCount = 0;
  page.on('request', (request) => {
    if (request.method() === 'POST' && /\/api\/cards\/[^/]+\/items$/.test(request.url())) {
      postCount += 1;
    }
  });

  await page.evaluate(() => {
    const form = document.querySelector('form[data-action="save-item-edit"]');
    if (!form) return;
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  });

  await expectToast(page, 'Goal added');
  expect(postCount).toBe(1);
  await expect(page.locator('.bingo-cell[data-item-id]')).toHaveCount(1);
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
