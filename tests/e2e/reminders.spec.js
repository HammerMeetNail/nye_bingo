const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromModal,
  fillCardWithSuggestions,
  finalizeCard,
  waitForEmail,
  extractTokenFromEmail,
  clearMailpit,
  expectToast,
} = require('./helpers');

async function verifyEmail(page, request, user) {
  await page.goto('/#profile');
  const afterVerify = Date.now();
  await page.getByRole('button', { name: 'Resend verification email' }).click();

  const verifyMessage = await waitForEmail(request, {
    to: user.email,
    subject: 'Verify your Year of Bingo account',
    after: afterVerify,
  });
  const token = extractTokenFromEmail(verifyMessage, 'verify-email');
  await page.goto(`/#verify-email?token=${token}`);
  await expect(page.getByRole('heading', { name: 'Email Verified!' })).toBeVisible();
}

async function enableReminders(page) {
  await page.goto('/#profile');
  const toggle = page.locator('#reminder-email-enabled');
  await expect(toggle).toBeEnabled();
  if (!(await toggle.isChecked())) {
    await toggle.check();
  }
}

test('reminder test email sends and arrives', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'remind');
  await clearMailpit(request);
  await register(page, user);

  await page.goto('/#dashboard');
  await createCardFromModal(page, { title: 'Reminder Card' });
  await fillCardWithSuggestions(page);
  await finalizeCard(page);
  const cardUrl = page.url();

  await verifyEmail(page, request, user);
  await clearMailpit(request);

  await enableReminders(page);
  await expect(page.getByRole('button', { name: 'Send test email' })).toBeEnabled();
  await page.getByRole('button', { name: 'Send test email' }).click();

  const message = await waitForEmail(request, {
    to: user.email,
    subject: 'check-in',
  });

  const body = message.Text || message.text || message.HTML || message.html || message.Body || message.body || '';
  expect(body).toContain('#profile');
});

test('reminder emails escape goal content', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'remxss');
  await clearMailpit(request);
  await register(page, user);

  await page.goto('/#dashboard');
  await createCardFromModal(page, { title: 'XSS Card', gridSize: 3 });
  await fillCardWithSuggestions(page);

  const xssGoal = '<img src=x onerror=alert(1)>';
  await page.locator('.bingo-cell[data-position="2"]').click();
  await expect(page.locator('#modal-title')).toHaveText('Edit Goal');
  await page.fill('textarea[id^="edit-item-content-"]', xssGoal);
  await page.getByRole('button', { name: 'Save' }).click();
  await expectToast(page, 'Goal updated');

  await finalizeCard(page);
  const cardUrl = page.url();

  await verifyEmail(page, request, user);
  await enableReminders(page);

  await page.goto(cardUrl);
  await expect(page.locator('.finalized-card-view')).toBeVisible();

  await page.locator('.bingo-cell[data-position="0"]').click();
  await page.getByRole('button', { name: 'Mark Complete' }).click();
  await page.locator('.bingo-cell[data-position="1"]').click();
  await page.getByRole('button', { name: 'Mark Complete' }).click();

  await page.locator('.bingo-cell[data-position="2"]').click();
  await page.getByRole('button', { name: 'Tomorrow morning' }).click();
  await expectToast(page, 'Goal reminder saved');
  await page.getByRole('button', { name: 'Cancel' }).click();

  await page.goto('/#profile');
  const reminderList = page.locator('#reminder-goal-list');
  await expect(reminderList).toContainText(xssGoal);
  await expect(reminderList.locator('img')).toHaveCount(0);

  await clearMailpit(request);
  await page.goto('/#profile');
  await expect(page.getByRole('button', { name: 'Send test email' })).toBeEnabled();
  await page.getByRole('button', { name: 'Send test email' }).click();

  const message = await waitForEmail(request, {
    to: user.email,
    subject: 'check-in',
  });
  const body = message.HTML || message.html || message.Body || message.body || '';
  expect(body).toContain('&lt;img src=x onerror=alert(1)&gt;');
  expect(body).not.toContain('<img src=x onerror=alert(1)>');
});

test('reminder emails include a PNG image URL', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'rempng');
  await clearMailpit(request);
  await register(page, user);

  await page.goto('/#dashboard');
  await createCardFromModal(page, { title: 'PNG Card' });
  await fillCardWithSuggestions(page);
  await finalizeCard(page);

  await verifyEmail(page, request, user);
  await enableReminders(page);
  await clearMailpit(request);

  await page.goto('/#profile');
  await expect(page.getByRole('button', { name: 'Send test email' })).toBeEnabled();
  await page.getByRole('button', { name: 'Send test email' }).click();

  const message = await waitForEmail(request, {
    to: user.email,
    subject: 'check-in',
  });
  const body = message.HTML || message.html || message.Body || message.body || '';

  const match = body.match(/https?:\/\/[^\s"']+\/r\/img\/[^\s"']+\.png/);
  expect(match).not.toBeNull();

  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://app:8080';
  const imageUrl = match[0].replace(/^https?:\/\/[^/]+/, baseURL);
  const response = await request.get(imageUrl);
  expect(response.ok()).toBeTruthy();
  expect(response.headers()['content-type']).toContain('image/png');
  const buffer = await response.body();
  expect(buffer.slice(0, 8)).toEqual(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]));
});
