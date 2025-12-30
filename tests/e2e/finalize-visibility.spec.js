const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
  expectToast,
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

  await pageB.goto('/#friends');
  await pageB.fill('#friend-search', userA.username);
  await pageB.click('#search-btn');
  const results = pageB.locator('#search-results');
  await expect(results).toContainText(userA.username);
  await results.getByRole('button', { name: 'Add Friend' }).click();
  await expectToast(pageB, 'Friend request sent!');

  await pageA.goto('/#friends');
  await expect(pageA.locator('#friend-requests')).toBeVisible();
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();
  await expectToast(pageA, 'Friend request accepted');

  await pageB.reload();
  await pageB.goto('/#friends');
  const friendRow = pageB.locator('#friends-list .friend-item').filter({ hasText: userA.username });
  await friendRow.getByRole('link', { name: 'View Card' }).click();
  await expect(pageB.getByRole('heading', { name: 'No Cards Available' })).toBeVisible();

  await contextA.close();
  await contextB.close();
});
