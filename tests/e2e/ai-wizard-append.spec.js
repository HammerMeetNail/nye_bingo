const { test, expect } = require('@playwright/test');
const { buildUser, register, createCardFromAuthenticatedCreate, expectToast } = require('./helpers');

test('AI wizard append mode fills only open cells and preserves existing goals', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'aiapp');
  await register(page, user);

  await createCardFromAuthenticatedCreate(page, { title: 'Append Card' });

  const manualGoals = ['Manual Goal 1', 'Manual Goal 2', 'Manual Goal 3'];
  for (const goal of manualGoals) {
    await page.fill('#item-input', goal);
    await page.click('#add-btn');
  }

  await expect(page.locator('.bingo-cell[data-item-id]:not(.bingo-cell--free)')).toHaveCount(3);
  const freePosBefore = await page.locator('.bingo-cell--free').getAttribute('data-position');

  await page.click('#ai-btn');
  await expect(page.locator('#modal-title')).toContainText('AI Goal Generator');

  await page.selectOption('#ai-category', 'travel');
  await page.check('input[name="difficulty"][value="medium"]');
  await page.check('input[name="budget"][value="free"]');
  await page.fill('#ai-focus', 'Local adventures');
  await page.evaluate(() => {
    document.getElementById('ai-wizard-form')?.requestSubmit();
  });

  await expect(page.locator('#modal-title')).toContainText('Review Your Goals');
  await expect(page.locator('.ai-goal-input')).toHaveCount(21);

  await page.locator('#modal-overlay').getByRole('button', { name: 'Add to Card →' }).click();
  await expectToast(page, 'Goals added!');

  for (const goal of manualGoals) {
    await expect(page.locator('#bingo-grid')).toContainText(goal);
  }

  await expect(page.locator('.bingo-cell[data-item-id]:not(.bingo-cell--free)')).toHaveCount(24);
  const freePosAfter = await page.locator('.bingo-cell--free').getAttribute('data-position');
  expect(freePosAfter).toBe(freePosBefore);
});

