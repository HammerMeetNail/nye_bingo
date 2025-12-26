const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
} = require('./helpers');

test('clear all removes draft items', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'clear');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page, { title: 'Clear Card' });

  await page.fill('#item-input', 'Clear me');
  await page.click('#add-btn');
  await page.fill('#item-input', 'Another goal');
  await page.click('#add-btn');

  await page.locator('#clear-btn').click();
  const modal = page.locator('#modal-overlay');
  await expect(modal).toHaveClass(/modal-overlay--visible/);
  await modal.getByRole('button', { name: 'Clear All' }).click();

  await expect(page.locator('.bingo-cell:not(.bingo-cell--free):not(.bingo-cell--empty)')).toHaveCount(0);
  await expect(page.locator('.progress-text')).toContainText('0/');
});

test('full draft warns before leaving without finalizing', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'warn');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page, { title: 'Warning Card' });

  await page.locator('#fill-empty-btn').click();
  await expect(page.locator('.progress-text')).toContainText('/24 items added');

  await page.getByRole('link', { name: 'Friends' }).click();
  await expect(page.locator('#modal-title')).toHaveText('Card Not Finalized');
  await page.getByRole('button', { name: 'Stay' }).click();
  await expect(page.locator('#item-input')).toBeVisible();

  await page.getByRole('link', { name: 'Friends' }).click();
  await expect(page.locator('#modal-title')).toHaveText('Card Not Finalized');
  await page.getByRole('button', { name: 'Leave Anyway' }).click();
  await expect(page.getByRole('heading', { name: 'Friends', level: 2 })).toBeVisible();
});
