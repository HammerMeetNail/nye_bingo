const { expect } = require('@playwright/test');
const {
  waitForEmail,
  extractTokenFromEmail,
} = require('./helpers');

async function verifyEmail(page, request, user) {
  await page.goto('/profile');
  const afterVerify = Date.now();
  await page.getByRole('button', { name: 'Resend verification email' }).click();

  const verifyMessage = await waitForEmail(request, {
    to: user.email,
    subject: 'Verify your Year of Bingo account',
    after: afterVerify,
  });
  const token = extractTokenFromEmail(verifyMessage, 'verify-email');
  await page.goto(`/verify-email?token=${token}`);
  await expect(page.getByRole('heading', { name: 'Email Verified!' })).toBeVisible();
}

async function enableReminders(page) {
  await page.goto('/profile');
  const toggle = page.locator('#reminder-email-enabled');
  await expect(toggle).toBeEnabled();
  if (!(await toggle.isChecked())) {
    const waitResponse = page.waitForResponse((response) => (
      response.url().includes('/api/reminders/settings')
        && response.request().method() === 'PUT'
        && response.ok()
    ));
    await toggle.check();
    await waitResponse;
    await expect(page.locator('#toast-container')).toContainText('Reminder settings updated');
  }
}

module.exports = {
  verifyEmail,
  enableReminders,
};
