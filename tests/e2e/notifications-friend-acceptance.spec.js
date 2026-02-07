const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  sendFriendRequest,
  respondToFriendRequest,
} = require('./helpers');

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
  await respondToFriendRequest(pageA, userB.username, 'accept');

  const acceptedMessage = pageB.locator('.notification-message');
  await expect(async () => {
    await pageB.goto('/notifications');
    await expect(acceptedMessage).toContainText('accepted your friend request');
  }).toPass({ timeout: 15000 });

  await contextA.close();
  await contextB.close();
});
