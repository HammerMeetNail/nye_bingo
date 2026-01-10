const { test, expect } = require('@playwright/test');

test('document includes OpenGraph/Twitter preview metadata and a public OG image', async ({ page, request }) => {
  const response = await page.goto('/');
  expect(response).not.toBeNull();

  const ogType = page.locator('meta[property="og:type"]');
  await expect(ogType).toHaveAttribute('content', 'website');
  await expect(page.locator('meta[property="og:site_name"]')).toHaveAttribute('content', 'Year of Bingo');
  await expect(page.locator('meta[property="og:locale"]')).toHaveAttribute('content', 'en_US');
  await expect(page.locator('meta[property="og:title"]')).toHaveAttribute('content', 'Year of Bingo - Goal Tracker');
  await expect(page.locator('meta[property="og:description"]')).toHaveAttribute(
    'content',
    'Create your annual bingo card and track your goals throughout the year',
  );

  const ogURL = page.locator('meta[property="og:url"]');
  await expect(ogURL).toHaveCount(1);
  const ogURLValue = await ogURL.getAttribute('content');
  expect(ogURLValue).toMatch(/^https?:\/\//);
  expect(ogURLValue).toBe(page.url());

  const ogImage = page.locator('meta[property="og:image"]');
  await expect(ogImage).toHaveCount(1);

  const ogImageURL = await ogImage.getAttribute('content');
  expect(ogImageURL).toMatch(/^https?:\/\//);
  expect(ogImageURL).toMatch(/\/og\/default\.png$/);

  await expect(page.locator('meta[property="og:image:width"]')).toHaveAttribute('content', '1200');
  await expect(page.locator('meta[property="og:image:height"]')).toHaveAttribute('content', '630');
  await expect(page.locator('meta[property="og:image:alt"]')).toHaveAttribute(
    'content',
    'Year of Bingo card preview with sample goals',
  );
  await expect(page.locator('meta[name="twitter:card"]')).toHaveAttribute('content', 'summary_large_image');
  await expect(page.locator('meta[name="twitter:title"]')).toHaveAttribute('content', 'Year of Bingo - Goal Tracker');
  await expect(page.locator('meta[name="twitter:description"]')).toHaveAttribute(
    'content',
    'Create your annual bingo card and track your goals throughout the year',
  );
  await expect(page.locator('meta[name="twitter:image"]')).toHaveAttribute('content', ogImageURL);

  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://app:8080';
  const imageURL = ogImageURL.replace(/^https?:\/\/[^/]+/, baseURL);
  const imageResponse = await request.get(imageURL);
  expect(imageResponse.ok()).toBeTruthy();
  expect(imageResponse.headers()['content-type']).toContain('image/png');

  const buffer = await imageResponse.body();
  expect(buffer.slice(0, 8)).toEqual(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]));
});
