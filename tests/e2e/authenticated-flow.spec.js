const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
} = require('./helpers');

test('authenticated user can configure, finalize, and complete a card', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'auth');
  await register(page, user, { searchable: true });

  await createCardFromAuthenticatedCreate(page, { title: 'Primary Card' });

  await page.fill('#card-header-input', 'GOAL');
  await page.dispatchEvent('#card-header-input', 'change');
  await expect(page.locator('.bingo-header').first()).toHaveText('G');
  await expect(page.locator('.bingo-header').nth(3)).toHaveText('L');

  const freeToggle = page.locator('#card-free-toggle');
  if (await freeToggle.isChecked()) {
    await freeToggle.uncheck();
  }
  await expect(page.locator('.bingo-cell--free')).toHaveCount(0);

  await fillCardWithSuggestions(page);
  await finalizeCard(page);

  await expect(page.locator('#card-header-input')).toHaveCount(0);
  await expect(page.locator('.bingo-cell--free')).toHaveCount(0);
  await expect(page.locator('.progress-text')).toContainText('0/25 completed');

  await page.locator('.bingo-cell:not(.bingo-cell--free)').first().click();
  await page.getByRole('button', { name: 'Mark Complete' }).click();
  await expect(page.locator('.progress-text')).toContainText('1/25 completed');

  await page.locator('.bingo-cell--completed').first().click();
  await page.getByRole('button', { name: 'Mark Incomplete' }).click();
  await expect(page.locator('.progress-text')).toContainText('0/25 completed');
});

test('cloned cards can change grid size and header', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'clone');
  await register(page, user);

  await createCardFromAuthenticatedCreate(page, { title: 'Source Card' });
  await fillCardWithSuggestions(page);
  await finalizeCard(page);

  await page.getByTitle('Clone card').click();
  await expect(page.locator('#modal-title')).toHaveText('Clone Card');

  await page.fill('#clone-card-title', 'Cloned 4x4');
  await page.selectOption('#clone-card-grid-size', '4');
  await page.fill('#clone-card-header', 'TEST');
  const cloneFreeToggle = page.locator('#clone-card-free-space');
  if (await cloneFreeToggle.isChecked()) {
    await cloneFreeToggle.uncheck();
  }

  await page.getByRole('button', { name: 'Clone' }).click();
  await expect(page.locator('#item-input')).toBeVisible();
  await expect(page.locator('#card-header-input')).toHaveValue('TEST');
  await expect(page.locator('.bingo-header')).toHaveCount(4);
  await expect(page.locator('.bingo-cell--free')).toHaveCount(0);
});
