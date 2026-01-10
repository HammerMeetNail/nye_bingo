const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  createCardFromAuthenticatedCreate,
  fillCardWithSuggestions,
  finalizeCard,
} = require('./helpers');

function parseMeta(html, propertyOrName) {
  const escaped = propertyOrName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const re = new RegExp(`<meta\\s+(?:property|name)="${escaped}"\\s+content="([^"]+)"`, 'i');
  const match = html.match(re);
  return match ? match[1] : null;
}

test('share landing page includes card-specific OG tags and image changes with progress', async ({ page, request }, testInfo) => {
  const user = buildUser(testInfo, 'shareog');
  await register(page, user);
  await createCardFromAuthenticatedCreate(page);

  await fillCardWithSuggestions(page);
  await finalizeCard(page);

  await page.locator('[data-action="open-share-modal"]').click();
  await page.getByRole('button', { name: 'Enable Sharing' }).click();
  const shareLink = await page.locator('#share-link-input').inputValue();
  expect(shareLink).toContain('/s/');
  await page.keyboard.press('Escape');
  await expect(page.locator('#modal-overlay')).not.toHaveClass(/modal-overlay--visible/);

  const shareResponse1 = await request.get(shareLink);
  expect(shareResponse1.ok()).toBeTruthy();
  const html1 = await shareResponse1.text();

  const ogURL = parseMeta(html1, 'og:url');
  const ogImage1 = parseMeta(html1, 'og:image');
  expect(ogURL).toContain('/s/');
  expect(ogImage1).toContain('/og/share/');

  const imageResponse1 = await request.get(ogImage1);
  expect(imageResponse1.ok()).toBeTruthy();
  const buf1 = await imageResponse1.body();
  expect(buf1.slice(0, 8)).toEqual(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]));

  // Mark a square complete to change progress, then ensure the og:image version changes.
  const completeResponse = page.waitForResponse((response) => (
    response.url().includes('/complete')
      && response.request().method() === 'PUT'
  ));
  await page.locator('.bingo-cell[data-position="0"]').click();
  await page.getByRole('button', { name: 'Mark Complete' }).click();
  await completeResponse;

  const shareResponse2 = await request.get(shareLink);
  expect(shareResponse2.ok()).toBeTruthy();
  const html2 = await shareResponse2.text();
  const ogImage2 = parseMeta(html2, 'og:image');
  expect(ogImage2).not.toBeNull();
  expect(ogImage2).not.toEqual(ogImage1);
});
