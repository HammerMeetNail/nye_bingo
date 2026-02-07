const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
  completeFirstItem,
  sendFriendRequest,
  respondToFriendRequest,
  waitForFriendInList,
} = require('./helpers');

test('users can connect and react to friend cards', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'alpha');
  const userB = buildUser(testInfo, 'beta');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });
  await createCardFromAuthenticatedCreate(pageA, { title: 'Alpha Card' });
  await fillCardWithSuggestions(pageA);
  await finalizeCard(pageA);
  await completeFirstItem(pageA);
  await expect(pageA.locator('.progress-text')).toContainText('1/24 completed');

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageB, userA.username);

  await respondToFriendRequest(pageA, userB.username, 'accept');
  await expect(pageA.locator('#friends-list')).toContainText(userB.username);

  const friendRow = await waitForFriendInList(pageB, userA.username, { timeout: 20000 });
  await friendRow.getByRole('link', { name: 'View Card' }).click();
  await expect(pageB.locator('.finalized-card-view')).toBeVisible();

  await pageB.locator('.bingo-cell--completed').first().click();
  await expect(pageB.getByRole('heading', { name: 'Completed Goal' })).toBeVisible();
  await pageB.locator('.emoji-btn').first().click();
  await expect(pageB.locator('.reaction-badge').first()).toBeVisible();

  await contextA.close();
  await contextB.close();
});
