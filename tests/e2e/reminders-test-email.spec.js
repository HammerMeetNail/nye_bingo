const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromModal,
  fillCardWithSuggestions,
  finalizeCard,
  waitForEmail,
} = require('./helpers');
const { verifyEmail, enableReminders } = require('./reminder-helpers');

test('reminder test email sends and arrives', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'remind');
  await register(page, user);

  await page.goto('/#dashboard');
  await createCardFromModal(page, { title: 'Reminder Card' });
  await fillCardWithSuggestions(page);
  await finalizeCard(page);

  await verifyEmail(page, request, user);

  await enableReminders(page);
  await expect(page.getByRole('button', { name: 'Send test email' })).toBeEnabled();
  const sendResponse = page.waitForResponse((response) => (
    response.url().includes('/api/reminders/test')
      && response.request().method() === 'POST'
  ));
  const after = Date.now();
  await page.getByRole('button', { name: 'Send test email' }).click();
  await sendResponse;

  const message = await waitForEmail(request, {
    to: user.email,
    subject: 'check-in',
    after,
  });

  const body = message.Text || message.text || message.HTML || message.html || message.Body || message.body || '';
  expect(body).toContain('#profile');
});
