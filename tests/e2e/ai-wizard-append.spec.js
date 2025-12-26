const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  createCardFromModal,
  expectToast,
} = require('./helpers');

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

test('AI wizard append mode respects no-FREE card capacity', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'ainofree');
  await register(page, user);

  await createCardFromAuthenticatedCreate(page, { title: 'Initial Card' });
  await createCardFromModal(page, { title: 'No Free', gridSize: 4, hasFree: false });
  await expect(page.locator('.bingo-header')).toHaveCount(4);
  await expect(page.locator('.bingo-cell[data-position]')).toHaveCount(16);

  const manualGoals = ['Manual Goal A', 'Manual Goal B'];
  for (const goal of manualGoals) {
    await page.fill('#item-input', goal);
    await page.click('#add-btn');
  }

  await expect(page.locator('.bingo-cell[data-item-id]')).toHaveCount(2);
  await expect(page.locator('.bingo-cell--free')).toHaveCount(0);

  const expectedOpen = await page.evaluate(() => {
    const grid = document.getElementById('bingo-grid');
    if (!grid) return null;
    const allCells = grid.querySelectorAll('.bingo-cell[data-position]:not(.bingo-cell--free)');
    const filledCells = grid.querySelectorAll('.bingo-cell[data-position][data-item-id]:not(.bingo-cell--free)');
    return Math.max(0, allCells.length - filledCells.length);
  });

  expect(typeof expectedOpen).toBe('number');
  expect(expectedOpen).toBeGreaterThan(0);

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
  await expect(page.locator('.ai-goal-input')).toHaveCount(expectedOpen);

  await page.locator('#modal-overlay').getByRole('button', { name: 'Add to Card →' }).click();
  await expectToast(page, 'Goals added!');

  for (const goal of manualGoals) {
    await expect(page.locator('#bingo-grid')).toContainText(goal);
  }

  const expectedCapacity = manualGoals.length + expectedOpen;
  await expect(page.locator('.bingo-cell[data-item-id]')).toHaveCount(expectedCapacity);
  await expect(page.locator('.bingo-cell--free')).toHaveCount(0);
});
