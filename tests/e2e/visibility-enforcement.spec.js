const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
  expectToast,
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

  await pageB.goto('/friends');
  await pageB.fill('#friend-search', userA.username);
  await pageB.click('#search-btn');
  const results = pageB.locator('#search-results');
  await expect(results).toContainText(userA.username);
  await results.getByRole('button', { name: 'Add Friend' }).click();

  await pageA.goto('/friends');
  await expect(pageA.locator('#friend-requests')).toBeVisible();
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();
  await expectToast(pageA, 'Friend request accepted');

  await pageB.reload();
  await pageB.goto('/friends');
  const friendRow = pageB.locator('#friends-list .friend-item').filter({ hasText: userA.username });
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
