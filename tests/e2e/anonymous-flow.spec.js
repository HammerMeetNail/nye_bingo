const { test, expect } = require('@playwright/test');
const { buildUser } = require('./helpers');

test('anonymous user can create, shuffle, and save a card', async ({ page }, testInfo) => {
  await page.goto('/#create');

  await page.fill('#card-title', 'Anon Card');
  await page.selectOption('#card-grid-size', '3');
  await page.fill('#card-header', 'ABC');
  await page.getByRole('button', { name: 'Create Card' }).click();

  await expect(page.locator('#item-input')).toBeVisible();
  await page.fill('#item-input', 'Goal one');
  await page.click('#add-btn');
  await page.fill('#item-input', 'Goal two');
  await page.click('#add-btn');

  const freeBefore = await page.locator('.bingo-cell--free').getAttribute('data-position');
  await page.getByRole('button', { name: /Shuffle/ }).click();
  await page.waitForSelector('.bingo-cell--shuffling', { state: 'attached' });
  await page.waitForSelector('.bingo-cell--shuffling', { state: 'detached' });
  const freeAfter = await page.locator('.bingo-cell--free').getAttribute('data-position');
  expect(freeAfter).toBe(freeBefore);

  await page.reload();
  await expect(page.locator('#item-input')).toBeVisible();
  await expect(page.locator('.bingo-cell').filter({ hasText: 'Goal one' })).toBeVisible();

  await page.locator('#fill-empty-btn').click();
  await expect(page.locator('.editor-actions').getByRole('button', { name: /Finalize Card/i })).toBeEnabled();
  await page.locator('.editor-actions').getByRole('button', { name: /Finalize Card/i }).click();

  const modal = page.locator('#modal-overlay');
  await expect(modal).toHaveClass(/modal-overlay--visible/);
  await modal.getByRole('button', { name: 'Create Account' }).click();

  const user = buildUser(testInfo, 'anon');
  await page.fill('#finalize-username', user.username);
  await page.fill('#finalize-email', user.email);
  await page.fill('#finalize-password', user.password);
  await page.getByRole('button', { name: 'Create Account & Save Card' }).click();

  await expect(page.locator('.finalized-card-view')).toBeVisible();
  await expect(page.locator('.progress-text')).toContainText('/8 completed');
  await expect(page.getByRole('link', { name: 'My Cards' })).toBeVisible();
});
