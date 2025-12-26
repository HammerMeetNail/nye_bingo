const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
} = require('./helpers');

test('completion notes persist for finalized cards', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'notes');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page, { title: 'Notes Card' });
  await fillCardWithSuggestions(page);
  await finalizeCard(page);

  const targetCell = page.locator('.bingo-cell:not(.bingo-cell--free)').first();
  const position = await targetCell.getAttribute('data-position');
  const notes = 'Finished on a sunny afternoon.';

  await targetCell.click();
  await expect(page.locator('#modal-title')).toHaveText('Mark Complete');
  await page.fill('#complete-notes', notes);
  await page.getByRole('button', { name: 'Mark Complete' }).click();
  await expect(page.locator('.progress-text')).toContainText('1/');

  await page.locator(`.bingo-cell[data-position="${position}"]`).click();
  await expect(page.locator('#modal-title')).toHaveText('Goal Completed!');
  await expect(page.locator('.item-detail-notes')).toContainText(notes);
  const modal = page.locator('#modal-overlay');
  await modal.getByRole('button', { name: 'Close', exact: true }).click();

  await page.reload();
  await expect(page.locator('.finalized-card-view')).toBeVisible();
  await page.locator(`.bingo-cell[data-position="${position}"]`).click();
  await expect(page.locator('#modal-title')).toHaveText('Goal Completed!');
  await expect(page.locator('.item-detail-notes')).toContainText(notes);
});
