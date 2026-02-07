const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  expectToast,
  sendFriendRequest,
  respondToFriendRequest,
  waitForFriendInList,
} = require('./helpers');

test('invite links connect friends', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'inva');
  const userB = buildUser(testInfo, 'invb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await pageA.goto('/friends');
  await pageA.getByRole('button', { name: 'Create Invite Link' }).click();
  const inviteInput = pageA.locator('#invite-result input');
  await expect(inviteInput).toBeVisible();
  const inviteLink = await inviteInput.inputValue();

  await pageB.goto(inviteLink);
  await expect(pageB.getByRole('heading', { name: "You're friends now!" })).toBeVisible();
  await pageB.getByRole('link', { name: 'Go to Friends' }).click();
  await expect(pageB.locator('#friends-list')).toContainText(userA.username);

  await pageA.reload();
  await expect(pageA.locator('#friends-list')).toContainText(userB.username);

  await contextA.close();
  await contextB.close();
});

test('blocking removes friendships and hides search results', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'blka');
  const userB = buildUser(testInfo, 'blkb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageB, userA.username);

  await respondToFriendRequest(pageA, userB.username, 'accept');
  await expectToast(pageA, 'Friend request accepted');

  const friendRow = await waitForFriendInList(pageA, userB.username, { timeout: 20000 });
  pageA.once('dialog', (dialog) => dialog.accept());
  await friendRow.getByRole('button', { name: 'Block' }).click();
  await expectToast(pageA, 'User blocked');
  await expect(pageA.locator('#blocked-users')).toContainText(userB.username);

  await pageB.reload();
  await expect(pageB.locator('#friends-list')).toContainText('No friends yet');
  await pageB.fill('#friend-search', userA.username);
  await pageB.click('#search-btn');
  await expect(pageB.locator('#search-results')).toContainText('No users found');

  await contextA.close();
  await contextB.close();
});
