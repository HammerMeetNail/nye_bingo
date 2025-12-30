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

function buildXssUsername(testInfo) {
  const id = testInfo?.testId ? testInfo.testId.slice(-6) : Date.now().toString().slice(-6);
  return `xss'"<b>test</b>-${id}`;
}

test('xss usernames render safely and friend actions still work', async ({ browser }, testInfo) => {
  const xssUsername = buildXssUsername(testInfo);
  const userA = buildUser(testInfo, 'xssa', { username: xssUsername });
  const userB = buildUser(testInfo, 'xssb');

  const contextA = await browser.newContext();
  const pageA = await contextA.newPage();
  await register(pageA, userA, { searchable: true });

  const contextB = await browser.newContext();
  const pageB = await contextB.newPage();
  await register(pageB, userB, { searchable: true });

  await pageB.goto('/#friends');
  await pageB.fill('#friend-search', userA.username);
  await pageB.click('#search-btn');
  const results = pageB.locator('#search-results');
  await expect(results).toContainText(userA.username);
  await results.getByRole('button', { name: 'Add Friend' }).click();

  await pageA.goto('/#friends');
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();
  await expectToast(pageA, 'Friend request accepted');

  await pageB.reload();
  const friendRow = pageB.locator('#friends-list .friend-item').filter({ hasText: userA.username });
  await expect(friendRow).toContainText(userA.username);
  pageB.once('dialog', (dialog) => dialog.accept());
  await friendRow.getByRole('button', { name: 'Remove' }).click();
  await expectToast(pageB, 'Friend removed');
  await expect(pageB.locator('#friends-list')).toContainText('No friends yet');

  await pageB.fill('#friend-search', userA.username);
  await pageB.click('#search-btn');
  await expect(results).toContainText(userA.username);
  await results.getByRole('button', { name: 'Add Friend' }).click();

  await pageA.reload();
  await pageA.locator('#requests-list .friend-item').getByRole('button', { name: 'Accept' }).click();
  await expectToast(pageA, 'Friend request accepted');

  await pageB.reload();
  const friendRowAgain = pageB.locator('#friends-list .friend-item').filter({ hasText: userA.username });
  await expect(friendRowAgain).toContainText(userA.username);
  pageB.once('dialog', (dialog) => dialog.accept());
  await friendRowAgain.getByRole('button', { name: 'Block' }).click();
  await expectToast(pageB, 'User blocked');

  const blockedSection = pageB.locator('#blocked-users');
  await expect(blockedSection).toBeVisible();
  await expect(blockedSection).toContainText(userA.username);
  const blockedRow = pageB.locator('#blocked-list .friend-item').filter({ hasText: userA.username });
  pageB.once('dialog', (dialog) => dialog.accept());
  await blockedRow.getByRole('button', { name: 'Unblock' }).click();
  await expectToast(pageB, 'User unblocked');

  await contextA.close();
  await contextB.close();
});

test('invite copy and revoke actions work with xss usernames', async ({ page }, testInfo) => {
  const xssUsername = buildXssUsername(testInfo);
  const user = buildUser(testInfo, 'xssi', { username: xssUsername });

  await stubClipboard(page);
  await register(page, user, { searchable: true });
  await page.goto('/#friends');

  await page.getByRole('button', { name: 'Create Invite Link' }).click();
  const inviteInput = page.locator('#invite-link-input');
  await expect(inviteInput).toBeVisible();
  const inviteLink = await inviteInput.inputValue();
  expect(inviteLink).toContain('http');

  await page.getByRole('button', { name: 'Copy' }).click();
  await expectToast(page, 'Invite link copied!');

  const inviteList = page.locator('#invite-list');
  await expect(inviteList).toContainText('Invite created');
  await inviteList.getByRole('button', { name: 'Revoke' }).click();
  await expectToast(page, 'Invite revoked');
  await expect(inviteList).toContainText('No active invites.');
});
