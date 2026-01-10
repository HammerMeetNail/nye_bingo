const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createFinalizedCardFromModal,
  expectToast,
} = require('./helpers');

test('account delete invalidates share links', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'accountshare');
  await register(page, user);

  await createFinalizedCardFromModal(page, { title: 'Share Me' });
  await page.getByTitle('Share card').click();
  await expect(page.locator('#share-modal-content')).toBeVisible();
  await page.getByRole('button', { name: 'Enable Sharing' }).click();
  const shareInput = page.locator('#share-link-input');
  await expect(shareInput).toHaveValue(/#share\//);
  const shareUrl = await shareInput.inputValue();
  await page.keyboard.press('Escape');

  await page.goto('/#profile');
  await page.getByRole('button', { name: 'Delete Account' }).click();
  await page.locator('#delete-account-username').fill(user.username);
  await page.locator('#delete-account-password').fill(user.password);
  await page.locator('#delete-account-confirm').check();
  await page.locator('#delete-account-submit').click();
  await expectToast(page, 'Account deleted');

  await page.goto(shareUrl);
  await expect(page.getByRole('heading', { name: 'Share Link Not Found' })).toBeVisible();
});
