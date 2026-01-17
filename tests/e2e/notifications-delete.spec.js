const { test, expect } = require('@playwright/test');
const { buildUser, register, createFinalizedCardFromModal, sendFriendRequest } = require('./helpers');

test('notifications can be deleted individually or all at once', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'ndela');
  const userB = buildUser(testInfo, 'ndelb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageA, userB.username);

  await pageB.goto('/friends');
  await pageB.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();

  await createFinalizedCardFromModal(pageA, { title: 'Delete Me' });

  await pageB.goto('/notifications');
  await expect(pageB.locator('.notification-item')).toHaveCount(2);

  const deleteButtons = pageB.getByRole('button', { name: 'Delete notification' });
  await deleteButtons.first().click();
  await expect(pageB.locator('.notification-item')).toHaveCount(1);

  pageB.once('dialog', (dialog) => dialog.accept());
  await pageB.getByRole('button', { name: 'Delete all' }).click();
  await expect(pageB.locator('.notification-item')).toHaveCount(0);
  await expect(pageB.locator('.notifications-list')).toContainText('No notifications yet.');

  await contextA.close();
  await contextB.close();
});

