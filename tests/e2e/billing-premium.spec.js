const { test, expect } = require('@playwright/test');

const {
  buildUser,
  register,
  postStripeWebhook,
  getLastStripeCheckoutSession,
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

function dateRegexLine(prefix) {
  return new RegExp(`${prefix} [A-Za-z]{3} \\d{1,2}, \\d{4}`);
}

test.describe.serial('Billing: Premium (mocked Stripe, no listener)', () => {
  test('subscription + tip activates Premium via signed webhooks', async ({ page }, testInfo) => {
    const priceMonthly = process.env.STRIPE_PREMIUM_PRICE_MONTHLY || 'price_premium_monthly';
    const priceTip5 = process.env.STRIPE_TIP_PRICE_5 || 'price_tip_5';

    const user = buildUser(testInfo, 'billingsub');
    await register(page, user);

    // Start checkout from the Premium page.
    await page.goto('/premium', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('heading', { name: 'Premium', exact: true, level: 1 })).toBeVisible();

    // Click the upgrade button in the hero CTA slot.
    // Wait explicitly for the button since CTA content loads async after billing status check.
    const upgradeBtn = page.locator('#premium-cta-slot').getByRole('button', { name: 'Upgrade to Premium' });
    await expect(upgradeBtn).toBeVisible({ timeout: 30000 });
    await upgradeBtn.click();
    await expect(page.locator('#modal-title')).toHaveText('Upgrade to Premium');

    // Add a $5 tip; keep Monthly as default.
    await page.getByRole('button', { name: '$5' }).click();

    await Promise.all([
      page.waitForURL(/\/test\/checkout\//),
      page.getByRole('button', { name: 'Checkout' }).click(),
    ]);

    // Verify the app built a combined checkout session (subscription + tip).
    const lastSession = await getLastStripeCheckoutSession(page.request);
    expect(lastSession.mode).toBe('subscription');
    const prices = (lastSession.line_items || []).map((li) => li.price);
    expect(prices).toContain(priceMonthly);
    expect(prices).toContain(priceTip5);

    // Get user_id for webhook metadata.
    const meResp = await page.request.get('/api/auth/me');
    expect(meResp.ok()).toBeTruthy();
    const me = await meResp.json();
    const userID = me?.user?.id;
    expect(userID).toBeTruthy();

    const customerID = lastSession.customer;
    expect(customerID).toBeTruthy();

    // Stripe webhook sequence (minimal):
    // 1) checkout.session.completed stores stripe customer/subscription IDs
    // 2) customer.subscription.* sets billing state / premium
    const subscriptionID = `sub_test_${Math.random().toString(16).slice(2)}`;

    await postStripeWebhook(page.request, stripeEventBase('checkout.session.completed', {
      id: lastSession.id,
      customer: customerID,
      subscription: subscriptionID,
      metadata: {
        user_id: userID,
        purchase: 'subscription',
        interval: 'month',
        tip_amount: '5',
      },
    }));

    const nowSec = Math.floor(Date.now() / 1000);
    const periodEndSec = nowSec + 31 * 24 * 60 * 60;
    await postStripeWebhook(page.request, stripeEventBase('customer.subscription.created', {
      id: subscriptionID,
      customer: customerID,
      status: 'active',
      current_period_end: periodEndSec,
      cancel_at_period_end: false,
    }));

    // Simulate Stripe redirect back to the app and ensure the UI completes the webhook-driven upgrade.
    await page.goto('/?billing=success&session_id=cs_test_e2e#profile', { waitUntil: 'domcontentloaded' });
    await expect(page.locator('#premium-badge-slot')).toContainText('Premium', { timeout: 60000 });
    await expect(page.locator('#modal-overlay')).not.toHaveClass(/modal-overlay--visible/);

    await page.goto('/premium', { waitUntil: 'domcontentloaded' });
    await expect(page.locator('#premium-billing-status')).toContainText('Premium');
    await expect(page.locator('#premium-billing-status')).toContainText(dateRegexLine('Renews'));
    // There are two "Manage subscription" buttons (one in the hero card, one in "Your plan").
    await expect(page.getByRole('button', { name: 'Manage subscription', exact: true }).first()).toBeVisible();
  });

  test('lifetime activates Premium without subscription webhooks', async ({ page }, testInfo) => {
    const priceLifetime = process.env.STRIPE_PREMIUM_PRICE_LIFETIME || 'price_premium_lifetime';

    const user = buildUser(testInfo, 'billinglife');
    await register(page, user);

    await page.goto('/premium', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('heading', { name: 'Premium', exact: true, level: 1 })).toBeVisible();
    await page.locator('#premium-cta-slot').getByRole('button', { name: 'Upgrade to Premium' }).click();
    await expect(page.locator('#modal-title')).toHaveText('Upgrade to Premium');

    await page.getByRole('button', { name: 'Lifetime' }).click();

    await Promise.all([
      page.waitForURL(/\/test\/checkout\//),
      page.getByRole('button', { name: 'Checkout' }).click(),
    ]);

    const lastSession = await getLastStripeCheckoutSession(page.request);
    expect(lastSession.mode).toBe('payment');
    const prices = (lastSession.line_items || []).map((li) => li.price);
    expect(prices).toContain(priceLifetime);

    const meResp = await page.request.get('/api/auth/me');
    expect(meResp.ok()).toBeTruthy();
    const me = await meResp.json();
    const userID = me?.user?.id;
    expect(userID).toBeTruthy();

    const customerID = lastSession.customer;
    expect(customerID).toBeTruthy();

    await postStripeWebhook(page.request, stripeEventBase('checkout.session.completed', {
      id: lastSession.id,
      customer: customerID,
      subscription: '',
      metadata: {
        user_id: userID,
        purchase: 'lifetime',
      },
    }));

    await page.goto('/premium', { waitUntil: 'domcontentloaded' });
    await expect(page.locator('#premium-billing-status')).toContainText('Premium');
    await expect(page.locator('#premium-billing-status')).toContainText('No expiration');
    await expect(page.getByRole('button', { name: 'Manage Subscription' })).toHaveCount(0);
  });
});
