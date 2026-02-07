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

test('private cards are hidden from friends', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'visa');
  const userB = buildUser(testInfo, 'visb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });
  await createCardFromAuthenticatedCreate(pageA, { title: 'Visible Card' });
  await fillCardWithSuggestions(pageA);
  await finalizeCard(pageA);

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageB, userA.username);

  await respondToFriendRequest(pageA, userB.username, 'accept');
  await expectToast(pageA, 'Friend request accepted');

  const friendRow = await waitForFriendInList(pageB, userA.username, { timeout: 20000 });
  await friendRow.getByRole('link', { name: 'View Card' }).click();
  await expect(pageB.locator('.finalized-card-view')).toBeVisible();

  await pageA.goto('/dashboard');
  await pageA.locator('.dashboard-card-preview').first().locator('a').first().click();
  const visibilityButton = pageA.locator('.visibility-toggle-btn');
  await visibilityButton.click();
  await expectToast(pageA, 'Card is now private');

  await pageB.goto('/friends');
  await friendRow.getByRole('link', { name: 'View Card' }).click();
  await expect(pageB.getByRole('heading', { name: 'No Cards Available' })).toBeVisible();

  await contextA.close();
  await contextB.close();
});
