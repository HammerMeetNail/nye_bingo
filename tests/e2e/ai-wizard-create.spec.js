const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  expectToast,
} = require('./helpers');

test('AI wizard generates goals and creates a card', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'aiwiz');
  await register(page, user);

  await page.getByRole('button', { name: /Generate with AI Wizard/i }).click();
  await expect(page.locator('#modal-title')).toContainText('AI Goal Wizard');

  await page.selectOption('#ai-category', 'travel');
  await page.check('input[name="difficulty"][value="medium"]');
  await page.check('input[name="budget"][value="free"]');
  await page.fill('#ai-focus', 'Local adventures');
  await page.evaluate(() => {
    document.getElementById('ai-wizard-form')?.requestSubmit();
  });

  await expect(page.locator('#modal-title')).toContainText('Review Your Goals');
  await expect(page.locator('.ai-goal-input')).toHaveCount(24);

  await page.locator('#modal-overlay').getByRole('button', { name: 'Create Card →' }).click();
  await expectToast(page, 'AI Card Created!');
  await expect(page.locator('#item-input')).toBeVisible();
  await expect(page.locator('.bingo-cell:not(.bingo-cell--free)')).toHaveCount(24);
  await expect(page.locator('#bingo-grid')).toContainText('Sunrise Walk');
});

test('AI wizard create mode respects non-default grid size', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'aigrid');
  await register(page, user);

  await page.getByRole('button', { name: /Generate with AI Wizard/i }).click();
  await expect(page.locator('#modal-title')).toContainText('AI Goal Wizard');

  await page.selectOption('#ai-category', 'travel');
  await page.selectOption('#ai-grid-size', '3');
  await page.check('input[name="difficulty"][value="medium"]');
  await page.check('input[name="budget"][value="free"]');
  await page.fill('#ai-focus', 'Weekend trips');
  await page.evaluate(() => {
    document.getElementById('ai-wizard-form')?.requestSubmit();
  });

  await expect(page.locator('#modal-title')).toContainText('Review Your Goals');
  await expect(page.locator('.ai-goal-input')).toHaveCount(8);

  await page.locator('#modal-overlay').getByRole('button', { name: 'Create Card →' }).click();
  await expectToast(page, 'AI Card Created!');
  await expect(page.locator('#item-input')).toBeVisible();
  await expect(page.locator('.bingo-cell--free')).toHaveCount(1);
  await expect(page.locator('.bingo-cell:not(.bingo-cell--free)')).toHaveCount(8);
  await expect(page.locator('.bingo-cell[data-item-id]:not(.bingo-cell--free)')).toHaveCount(8);
});
