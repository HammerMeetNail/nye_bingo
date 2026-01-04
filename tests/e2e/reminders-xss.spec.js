const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromModal,
  fillCardWithSuggestions,
  finalizeCard,
  waitForEmail,
  expectToast,
} = require('./helpers');
const { verifyEmail, enableReminders } = require('./reminder-helpers');

test('reminder emails escape goal content', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'remxss');
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

  await page.goto('/#profile');
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
  const body = message.HTML || message.html || message.Body || message.body || '';
  expect(body).toContain('&lt;img src=x onerror=alert(1)&gt;');
  expect(body).not.toContain('<img src=x onerror=alert(1)>');
});
