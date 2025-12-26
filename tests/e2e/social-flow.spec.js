const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
  completeFirstItem,
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

  await pageB.goto('/#friends');
  await pageB.fill('#friend-search', userA.username);
  await pageB.click('#search-btn');
  const results = pageB.locator('#search-results');
  await expect(results).toContainText(userA.username);
  await results.getByRole('button', { name: 'Add Friend' }).click();
  await expect(pageB.locator('#sent-requests')).toContainText(userA.username);

  await pageA.goto('/#friends');
  await expect(pageA.locator('#friend-requests')).toBeVisible();
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();
  await expect(pageA.locator('#friends-list')).toContainText(userB.username);

  await pageB.reload();
  await pageB.goto('/#friends');
  await expect(pageB.locator('#friends-list')).toContainText(userA.username, { timeout: 15000 });
  const friendRow = pageB.locator('#friends-list .friend-item').filter({ hasText: userA.username });
  await friendRow.getByRole('link', { name: 'View Card' }).click();
  await expect(pageB.locator('.finalized-card-view')).toBeVisible();

  await pageB.locator('.bingo-cell--completed').first().click();
  await expect(pageB.getByRole('heading', { name: 'Completed Goal' })).toBeVisible();
  await pageB.locator('.emoji-btn').first().click();
  await expect(pageB.locator('.reaction-badge').first()).toBeVisible();

  await contextA.close();
  await contextB.close();
});
