const { test, expect } = require('@playwright/test');
const {
  buildUser,
  register,
  expectToast,
} = require('./helpers');

function stubClipboard(page) {
  return page.addInitScript(() => {
    try {
      Object.defineProperty(Navigator.prototype, 'clipboard', {
        get() {
          return {
            writeText: () => Promise.resolve(),
          };
        },
        configurable: true,
      });
    } catch (error) {
      // Ignore clipboard stubbing failures.
    }
  });
}

async function createToken(page, name, { closeModal = true } = {}) {
  await page.getByRole('button', { name: 'Create New Token' }).click();
  await expect(page.getByRole('heading', { name: 'Create API Token' })).toBeVisible();
  await page.fill('#token-name', name);
  await page.selectOption('#token-scope', 'read_write');
  await page.selectOption('#token-expiry', '0');
  await page.getByRole('button', { name: 'Generate Token' }).click();
  await expect(page.getByRole('heading', { name: 'Token Generated' })).toBeVisible();
  const token = await page.locator('#new-token').innerText();
  if (closeModal) {
    await page.getByRole('button', { name: 'Done' }).click();
  }
  return token;
}

test('API tokens can be created, copied, and revoked', async ({ page, request }, testInfo) => {
  await stubClipboard(page);
  const user = buildUser(testInfo, 'token');
  await register(page, user);

  await page.goto('/profile');
  await expect(page.locator('#api-tokens-list')).toContainText('No active tokens');

  const token = await createToken(page, 'CLI Token', { closeModal: false });
  await page.getByRole('button', { name: 'Copy' }).click();
  await expectToast(page, 'Copied to clipboard');
  await page.getByRole('button', { name: 'Done' }).click();

  const list = page.locator('#api-tokens-list');
  await expect(list).toContainText('CLI Token');

  const apiResponse = await request.get('/api/cards', {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(apiResponse.ok()).toBeTruthy();

  page.once('dialog', (dialog) => dialog.accept());
  await list.locator('button[title="Revoke Token"]').first().click();
  await expectToast(page, 'Token revoked');
  await expect(list).toContainText('No active tokens');

  const revokedResponse = await request.get('/api/cards', {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(revokedResponse.status()).toBe(401);
});

test('API tokens can be revoked all at once', async ({ page }, testInfo) => {
  const user = buildUser(testInfo, 'tokens');
  await register(page, user);

  await page.goto('/profile');
  await expect(page.locator('#api-tokens-list')).toContainText('No active tokens');

  await createToken(page, 'Token One');
  await createToken(page, 'Token Two');

  const list = page.locator('#api-tokens-list');
  await expect(list).toContainText('Token One');
  await expect(list).toContainText('Token Two');

  page.once('dialog', (dialog) => dialog.accept());
  await page.getByRole('button', { name: 'Revoke All Tokens' }).click();
  await expectToast(page, 'All tokens revoked');
  await expect(list).toContainText('No active tokens');
});
