const { test, expect } = require('@playwright/test');

test('anonymous users see AI wizard auth gate modal', async ({ page }) => {
  await page.goto('/#create');
  await page.getByRole('button', { name: /Generate with AI Wizard/i }).click();

  await expect(page.locator('#modal-title')).toContainText('Use the AI Goal Wizard');
  await expect(page.getByRole('link', { name: 'Create Account' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'I Already Have an Account' })).toBeVisible();
});

