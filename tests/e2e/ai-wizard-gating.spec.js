const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  expectToast,
  waitForEmail,
  extractTokenFromEmail,
} = require('./helpers');

test('AI wizard requires verification after free generations are used', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'aigate');
  await register(page, user);

  await page.getByRole('button', { name: /Generate with AI Wizard/i }).click();
  await expect(page.locator('#modal-title')).toContainText('AI Goal Wizard');

  for (let i = 0; i < 5; i += 1) {
    await page.selectOption('#ai-category', 'travel');
    await page.check('input[name="difficulty"][value="medium"]');
    await page.check('input[name="budget"][value="free"]');
    await page.fill('#ai-focus', `Hiking ${i}`);
    await page.evaluate(() => {
      document.getElementById('ai-wizard-form')?.requestSubmit();
    });
    await expect(page.locator('#modal-title')).toContainText('Review Your Goals');

    const startOver = page.getByRole('button', { name: 'Start Over' });
    await startOver.scrollIntoViewIfNeeded();
    if (i < 4) {
      await startOver.click();
      await expect(page.locator('#modal-title')).toContainText('AI Goal Wizard');
    } else {
      await startOver.click();
    }
  }

  await expect(page.locator('#modal-title')).toContainText('Verify Email Required');
});

test('AI wizard continues after free limit for verified users', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'aiver');
  await register(page, user);

  await page.goto('/dashboard');
  const banner = page.locator('.verification-banner');
  await expect(banner).toBeVisible();
  const after = Date.now();
  await banner.getByRole('button', { name: 'Resend verification email' }).click();
  await expectToast(page, 'Verification email sent');

  const message = await waitForEmail(request, {
    to: user.email,
    subject: 'Verify your Year of Bingo account',
    after,
  });
  const token = extractTokenFromEmail(message, 'verify-email');

  await page.goto(`/verify-email?token=${token}`);
  await expect(page.getByRole('heading', { name: 'Email Verified!' })).toBeVisible();

  await page.goto('/create');
  await page.getByRole('button', { name: /Generate with AI Wizard/i }).click();
  await expect(page.locator('#modal-title')).toContainText('AI Goal Wizard');

  for (let i = 0; i < 6; i += 1) {
    await page.selectOption('#ai-category', 'travel');
    await page.check('input[name="difficulty"][value="medium"]');
    await page.check('input[name="budget"][value="free"]');
    await page.fill('#ai-focus', `Trail ${i}`);
    await page.evaluate(() => {
      document.getElementById('ai-wizard-form')?.requestSubmit();
    });
    await expect(page.locator('#modal-title')).toContainText('Review Your Goals');

    const startOver = page.getByRole('button', { name: 'Start Over' });
    await startOver.scrollIntoViewIfNeeded();
    await startOver.click();
    await expect(page.locator('#modal-title')).toContainText('AI Goal Wizard');
  }
});
