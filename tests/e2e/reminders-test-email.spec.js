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

test('reminder test email sends and arrives', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'remind');
  await clearMailpit(request);
  await register(page, user);

  await page.goto('/#dashboard');
  await createCardFromModal(page, { title: 'Reminder Card' });
  await fillCardWithSuggestions(page);
  await finalizeCard(page);

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
