const { test, expect } = require('@playwright/test');
const { buildUser, register, sendFriendRequest } = require('./helpers');

test('notification rendering escapes usernames', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'nxssa', { username: '<img src=x onerror=alert(1)>' });
  const userB = buildUser(testInfo, 'nxssb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageA, userB.username);

  await pageB.goto('/#notifications');
  const message = pageB.locator('.notification-message').first();
  await expect(message).toContainText('<img src=x onerror=alert(1)>');
  await expect(message.locator('img')).toHaveCount(0);

  await contextA.close();
  await contextB.close();
});

