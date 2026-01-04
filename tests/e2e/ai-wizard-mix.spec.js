const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
} = require('./helpers');

test('AI wizard handles mix category in stub mode', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'aimix');
  await register(page, user);

  await page.getByRole('button', { name: /Generate with AI Wizard/i }).click();
  await expect(page.locator('#modal-title')).toContainText('AI Goal Wizard');

  await page.selectOption('#ai-category', 'mix');
  await page.check('input[name="difficulty"][value="medium"]');
  await page.check('input[name="budget"][value="free"]');
  await page.fill('#ai-focus', 'Surprise me');
  await page.evaluate(() => {
    document.getElementById('ai-wizard-form')?.requestSubmit();
  });

  await expect(page.locator('#modal-title')).toContainText('Review Your Goals');
  await expect(page.locator('.ai-goal-input')).toHaveCount(24);
  await expect(page.locator('.ai-goal-input').first()).toHaveValue(/.+/);

  await page.keyboard.press('Escape');
  await expect(page.locator('#modal-overlay')).not.toHaveClass(/modal-overlay--visible/);
});
