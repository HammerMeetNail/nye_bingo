const { test, expect } = require('@playwright/test');
const { buildUser, register, createFinalizedCardFromModal, sendFriendRequest, expectToast } = require('./helpers');

test('new card notifications link to friend cards', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'ncarda');
  const userB = buildUser(testInfo, 'ncardb');

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

  await createFinalizedCardFromModal(pageA, { title: 'New Friends Card' });

  // Wait for the new card notification to be created server-side
  const newCardNotification = pageB.locator('.notification-item').filter({ hasText: 'created a new card' });
  await expect(async () => {
    await pageB.goto('/notifications');
    await expect(newCardNotification).toHaveCount(1);
  }).toPass({ timeout: 15000 });
  await newCardNotification.getByRole('link', { name: 'View' }).click();
  await expect(pageB.locator('.finalized-card-view')).toBeVisible();

  await contextA.close();
  await contextB.close();
});

