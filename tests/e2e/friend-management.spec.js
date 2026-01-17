const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  expectToast,
} = require('./helpers');

test('friend requests can be canceled and rejected', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'reqa');
  const userB = buildUser(testInfo, 'reqb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await pageB.goto('/friends');
  await pageB.fill('#friend-search', userA.username);
  await pageB.click('#search-btn');
  const results = pageB.locator('#search-results');
  await expect(results).toContainText(userA.username);
  await results.getByRole('button', { name: 'Add Friend' }).click();
  await expect(pageB.locator('#sent-requests')).toContainText(userA.username);

  await pageB.locator('#sent-list .friend-item').getByRole('button', { name: 'Cancel' }).click();
  await expectToast(pageB, 'Friend request canceled');
  await expect(pageB.locator('#sent-requests')).toBeHidden();

  await pageA.goto('/friends');
  await expect(pageA.locator('#friend-requests')).toBeHidden();

  await pageB.goto('/friends');
  await pageB.fill('#friend-search', userA.username);
  await pageB.click('#search-btn');
  await results.getByRole('button', { name: 'Add Friend' }).click();
  await expect(pageB.locator('#sent-requests')).toContainText(userA.username);

  await pageA.reload();
  await expect(pageA.locator('#friend-requests')).toBeVisible();
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Reject' }).click();
  await expectToast(pageA, 'Friend request rejected');
  await expect(pageA.locator('#friend-requests')).toBeHidden();

  await pageB.reload();
  await expect(pageB.locator('#sent-requests')).toBeHidden();

  await contextA.close();
  await contextB.close();
});

test('friends can be removed after connecting', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'rema');
  const userB = buildUser(testInfo, 'remb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await pageB.goto('/friends');
  await pageB.fill('#friend-search', userA.username);
  await pageB.click('#search-btn');
  const results = pageB.locator('#search-results');
  await expect(results).toContainText(userA.username);
  await results.getByRole('button', { name: 'Add Friend' }).click();

  await pageA.goto('/friends');
  await expect(pageA.locator('#friend-requests')).toBeVisible();
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();
  await expectToast(pageA, 'Friend request accepted');

  await pageB.reload();
  await expect(pageB.locator('#friends-list')).toContainText(userA.username, { timeout: 15000 });
  const friendRow = pageB.locator('#friends-list .friend-item').filter({ hasText: userA.username });
  pageB.once('dialog', (dialog) => dialog.accept());
  await friendRow.getByRole('button', { name: 'Remove' }).click();
  await expectToast(pageB, 'Friend removed');

  await expect(pageB.locator('#friends-list')).toContainText('No friends yet');

  await pageA.reload();
  await expect(pageA.locator('#friends-list')).toContainText('No friends yet');

  await contextA.close();
  await contextB.close();
});
