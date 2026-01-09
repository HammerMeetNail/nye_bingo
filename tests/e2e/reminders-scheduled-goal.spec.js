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

test('scheduled goal reminders are delivered by the background runner', async ({ page, request }, testInfo) => {
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
  const after = Date.now();
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

  const message = await waitForEmail(request, {
    to: user.email,
    subject: 'Reminder:',
    after,
    timeout: 60_000,
  });

  const body = message.Text || message.text || message.HTML || message.html || message.Body || message.body || '';
  expect(body).toContain('#card/');
  expect(body).toContain(itemId);
  expect(body).toContain('#profile');
});
