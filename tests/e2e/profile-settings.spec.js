const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  loginWithCredentials,
  expectToast,
} = require('./helpers');

test('profile searchable toggle controls friend search results', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'private');
  const userB = buildUser(testInfo, 'search');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA);
  await pageA.goto('/profile');
  const toggle = pageA.locator('#searchable-toggle');
  await expect(toggle).not.toBeChecked();

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });
  await pageB.goto('/friends');
  await pageB.fill('#friend-search', userA.username);
  await pageB.click('#search-btn');
  await expect(pageB.locator('#search-results')).toContainText('No users found');

  await toggle.check();
  await expectToast(pageA, 'You are now searchable');

  await pageB.fill('#friend-search', '');
  await pageB.fill('#friend-search', userA.username);
  await pageB.click('#search-btn');
  await expect(pageB.locator('#search-results')).toContainText(userA.username);

  await contextA.close();
  await contextB.close();
});

test('user can change password and log in with new password', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'password');
  await register(page, user);

  await page.goto('/profile');
  await page.fill('#current-password', user.password);
  await page.fill('#new-password', 'NewPass1');
  await page.fill('#confirm-password', 'NewPass2');
  await page.getByRole('button', { name: 'Update Password' }).click();
  await expect(page.locator('#password-error')).toContainText('New passwords do not match');

  await page.fill('#new-password', 'NewPass1');
  await page.fill('#confirm-password', 'NewPass1');
  await page.getByRole('button', { name: 'Update Password' }).click();
  await expectToast(page, 'Password updated successfully');

  await page.getByRole('button', { name: 'Sign Out' }).click();
  await loginWithCredentials(page, user.email, 'NewPass1');
});
