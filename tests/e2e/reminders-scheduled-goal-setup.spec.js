const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromModal,
  fillCardWithSuggestions,
  finalizeCard,
  clearMailpit,
} = require('./helpers');
const { verifyEmail, enableReminders } = require('./reminder-helpers');

test('scheduled goal reminders can be created', async ({ page, request }, testInfo) => {
  await clearMailpit(request);

  const user = buildUser(testInfo, 'remdue');
  await register(page, user);

  await page.goto('/#dashboard');
  await createCardFromModal(page, { title: 'Scheduled Reminder Card' });
  await fillCardWithSuggestions(page);
  await finalizeCard(page);
  const cardUrl = page.url();

  await verifyEmail(page, request, user);
  await enableReminders(page);

  await page.goto(cardUrl);
  await expect(page.locator('.finalized-card-view')).toBeVisible();
  const goalCell = page.locator('.bingo-cell[data-item-id]:not(.bingo-cell--free)').first();
  const itemId = await goalCell.getAttribute('data-item-id');
  expect(itemId).toBeTruthy();

  const csrfResponse = await page.request.get('/api/csrf');
  expect(csrfResponse.ok()).toBeTruthy();
  const csrf = await csrfResponse.json();
  const csrfToken = csrf && csrf.token ? csrf.token : '';
  expect(csrfToken).toBeTruthy();

  const sendAt = new Date(Date.now() + 15_000).toISOString();
  const response = await page.request.post('/api/reminders/goals', {
    headers: {
      'X-CSRF-Token': csrfToken,
    },
    data: {
      item_id: itemId,
      kind: 'one_time',
      schedule: { send_at: sendAt },
    },
  });
  expect(response.ok()).toBeTruthy();
});
