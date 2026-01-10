const { test, expect } = require('@playwright/test');
const { buildUser, register, expectToast } = require('./helpers');

test('account delete logs out and blocks login', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'accountdelete');
  await register(page, user);

  await page.goto('/#profile');
  await page.getByRole('button', { name: 'Delete Account' }).click();

  await page.locator('#delete-account-username').fill(user.username);
  await page.locator('#delete-account-password').fill(user.password);
  await page.locator('#delete-account-confirm').check();
  await page.locator('#delete-account-submit').click();

  await expectToast(page, 'Account deleted');
  await expect(page.getByRole('heading', { name: 'Year of Bingo' })).toBeVisible();

  await page.goto('/#login');
  await page.locator('#login-form #email').fill(user.email);
  await page.locator('#login-form #password').fill(user.password);
  await page.getByRole('button', { name: 'Sign In' }).click();
  await expect(page.locator('#login-error')).toContainText('Invalid email or password');
});
