const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  expectToast,
  sendFriendRequest,
  cancelSentFriendRequest,
  respondToFriendRequest,
  waitForFriendInList,
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

  await sendFriendRequest(pageB, userA.username);

  await cancelSentFriendRequest(pageB, userA.username);
  await expectToast(pageB, 'Friend request canceled');
  await expect(pageB.locator('#sent-requests')).toBeHidden();

  await pageA.goto('/friends');
  await expect(pageA.locator('#friend-requests')).toBeHidden();

  await sendFriendRequest(pageB, userA.username);

  await respondToFriendRequest(pageA, userB.username, 'reject');
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

  await sendFriendRequest(pageB, userA.username);

  await respondToFriendRequest(pageA, userB.username, 'accept');
  await expectToast(pageA, 'Friend request accepted');

  const friendRow = await waitForFriendInList(pageB, userA.username, { timeout: 20000 });
  pageB.once('dialog', (dialog) => dialog.accept());
  await friendRow.getByRole('button', { name: 'Remove' }).click();
  await expectToast(pageB, 'Friend removed');

  await expect(pageB.locator('#friends-list')).toContainText('No friends yet');

  await pageA.reload();
  await expect(pageA.locator('#friends-list')).toContainText('No friends yet');

  await contextA.close();
  await contextB.close();
});
