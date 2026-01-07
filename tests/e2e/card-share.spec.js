const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
  expectToast,
} = require('./helpers');

test('share links show read-only cards and escape XSS goals', async ({ browser }, testInfo) => {
  const user = buildUser(testInfo, 'share');
  const xssGoal = `"<img src=x onerror=alert(1)>-${Date.now()}"`;

  const context = await browser.newContext();
  const page = await context.newPage();
  await register(page, user);
  await createCardFromAuthenticatedCreate(page);

  await expect(page.locator('#item-input')).toBeVisible();
  await page.fill('#item-input', xssGoal);
  await page.click('#add-btn');
  await fillCardWithSuggestions(page);
  await finalizeCard(page);

  await page.locator('[data-action="open-share-modal"]').click();
  await page.getByRole('button', { name: 'Enable Sharing' }).click();

  const shareInput = page.locator('#share-link-input');
  await expect(shareInput).toBeVisible();
  const shareLink = await shareInput.inputValue();
  expect(shareLink).toContain('#share/');

  const publicContext = await browser.newContext();
  const publicPage = await publicContext.newPage();
  await publicPage.goto(shareLink);

  await expect(publicPage.locator('.finalized-card-view')).toBeVisible();
  await expect(publicPage.locator('.badge').filter({ hasText: 'Shared view' })).toBeVisible();
  await expect(publicPage.locator('.bingo-cell-content').filter({ hasText: '<img' }).first()).toBeVisible();
  await expect(publicPage.locator('#bingo-grid img')).toHaveCount(0);
  await expect(publicPage.locator('[data-action="edit-card-meta"]')).toHaveCount(0);
  await expect(publicPage.locator('#item-input')).toHaveCount(0);

  await publicContext.close();
  await context.close();
});

test('revoked share links no longer work', async ({ browser }, testInfo) => {
  const user = buildUser(testInfo, 'revoke');

  const context = await browser.newContext();
  const page = await context.newPage();
  await register(page, user);
  await createCardFromAuthenticatedCreate(page);

  await fillCardWithSuggestions(page);
  await finalizeCard(page);

  await page.locator('[data-action="open-share-modal"]').click();
  await page.getByRole('button', { name: 'Enable Sharing' }).click();

  const shareInput = page.locator('#share-link-input');
  await expect(shareInput).toBeVisible();
  const shareLink = await shareInput.inputValue();

  page.once('dialog', (dialog) => dialog.accept());
  await page.getByRole('button', { name: 'Disable Sharing' }).click();
  await expectToast(page, 'Sharing disabled');

  const publicContext = await browser.newContext();
  const publicPage = await publicContext.newPage();
  await publicPage.goto(shareLink);
  await expect(publicPage.getByRole('heading', { name: 'Share Link Not Found' })).toBeVisible();

  await publicContext.close();
  await context.close();
});
