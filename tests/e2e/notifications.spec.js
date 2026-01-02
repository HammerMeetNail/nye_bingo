const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createFinalizedCardFromModal,
  createCardFromModal,
  fillCardWithSuggestions,
  finalizeCard,
  clearMailpit,
  waitForEmail,
  expectNoEmail,
  extractTokenFromEmail,
} = require('./helpers');

async function sendFriendRequest(page, username) {
  await page.goto('/#friends');
  await page.fill('#friend-search', username);
  await page.click('#search-btn');
  const results = page.locator('#search-results');
  await expect(results).toContainText(username);
  await results.getByRole('button', { name: 'Add Friend' }).click();
}

test('viewing notifications marks them read', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'nreqa');
  const userB = buildUser(testInfo, 'nreqb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageA, userB.username);

  await pageB.goto('/#notifications');
  const notification = pageB.locator('.notification-item', { hasText: 'friend request' });
  await expect(notification).toBeVisible();
  await expect(notification).not.toHaveClass(/notification-item--unread/);
  await expect(pageB.locator('#notification-badge')).toHaveClass(/nav-badge--hidden/);

  await contextA.close();
  await contextB.close();
});

test('notifications can be deleted individually or all at once', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'ndela');
  const userB = buildUser(testInfo, 'ndelb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageA, userB.username);

  await pageB.goto('/#friends');
  await pageB.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();

  await createFinalizedCardFromModal(pageA, { title: 'Delete Me' });

  await pageB.goto('/#notifications');
  await expect(pageB.locator('.notification-item')).toHaveCount(2);

  const deleteButtons = pageB.getByRole('button', { name: 'Delete notification' });
  await deleteButtons.first().click();
  await expect(pageB.locator('.notification-item')).toHaveCount(1);

  pageB.once('dialog', (dialog) => dialog.accept());
  await pageB.getByRole('button', { name: 'Delete all' }).click();
  await expect(pageB.locator('.notification-item')).toHaveCount(0);
  await expect(pageB.locator('.notifications-list')).toContainText('No notifications yet.');

  await contextA.close();
  await contextB.close();
});

test('friend acceptance notifications are delivered', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'nacca');
  const userB = buildUser(testInfo, 'naccb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageB, userA.username);

  await pageA.goto('/#friends');
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();

  await pageB.goto('/#notifications');
  await expect(pageB.locator('.notification-message')).toContainText('accepted your friend request');

  await contextA.close();
  await contextB.close();
});

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
  await pageA.goto('/#friends');
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();

  await createFinalizedCardFromModal(pageA, { title: 'New Friends Card' });

  await pageB.goto('/#notifications');
  const newCardNotification = pageB.locator('.notification-item', { hasText: 'created a new card' });
  await expect(newCardNotification).toHaveCount(1);
  await newCardNotification.getByRole('link', { name: 'View' }).click();
  await expect(pageB.locator('.finalized-card-view')).toBeVisible();

  await contextA.close();
  await contextB.close();
});

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
  await pageA.goto('/#friends');
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();

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

  await pageB.goto('/#notifications');
  const bingoMessages = pageB.locator('.notification-message', { hasText: 'got a bingo' });
  await expect(bingoMessages).toHaveCount(1);

  await pageA.locator('.bingo-cell--completed').first().click();
  await pageA.getByRole('button', { name: 'Mark Incomplete' }).click();
  await cells.nth(0).click();
  await pageA.getByRole('button', { name: 'Mark Complete' }).click();

  await pageB.reload();
  await expect(pageB.locator('.notification-message', { hasText: 'got a bingo' })).toHaveCount(1);

  await contextA.close();
  await contextB.close();
});

test('notification rendering escapes usernames', async ({ browser }, testInfo) => {
  const userA = buildUser(testInfo, 'nxssa', { username: '<img src=x onerror=alert(1)>' });
  const userB = buildUser(testInfo, 'nxssb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await sendFriendRequest(pageA, userB.username);

  await pageB.goto('/#notifications');
  const message = pageB.locator('.notification-message').first();
  await expect(message).toContainText('<img src=x onerror=alert(1)>');
  await expect(message.locator('img')).toHaveCount(0);

  await contextA.close();
  await contextB.close();
});

test('email notifications respect opt-in', async ({ browser, request }, testInfo) => {
  await clearMailpit(request);

  const userB = buildUser(testInfo, 'nemailb');
  const userA = buildUser(testInfo, 'nemaila');
  const userC = buildUser(testInfo, 'nemailc');

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await pageB.goto('/#profile');
  const afterVerify = Date.now();
  await pageB.getByRole('button', { name: 'Resend verification email' }).click();

  const verifyMessage = await waitForEmail(request, {
    to: userB.email,
    subject: 'Verify your Year of Bingo account',
    after: afterVerify,
  });
  const token = extractTokenFromEmail(verifyMessage, 'verify-email');
  await pageB.goto(`/#verify-email?token=${token}`);
  await expect(pageB.getByRole('heading', { name: 'Email Verified!' })).toBeVisible();

  await clearMailpit(request);

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const afterDefault = Date.now();
  await sendFriendRequest(pageA, userB.username);
  await expectNoEmail(request, { to: userB.email, subject: 'New friend request', after: afterDefault });

  await pageB.goto('/#profile');
  const emailToggle = pageB.locator('#notify-email-enabled');
  await expect(emailToggle).toBeEnabled();
  if (!(await emailToggle.isChecked())) {
    await emailToggle.check();
  }
  const scenarioToggle = pageB.locator('[data-setting="email_friend_request_received"]');
  if (!(await scenarioToggle.isChecked())) {
    await scenarioToggle.check();
  }

  const contextC = await browser.newContext();
  const pageC = await contextC.newPage();
  await register(pageC, userC, { searchable: true });

  const afterOptIn = Date.now();
  await sendFriendRequest(pageC, userB.username);

  const message = await waitForEmail(request, {
    to: userB.email,
    subject: 'New friend request',
    after: afterOptIn,
  });

  const body = message.Text || message.text || message.HTML || message.html || message.Body || message.body || '';
  expect(body).toMatch(/#notifications|#friends/);

  await contextA.close();
  await contextB.close();
  await contextC.close();
});
