const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
  expectToast,
} = require('./helpers');

test('completing a row triggers a bingo celebration', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'bingo');
  await register(page, user);

  await createCardFromAuthenticatedCreate(page, { title: 'Bingo Card' });

  await fillCardWithSuggestions(page);
  await finalizeCard(page);

  const positions = ['0', '1', '2', '3', '4'];
  for (let i = 0; i < positions.length; i += 1) {
    await page.locator(`.bingo-cell[data-position="${positions[i]}"]`).click();
    await page.getByRole('button', { name: 'Mark Complete' }).click();
    if (i === positions.length - 1) {
      await expectToast(page, 'BINGO! Row complete');
    }
  }
});
