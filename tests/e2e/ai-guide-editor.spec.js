const { test, expect } = require('@playwright/test');
const { buildUser, register, createCardFromAuthenticatedCreate, expectToast } = require('./helpers');

test('AI guide refine updates a goal in the editor', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'aiguide');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page, { title: 'AI Guide Refine' });

  await page.fill('#item-input', "Visit a local farmer's market");
  await page.click('#add-btn');

  const filledCell = page.locator('.bingo-cell[data-item-id]').first();
  await filledCell.click();
  await expect(page.locator('#modal-title')).toContainText('Edit Goal');

  await page.fill('#ai-refine-hint', 'shorter and cheaper');
  await page.click('#ai-refine-generate');

  const suggestion = page.locator('#ai-refine-results [data-ai-suggestion="0"]');
  await expect(suggestion).toBeVisible();
  const suggestionText = (await suggestion.innerText()).trim();
  await suggestion.click();

  const editTextarea = page.locator('textarea[id^="edit-item-content-"]');
  await expect(editTextarea).toHaveValue(suggestionText);

  await page.getByRole('button', { name: 'Save' }).click();
  await expectToast(page, 'Goal updated');
  await expect(filledCell).toContainText(suggestionText.slice(0, 12));
});

test('empty cell opens add goal modal', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'aiguide2');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page, { title: 'AI Guide Empty' });

  const emptyCell = page.locator('.bingo-cell--empty').first();
  await emptyCell.click();
  await expect(page.locator('#modal-title')).toContainText('Add Goal');

  await page.fill('textarea[id^="edit-item-content-"]', 'Walk the neighborhood loop');
  await page.getByRole('button', { name: 'Save' }).click();

  await expectToast(page, 'Goal added');
  await expect(page.locator('.bingo-cell[data-item-id]').first()).toContainText('Walk');
});
