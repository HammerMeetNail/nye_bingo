const { expect } = require('@playwright/test');
const crypto = require('crypto');

const MAILPIT_BASE_URL = process.env.MAILPIT_BASE_URL || 'http://mailpit:8025';
const MAILPIT_WAIT_TIMEOUT_MS = Number.parseInt(process.env.MAILPIT_WAIT_TIMEOUT_MS || '30000', 10);

function buildUser(testInfo, prefix, options = {}) {
  const workerIndex = testInfo && Number.isInteger(testInfo.workerIndex) ? testInfo.workerIndex : 0;
  const rawId = testInfo && testInfo.testId
    ? testInfo.testId
    : crypto.randomUUID();
  const safeId = String(rawId).toLowerCase().replace(/[^a-z0-9]/g, '');
  const baseId = safeId.slice(-8) || Date.now().toString(36).slice(-8);
  const safePrefix = String(prefix || 'user')
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '')
    .slice(0, 10) || 'user';
  const base = `${safePrefix}${workerIndex}${baseId}`;
  return {
    username: options.username || base,
    email: options.email || `${base}@test.com`,
    password: options.password || 'Password1',
  };
}

async function register(page, user, { searchable = false } = {}) {
  await page.goto('/#register', { waitUntil: 'domcontentloaded' });
  await expect(page.getByRole('heading', { name: 'Create Account' })).toBeVisible();
  await page.locator('#register-form #username').fill(user.username);
  await page.locator('#register-form #email').fill(user.email);
  await page.locator('#register-form #password').fill(user.password);
  if (searchable) {
    await page.locator('#register-form #searchable').check();
  }
  await page.getByRole('button', { name: 'Create Account' }).click();
  await expect(page.getByRole('heading', { name: 'Create New Card' })).toBeVisible();
}

async function loginWithCredentials(page, email, password) {
  await page.goto('/#login');
  await page.locator('#login-form #email').fill(email);
  await page.locator('#login-form #password').fill(password);
  await page.evaluate(() => {
    document.getElementById('login-form')?.requestSubmit();
  });
  await expect(page.getByRole('heading', { name: 'My Bingo Cards' })).toBeVisible();
}

async function createCardFromAuthenticatedCreate(page, { title } = {}) {
  await expect(page.getByRole('heading', { name: 'Create New Card' })).toBeVisible();
  if (title) {
    await page.fill('#card-title', title);
  }
  await page.locator('#create-card-form button[type="submit"]').click();
  await expect(page.locator('#item-input')).toBeVisible();
}

async function createCardFromModal(page, { title, gridSize, header, hasFree = true, year } = {}) {
  await page.goto('/#dashboard');
  await expect(page.getByRole('heading', { name: 'My Bingo Cards' })).toBeVisible();
  const addButton = page.getByRole('button', { name: '+ Card' });
  if (await addButton.isVisible()) {
    await addButton.click();
    await expect(page.locator('#modal-title')).toHaveText('Create New Card');

    if (title) {
      await page.fill('#modal-card-title', title);
    }
    if (year) {
      await page.selectOption('#modal-card-year', String(year));
    }
    if (gridSize) {
      await page.selectOption('#modal-card-grid-size', String(gridSize));
    }
    if (header) {
      await page.fill('#modal-card-header', header);
    }
    if (!hasFree) {
      const freeToggle = page.locator('#modal-card-free-space');
      if (await freeToggle.isChecked()) {
        await freeToggle.uncheck();
      }
    }

    await page.getByRole('button', { name: 'Create Card' }).click();
    await expect(page.locator('#item-input')).toBeVisible();
    return;
  }

  await page.goto('/#create');
  await expect(page.getByRole('heading', { name: 'Create New Card' })).toBeVisible();
  if (year) {
    await page.selectOption('#card-year', String(year));
  }
  if (title) {
    await page.fill('#card-title', title);
  }
  await page.locator('#create-card-form button[type="submit"]').click();
  await expect(page.locator('#item-input')).toBeVisible();
  if (header) {
    await page.fill('#card-header-input', header);
    await page.dispatchEvent('#card-header-input', 'change');
  }
  if (!hasFree) {
    const freeToggle = page.locator('#card-free-toggle');
    if (await freeToggle.isChecked()) {
      await freeToggle.uncheck();
    }
  }
}

