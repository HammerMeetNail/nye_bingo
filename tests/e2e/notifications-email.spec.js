const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  sendFriendRequest,
  waitForEmail,
  expectNoEmail,
  extractTokenFromEmail,
} = require('./helpers');

test('email notifications respect opt-in', async ({ browser, request }, testInfo) => {
  const userB = buildUser(testInfo, 'nemailb');
  const userA = buildUser(testInfo, 'nemaila');
  const userC = buildUser(testInfo, 'nemailc');

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await pageB.goto('/profile');
  const afterVerify = Date.now();
  await pageB.getByRole('button', { name: 'Resend verification email' }).click();

  const verifyMessage = await waitForEmail(request, {
    to: userB.email,
    subject: 'Verify your Year of Bingo account',
    after: afterVerify,
  });
  const token = extractTokenFromEmail(verifyMessage, 'verify-email');
  await pageB.goto(`/verify-email?token=${token}`);
  await expect(pageB.getByRole('heading', { name: 'Email Verified!' })).toBeVisible();

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const afterDefault = Date.now();
  await sendFriendRequest(pageA, userB.username);
  await expectNoEmail(request, { to: userB.email, subject: 'New friend request', after: afterDefault });

  await pageB.goto('/profile');
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
  expect(body).toMatch(/\/(notifications|friends)/);

  await contextA.close();
  await contextB.close();
  await contextC.close();
});
