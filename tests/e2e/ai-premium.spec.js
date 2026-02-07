const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  expectToast,
  postStripeWebhook,
} = require('./helpers');

function stripeEventBase(type, object, { id, created, livemode } = {}) {
  return {
    id: id || `evt_test_${Math.random().toString(16).slice(2)}`,
    type,
    livemode: Boolean(livemode),
    created: Number.isFinite(created) ? created : Math.floor(Date.now() / 1000),
    data: {
      object,
    },
  };
}

async function activateLifetimePremium(page) {
  const meResp = await page.request.get('/api/auth/me');
  expect(meResp.ok(), 'User should be authenticated').toBeTruthy();
  const me = await meResp.json();
  const userID = me?.user?.id;
  expect(userID).toBeTruthy();

  await postStripeWebhook(page.request, stripeEventBase('checkout.session.completed', {
    id: `cs_test_${Math.random().toString(16).slice(2)}`,
    customer: `cus_test_${Math.random().toString(16).slice(2)}`,
    subscription: '',
    metadata: {
      user_id: userID,
      purchase: 'lifetime',
    },
  }));

  await expect.poll(async () => {
    const response = await page.request.get('/api/billing/status');
    if (!response.ok()) return false;
    const status = await response.json();
    return status?.is_premium === true && status?.features?.ai_enhancements === true;
  }, { timeout: 30000 }).toBe(true);
}

test('premium AI assist, regenerate, and fill-empty consume monthly enhancements', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'aipremium');
  await register(page, user);
  await activateLifetimePremium(page);

  const statusResp = await page.request.get('/api/ai/premium/status');
  expect(statusResp.ok()).toBeTruthy();
  const status = await statusResp.json();
  const limit = status.limit;
  let remaining = status.remaining;
  expect(limit).toBeGreaterThan(0);
  expect(remaining).toBeGreaterThan(3);

  await page.goto('/premium');
  await expect(page.locator('#premium-ai-status')).toContainText(`AI Enhancements remaining: ${remaining} / ${limit}`);

  await page.goto('/create');
  await createCardFromAuthenticatedCreate(page, { title: 'Premium AI Actions' });

  await page.fill('#item-input', 'Complete a local hiking trail');
  await page.click('#add-btn');
  await expect(page.locator('#ai-fill-empty-btn')).toBeVisible();

  await page.locator('.bingo-cell[data-item-id]').first().click();
  await expect(page.locator('#modal-title')).toContainText('Edit Goal');
  await expect(page.locator('#ai-premium-generate')).toBeVisible();

  await page.selectOption('#ai-premium-mode', 'next_step');
  await page.fill('#ai-premium-notes', 'I only have 20 minutes today.');
  const assistRequest = page.waitForResponse((response) => (
    response.url().includes('/api/ai/assist')
      && response.request().method() === 'POST'
  ));
  await page.click('#ai-premium-generate');
  const assistResponse = await assistRequest;
  expect(assistResponse.ok()).toBeTruthy();
  const assistData = await assistResponse.json();
  expect(typeof assistData.reply).toBe('string');
  expect(assistData.reply.length).toBeGreaterThan(0);
  expect(assistData.enhancements_remaining).toBe(remaining - 1);
  remaining = assistData.enhancements_remaining;
  await expect(page.locator('#ai-premium-results')).not.toBeEmpty();
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();

  await page.click('#ai-btn');
  await expect(page.locator('#modal-title')).toContainText('AI Goal Generator');
  await page.selectOption('#ai-category', 'travel');
  await page.check('input[name="difficulty"][value="medium"]');
  await page.check('input[name="budget"][value="free"]');
  await page.fill('#ai-focus', 'Short weekend adventures');
  await page.evaluate(() => {
    document.getElementById('ai-wizard-form')?.requestSubmit();
  });

  await expect(page.locator('#modal-title')).toContainText('Review Your Goals');
  const firstGoalInput = page.locator('.ai-goal-input[data-index="0"]');
  const firstGoalBefore = await firstGoalInput.inputValue();

  const regenerateRequest = page.waitForResponse((response) => (
    response.url().includes('/api/ai/regenerate')
      && response.request().method() === 'POST'
  ));
  await page.locator('[data-action="ai-regenerate-goal"][data-index="0"]').click();
  const regenerateResponse = await regenerateRequest;
  expect(regenerateResponse.ok()).toBeTruthy();
  const regenerateData = await regenerateResponse.json();
  expect(typeof regenerateData.goal).toBe('string');
  expect(regenerateData.goal.length).toBeGreaterThan(0);
  expect(regenerateData.enhancements_remaining).toBe(remaining - 1);
  remaining = regenerateData.enhancements_remaining;
  await expect(firstGoalInput).not.toHaveValue(firstGoalBefore);

  await page.getByRole('button', { name: 'Start Over' }).click();
  await expect(page.locator('#modal-title')).toContainText('AI Goal Generator');
  await page.getByRole('button', { name: 'Cancel' }).click();

  const fillRequest = page.waitForResponse((response) => (
    response.url().includes('/api/ai/fill-empty')
      && response.request().method() === 'POST'
  ));
  await page.click('#ai-fill-empty-btn');
  const fillResponse = await fillRequest;
  expect(fillResponse.ok()).toBeTruthy();
  const fillData = await fillResponse.json();
  expect(fillData.enhancements_remaining).toBe(remaining - 1);
  remaining = fillData.enhancements_remaining;

  await expectToast(page, 'Filled empty squares with AI');
  await expect(page.locator('.bingo-cell[data-item-id]:not(.bingo-cell--free)')).toHaveCount(24);

  await page.goto('/premium');
  await expect(page.locator('#premium-ai-status')).toContainText(`AI Enhancements remaining: ${remaining} / ${limit}`);
});