async function createFinalizedCardFromModal(page, options) {
  await createCardFromModal(page, options);
  await fillCardWithSuggestions(page);
  await finalizeCard(page);
}

async function fillCardWithSuggestions(page) {
  const fillButton = page.locator('#fill-empty-btn');
  await expect(fillButton).toBeEnabled();
  await fillButton.click();
  await expect(page.locator('.editor-actions').getByRole('button', { name: /Finalize Card/i })).toBeEnabled();
}

async function finalizeCard(page, { visibleToFriends = true } = {}) {
  await page.locator('.editor-actions').getByRole('button', { name: /Finalize Card/i }).click();
  const modal = page.locator('#modal-overlay');
  await expect(modal).toHaveClass(/modal-overlay--visible/);
  if (!visibleToFriends) {
    const checkbox = modal.locator('#finalize-visibility');
    await expect(checkbox).toBeVisible();
    if (await checkbox.isChecked()) {
      await checkbox.uncheck();
    }
  }
  await modal.getByRole('button', { name: 'Finalize Card' }).click();
  await expect(page.locator('.finalized-card-view')).toBeVisible();
}

async function completeFirstItem(page) {
  await page.locator('.bingo-cell:not(.bingo-cell--free)').first().click();
  await expect(page.getByRole('button', { name: 'Mark Complete' })).toBeVisible();
  await page.getByRole('button', { name: 'Mark Complete' }).click();
}

async function logout(page) {
  await page.goto('/#profile');
  await page.getByRole('button', { name: 'Sign Out' }).click();
  await expect(page.getByRole('heading', { name: 'Year of Bingo' })).toBeVisible();
}

async function expectToast(page, message) {
  const toast = page.locator('#toast-container .toast').last();
  await expect(toast).toContainText(message);
}

async function ensureSelectedCount(page, expected) {
  const selectAll = page.getByRole('button', { name: 'Select All', exact: true });
  const deselectAll = page.getByRole('button', { name: 'Deselect All', exact: true });

  for (let attempts = 0; attempts < 3; attempts += 1) {
    const text = await page.locator('#selected-count').innerText();
    const current = Number.parseInt(text, 10);
    if (Number.isFinite(current) && current === expected) {
      return;
    }

    if (current > expected) {
      await deselectAll.click();
    } else {
      await selectAll.click();
    }
  }

  const finalText = await page.locator('#selected-count').innerText();
  throw new Error(`Unable to reach ${expected} selected cards (got "${finalText}")`);
}

async function sendFriendRequest(page, username) {
  await page.goto('/#friends');
  await page.fill('#friend-search', username);
  await page.click('#search-btn');
  const results = page.locator('#search-results');
  await expect(results).toContainText(username);
  const requestResponse = page.waitForResponse((response) => (
    response.url().includes('/api/friends/requests')
      && response.request().method() === 'POST'
      && response.ok()
  ));
  await results.getByRole('button', { name: 'Add Friend' }).click();
  await requestResponse;
  await expectToast(page, 'Friend request sent!');
}

async function clearMailpit(request) {
  const response = await request.delete(`${MAILPIT_BASE_URL}/api/v1/messages`);
  if (!response.ok()) {
    return;
  }
}

function getMessageId(message) {
  return message.ID || message.id || message.Id || null;
}

function getMessageSubject(message) {
  return message.Subject || message.subject || '';
}

function getMessageRecipients(message) {
  const to = message.To || message.to || message.Recipients || message.recipients || [];
  if (Array.isArray(to)) {
    return to.map((entry) => {
      if (!entry) return '';
      if (typeof entry === 'string') return entry;
      return entry.Address || entry.address || entry.Email || entry.email || entry.Mailbox || '';
    }).filter(Boolean);
  }
  if (typeof to === 'string') {
    return [to];
  }
  return [];
}

function getMessageCreated(message) {
  const created = message.Created || message.created || message.Date || message.date || '';
  const parsed = Date.parse(created);
  return Number.isNaN(parsed) ? 0 : parsed;
}

