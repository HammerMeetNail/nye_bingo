const { test, expect } = require('@playwright/test');
const { loginWithCredentials } = require('./helpers');

test('seeded cards render with progress and FREE space', async ({ page }) => {
  await loginWithCredentials(page, 'alice@test.com', 'Password1');

  const cardPreview = page.locator('.dashboard-card-preview').first();
  await expect(cardPreview).toBeVisible();

  await cardPreview.locator('a').first().click();

  await expect(page.locator('.finalized-card-view')).toBeVisible();
  const progressText = await page.locator('.progress-text').innerText();
  const progressMatch = progressText.match(/(\d+)\/(\d+) completed/);
  expect(progressMatch).not.toBeNull();
  if (progressMatch) {
    expect(Number.parseInt(progressMatch[1], 10)).toBeGreaterThan(0);
    expect(Number.parseInt(progressMatch[2], 10)).toBeGreaterThan(0);
  }
  await expect(page.locator('.bingo-cell--free')).toHaveCount(1);
});

test('friend card reactions can be added', async ({ page }) => {
  await loginWithCredentials(page, 'bob@test.com', 'Password1');

  await page.goto('/#friends');
  await expect(page.getByRole('heading', { name: 'Friends', level: 2 })).toBeVisible();

  const friendRow = page.locator('#friends-list .friend-item').first();
  await expect(friendRow).toBeVisible();
  await friendRow.getByRole('link', { name: 'View Card' }).click();

  await expect(page.locator('.finalized-card-view')).toBeVisible();

  await page.locator('.bingo-cell--completed').first().click();
  await expect(page.getByRole('heading', { name: 'Completed Goal' })).toBeVisible();
  if (await page.locator('.reaction-badge').count() === 0) {
    await page.locator('.emoji-btn').first().click();
  }
  await expect(page.locator('.reaction-badge').first()).toBeVisible();
});

test('archived cards show archived view', async ({ page }) => {
  await loginWithCredentials(page, 'alice@test.com', 'Password1');

  const firstCard = page.locator('.dashboard-card-preview').first();
  await expect(firstCard).toBeVisible();
  const yearText = await firstCard.locator('.year-badge').innerText();
  const cardByYear = () => page.locator('.dashboard-card-preview').filter({
    has: page.locator('.year-badge', { hasText: yearText }),
  });

  await cardByYear().locator('.dashboard-card-checkbox').check();
  const selectedText = await page.locator('#selected-count').innerText();
  expect(selectedText).toMatch(/\d+ selected/);
  await page.getByRole('button', { name: 'Actions' }).click();
  await page.locator('.dropdown-menu .dropdown-item').filter({ hasText: /^\s*Archive\s*$/ }).click();
  await expect(cardByYear().locator('.archive-badge')).toBeVisible();

  await cardByYear().locator('a').first().click();
  await expect(page.locator('.bingo-grid--archive')).toBeVisible();
  await expect(page.locator('.archive-badge')).toBeVisible();
});
