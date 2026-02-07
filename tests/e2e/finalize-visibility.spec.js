const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
  expectToast,
  sendFriendRequest,
  respondToFriendRequest,
  waitForFriendInList,
} = require('./helpers');

test('finalize modal visibility checkbox controls friend access', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'fvisa');
  const userB = buildUser(testInfo, 'fvisb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });
  await createCardFromAuthenticatedCreate(pageA, { title: 'Private at Finalize' });
  await fillCardWithSuggestions(pageA);
  await finalizeCard(pageA, { visibleToFriends: false });
  await expect(pageA.locator('.visibility-toggle-btn')).toContainText('Private');

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageB, userA.username);

  await respondToFriendRequest(pageA, userB.username, 'accept');
  await expectToast(pageA, 'Friend request accepted');

  const friendRow = await waitForFriendInList(pageB, userA.username, { timeout: 20000 });
  await friendRow.getByRole('link', { name: 'View Card' }).click();
  await expect(pageB.getByRole('heading', { name: 'No Cards Available' })).toBeVisible();

  await contextA.close();
  await contextB.close();
});
