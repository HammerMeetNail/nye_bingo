const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  expectToast,
  waitForEmail,
  expectNoEmail,
} = require('./helpers');

test('support form sends an email to support', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'support');

  await page.goto('/support');
  await page.fill('#support-email', user.email);
  await page.selectOption('#support-category', 'Bug Report');
  const message = 'Support request message for E2E coverage.';
  await page.fill('#support-message', message);
  const after = Date.now();
  await page.getByRole('button', { name: 'Send Message' }).click();

  await expectToast(page, 'message has been sent');

  const email = await waitForEmail(request, {
    to: 'support@yearofbingo.com',
    subject: '[Support] Bug Report',
    after,
  });
  const body = email.Text || email.text || email.HTML || email.html || '';
  expect(body).toContain(user.email);
  expect(body).toContain(message);
});

test('support form validates required fields and message length', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'support');

  await page.goto('/support');
  await page.fill('#support-email', user.email);
  await page.selectOption('#support-category', 'General Question');
  await page.fill('#support-message', 'Too short');
  await page.evaluate(() => {
    const form = document.getElementById('support-form');
    if (form) form.noValidate = true;
  });
  const after = Date.now();
  await page.getByRole('button', { name: 'Send Message' }).click();

  await expectToast(page, 'Message must be at least 10 characters');
  await expectNoEmail(request, { to: 'support@yearofbingo.com', timeout: 2000, after });
});

test('support form pre-fills email when logged in', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'support');
  await register(page, user);

  await page.goto('/support');
  await expect(page.locator('#support-email')).toHaveValue(user.email);
});
