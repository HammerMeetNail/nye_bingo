const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  loginWithCredentials,
  logout,
  waitForEmail,
  extractTokenFromEmail,
  setOIDCNextUser,
} = require('./helpers');

test.describe.serial('google auth', () => {
  test('google login prompts for username on first login', async ({ page, request }, testInfo) => {
    const user = buildUser(testInfo, 'google');
    await setOIDCNextUser(request, {
      email: user.email,
      emailVerified: true,
      sub: `sub-${user.email}`,
    });

    await page.goto('/#login');
    await page.getByRole('link', { name: 'Continue with Google' }).click();

    await expect(page.getByRole('heading', { name: 'Complete Your Signup' })).toBeVisible();
    await page.fill('#google-username', user.username);
    await page.check('#google-searchable');
    const completeResponse = page.waitForResponse((response) => (
      response.url().includes('/api/auth/google/complete')
        && response.request().method() === 'POST'
        && response.ok()
    ));
    await page.getByRole('button', { name: 'Finish Signup' }).click();
    await completeResponse;

    await expect(page.getByRole('heading', { name: 'My Bingo Cards' })).toBeVisible();
    await page.goto('/#profile');
    await expect(page.locator('.badge').filter({ hasText: 'Verified' })).toBeVisible();
  });

  test('google login links to existing password account by email', async ({ page, request }, testInfo) => {
    const user = buildUser(testInfo, 'googlelink');
    await register(page, user);
    await logout(page);

    await setOIDCNextUser(request, {
      email: user.email,
      emailVerified: true,
      sub: `sub-${user.email}`,
    });

    await page.goto('/#login');
    await page.getByRole('link', { name: 'Continue with Google' }).click();

    await expect(page.getByRole('heading', { name: 'My Bingo Cards' })).toBeVisible();
    await expect(page.getByRole('link', { name: `Hi, ${user.username}` })).toBeVisible();
  });

  test('google-first user can set password via reset flow and login with password', async ({ page, request }, testInfo) => {
    const user = buildUser(testInfo, 'googlereset');
    await setOIDCNextUser(request, {
      email: user.email,
      emailVerified: true,
      sub: `sub-${user.email}`,
    });

    await page.goto('/#login');
    await page.getByRole('link', { name: 'Continue with Google' }).click();
    await expect(page.getByRole('heading', { name: 'Complete Your Signup' })).toBeVisible();
    await page.fill('#google-username', user.username);
    await page.getByRole('button', { name: 'Finish Signup' }).click();
    await expect(page.getByRole('heading', { name: 'My Bingo Cards' })).toBeVisible();
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
    await page.getByRole('button', { name: 'Reset Password' }).click();
    await expect(page.getByRole('heading', { name: 'My Bingo Cards' })).toBeVisible();
    await logout(page);

    await loginWithCredentials(page, user.email, 'NewPass1');
  });
});
