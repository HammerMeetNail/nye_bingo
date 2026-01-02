const { test, expect } = require('@playwright/test');
const { buildUser, register, sendFriendRequest } = require('./helpers');

test('viewing notifications marks them read', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'nreqa');
  const userB = buildUser(testInfo, 'nreqb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageA, userB.username);

  await pageB.goto('/#notifications');
  const notification = pageB.locator('.notification-item', { hasText: 'friend request' });
  await expect(notification).toBeVisible();
  await expect(notification).not.toHaveClass(/notification-item--unread/);
  await expect(pageB.locator('#notification-badge')).toHaveClass(/nav-badge--hidden/);

  await contextA.close();
  await contextB.close();
});

