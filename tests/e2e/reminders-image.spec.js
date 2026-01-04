const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromModal,
  fillCardWithSuggestions,
  finalizeCard,
  waitForEmail,
  clearMailpit,
} = require('./helpers');
const { verifyEmail, enableReminders } = require('./reminder-helpers');

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
