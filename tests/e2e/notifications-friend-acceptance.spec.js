const { test, expect } = require('@playwright/test');
const { buildUser, register, sendFriendRequest } = require('./helpers');

test('friend acceptance notifications are delivered', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'nacca');
  const userB = buildUser(testInfo, 'naccb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageB, userA.username);

  await pageA.goto('/friends');
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();

  await pageB.goto('/notifications');
  await expect(pageB.locator('.notification-message')).toContainText('accepted your friend request');

  await contextA.close();
  await contextB.close();
});

