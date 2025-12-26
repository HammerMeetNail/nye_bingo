const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
} = require('./helpers');

test('dragging items reorders without moving FREE cell', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'drag');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page, { title: 'Drag Card' });

  await page.fill('#item-input', 'Move Me');
  await page.click('#add-btn');
  await page.fill('#item-input', 'Target Spot');
  await page.click('#add-btn');
  await page.fill('#item-input', 'Extra Goal');
  await page.click('#add-btn');

  const freeBefore = await page.locator('.bingo-cell--free').getAttribute('data-position');
  const filledCells = page.locator('.bingo-cell:not(.bingo-cell--free):not(.bingo-cell--empty)');
  await expect(filledCells).toHaveCount(3);

  const sourceCell = filledCells.nth(0);
  const sourceText = await sourceCell.locator('.bingo-cell-content').innerText();
  const sourcePosition = await sourceCell.getAttribute('data-position');
  const targetCell = page.locator('.bingo-cell--empty').first();
  const targetPosition = await targetCell.getAttribute('data-position');

  const dataTransfer = await page.evaluateHandle(() => new DataTransfer());
  await sourceCell.dispatchEvent('dragstart', { dataTransfer });
  await targetCell.dispatchEvent('drop', { dataTransfer });
  await sourceCell.dispatchEvent('dragend', { dataTransfer });

  await expect(page.locator(`.bingo-cell[data-position="${targetPosition}"] .bingo-cell-content`)).toHaveText(sourceText);
  await expect(page.locator(`.bingo-cell[data-position="${sourcePosition}"]`)).toHaveClass(/bingo-cell--empty/);
  const freeAfter = await page.locator('.bingo-cell--free').getAttribute('data-position');
  expect(freeAfter).toBe(freeBefore);
});
