const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  loginWithCredentials,
  logout,
  expectToast,
  waitForEmail,
  extractTokenFromEmail,
} = require('./helpers');

test('magic link login signs in with emailed token', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'magic');
  await register(page, user);
  await logout(page);

  await page.goto('/#magic-link');
  await page.fill('#magic-link-form #email', user.email);
  const after = Date.now();
  await page.getByRole('button', { name: 'Send login link' }).click();
  await expect(page.getByRole('heading', { name: 'Check your email' })).toBeVisible();

  const message = await waitForEmail(request, {
    to: user.email,
    subject: 'Your Year of Bingo login link',
    after,
  });
  const token = extractTokenFromEmail(message, 'magic-link');

  await page.goto(`/#magic-link?token=${token}`);
  await expect(page.getByRole('heading', { name: 'My Bingo Cards' })).toBeVisible();
});

test('password reset flow updates credentials', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'reset');
  await register(page, user);
  await logout(page);

  await page.goto('/#forgot-password');
  await page.fill('#forgot-password-form #email', user.email);
  const after = Date.now();
  await page.getByRole('button', { name: 'Send reset link' }).click();
  await expect(page.getByRole('heading', { name: 'Check your email' })).toBeVisible();

  const message = await waitForEmail(request, {
    to: user.email,
    subject: 'Reset your Year of Bingo password',
    after,
  });
  const token = extractTokenFromEmail(message, 'reset-password');

  await page.goto(`/#reset-password?token=${token}`);
  await page.fill('#reset-password-form #password', 'NewPass1');
  await page.fill('#reset-password-form #confirm-password', 'NewPass1');
  const resetResponse = page.waitForResponse((response) => (
    response.url().includes('/api/auth/reset-password')
      && response.request().method() === 'POST'
  ));
  await page.getByRole('button', { name: 'Reset Password' }).click({ noWaitAfter: true });
  await resetResponse;
  await expectToast(page, 'Password reset successfully');
  await expect(page.getByRole('heading', { name: 'My Bingo Cards' })).toBeVisible();

  await logout(page);
  await loginWithCredentials(page, user.email, 'NewPass1');
});

test('email verification banner clears after verifying', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'verify');
  await register(page, user);

  await page.goto('/#dashboard');
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

  await page.goto(`/#verify-email?token=${token}`);
  await expect(page.getByRole('heading', { name: 'Email Verified!' })).toBeVisible();

  await page.goto('/#dashboard');
  await expect(page.locator('.verification-banner')).toHaveCount(0);

  await page.goto('/#profile');
  await expect(page.locator('.badge').filter({ hasText: 'Verified' })).toBeVisible();
});

test('verification link does not break authenticated actions in an already-open tab', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'csrfverify');
  await register(page, user);

  await page.getByRole('link', { name: `Hi, ${user.username}` }).click();
  await expect(page.getByRole('heading', { name: 'Account Settings' })).toBeVisible();

  const after = Date.now();
  await page.getByRole('button', { name: 'Resend verification email' }).click();
  await expectToast(page, 'Verification email sent');

  const message = await waitForEmail(request, {
    to: user.email,
    subject: 'Verify your Year of Bingo account',
    after,
  });
  const token = extractTokenFromEmail(message, 'verify-email');

  const mailPage = await page.context().newPage();
  await mailPage.goto('http://mailpit:8025');
  await mailPage.goto(`/#verify-email?token=${token}`);
  await expect(mailPage.getByRole('heading', { name: 'Email Verified!' })).toBeVisible();
  await mailPage.close();

  await page.getByRole('link', { name: '← Back' }).click();
  await expect(page.getByRole('heading', { name: 'My Bingo Cards' })).toBeVisible();
  await page.getByRole('link', { name: 'Create Your First Card' }).click();

  await createCardFromAuthenticatedCreate(page);
  expect(page.url()).toContain('#card/');
});
