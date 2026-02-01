const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromModal,
  fillCardWithSuggestions,
  finalizeCard,
  sendFriendRequest,
  expectToast,
} = require('./helpers');

test('bingo notifications only fire once per card', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'nbinga');
  const userB = buildUser(testInfo, 'nbingb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageB, userA.username);
  await pageA.goto('/friends');
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();
  await expectToast(pageA, 'Friend request accepted');

  await createCardFromModal(pageA, { title: 'Quick Bingo', gridSize: 2, hasFree: false });
  await fillCardWithSuggestions(pageA);
  await finalizeCard(pageA);

  const cells = pageA.locator('.bingo-cell:not(.bingo-cell--free)');
  const cellCount = await cells.count();
  const gridSize = Math.round(Math.sqrt(cellCount));
  for (let i = 0; i < gridSize; i += 1) {
    await cells.nth(i).click();
    await pageA.getByRole('button', { name: 'Mark Complete' }).click();
  }

  // Wait for the bingo notification to be created server-side, then check
  const bingoMessages = pageB.locator('.notification-message').filter({ hasText: 'got a bingo' });
  await expect(async () => {
    await pageB.goto('/notifications');
    await expect(bingoMessages).toHaveCount(1);
  }).toPass({ timeout: 15000 });

  await pageA.locator('.bingo-cell--completed').first().click();
  await pageA.getByRole('button', { name: 'Mark Incomplete' }).click();
  await cells.nth(0).click();
  await pageA.getByRole('button', { name: 'Mark Complete' }).click();

  await pageB.reload();
  await expect(pageB.locator('.notification-message').filter({ hasText: 'got a bingo' })).toHaveCount(1, { timeout: 15000 });

  await contextA.close();
  await contextB.close();
});