function pickLatestMessage(messages) {
  if (messages.length === 0) return null;
  const sorted = [...messages].sort((a, b) => getMessageCreated(b) - getMessageCreated(a));
  return sorted[0];
}

function getMessageBody(message) {
  return message.Text || message.text || message.HTML || message.html || message.Body || message.body || '';
}

async function waitForEmail(request, { to, subject, timeout = MAILPIT_WAIT_TIMEOUT_MS, after = null } = {}) {
  const start = Date.now();
  const lowerTo = String(to || '').toLowerCase();
  const lowerSubject = subject ? String(subject).toLowerCase() : '';

  while (Date.now() - start < timeout) {
    const response = await request.get(`${MAILPIT_BASE_URL}/api/v1/messages`);
    if (response.ok()) {
      let data = null;
      try {
        data = await response.json();
      } catch (error) {
        data = null;
      }

      const messages = (data && (data.messages || data.Messages || data.items)) || [];
      const filtered = messages.filter((message) => {
        if (typeof after === 'number' && after > 0) {
          const createdAt = getMessageCreated(message);
          if (!createdAt || createdAt <= after) return false;
        }
        const recipients = getMessageRecipients(message).map((recipient) => recipient.toLowerCase());
        const matchesRecipient = !lowerTo || recipients.some((recipient) => recipient.includes(lowerTo));
        const matchesSubject = !lowerSubject || getMessageSubject(message).toLowerCase().includes(lowerSubject);
        return matchesRecipient && matchesSubject;
      });

      const match = pickLatestMessage(filtered);
      if (match) {
        const messageId = getMessageId(match);
        if (!messageId) {
          throw new Error('Mailpit message missing ID');
        }

        const messageResponse = await request.get(`${MAILPIT_BASE_URL}/api/v1/message/${messageId}`);
        if (messageResponse.ok()) {
          return messageResponse.json();
        }
      }
    }

    await new Promise((resolve) => setTimeout(resolve, 500));
  }

  throw new Error(`Timed out waiting for email${to ? ` to ${to}` : ''}${subject ? ` with subject ${subject}` : ''}`);
}

async function expectNoEmail(request, { to, subject, timeout = 3000, after = null } = {}) {
  const start = Date.now();
  const lowerTo = String(to || '').toLowerCase();
  const lowerSubject = subject ? String(subject).toLowerCase() : '';

  while (Date.now() - start < timeout) {
    const response = await request.get(`${MAILPIT_BASE_URL}/api/v1/messages`);
    if (response.ok()) {
      let data = null;
      try {
        data = await response.json();
      } catch (error) {
        data = null;
      }

      const messages = (data && (data.messages || data.Messages || data.items)) || [];
      const filtered = messages.filter((message) => {
        if (typeof after === 'number' && after > 0) {
          const createdAt = getMessageCreated(message);
          if (!createdAt || createdAt <= after) return false;
        }
        const recipients = getMessageRecipients(message).map((recipient) => recipient.toLowerCase());
        const matchesRecipient = !lowerTo || recipients.some((recipient) => recipient.includes(lowerTo));
        const matchesSubject = !lowerSubject || getMessageSubject(message).toLowerCase().includes(lowerSubject);
        return matchesRecipient && matchesSubject;
      });

      if (filtered.length > 0) {
        throw new Error(`Unexpected email received${to ? ` to ${to}` : ''}${subject ? ` with subject ${subject}` : ''}`);
      }
    }

    await new Promise((resolve) => setTimeout(resolve, 250));
  }
}

function extractTokenFromEmail(message, route) {
  const body = getMessageBody(message);
  const tokenMatch = body.match(new RegExp(`#${route}\\?token=([a-f0-9]+)`, 'i'));
  if (!tokenMatch) {
    throw new Error(`Unable to find ${route} token in email`);
  }
  return tokenMatch[1];
}

module.exports = {
  buildUser,
  register,
  loginWithCredentials,
  createCardFromAuthenticatedCreate,
  createCardFromModal,
  createFinalizedCardFromModal,
  fillCardWithSuggestions,
  finalizeCard,
  completeFirstItem,
  logout,
  expectToast,
  ensureSelectedCount,
  sendFriendRequest,
  clearMailpit,
  waitForEmail,
  expectNoEmail,
  extractTokenFromEmail,
};
