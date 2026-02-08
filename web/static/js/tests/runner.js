#!/usr/bin/env node
/**
 * Year of Bingo - JavaScript Test Runner
 *
 * Zero dependencies - uses only Node.js built-ins.
 * Run with: node web/static/js/tests/runner.js
 */

const fs = require('fs');
const path = require('path');
const vm = require('vm');

// Test state
let testCount = 0;
let passCount = 0;
let failCount = 0;
let currentSuite = '';
const registeredTests = [];

const colors = {
  reset: '\x1b[0m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  dim: '\x1b[2m',
};

function describe(name, fn) {
  currentSuite = name;
  console.log(`\n${colors.blue}${name}${colors.reset}`);
  fn(); // Registers tests via test(...)
}

function test(name, fn) {
  registeredTests.push({ suite: currentSuite, name, fn });
}

function expect(actual) {
  return {
    toBe(expected) {
      if (actual !== expected) {
        throw new Error(`Expected ${JSON.stringify(expected)} but got ${JSON.stringify(actual)}`);
      }
    },
    toEqual(expected) {
      if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        throw new Error(`Expected ${JSON.stringify(expected)} but got ${JSON.stringify(actual)}`);
      }
    },
    toBeTruthy() {
      if (!actual) {
        throw new Error(`Expected truthy value but got ${JSON.stringify(actual)}`);
      }
    },
    toBeFalsy() {
      if (actual) {
        throw new Error(`Expected falsy value but got ${JSON.stringify(actual)}`);
      }
    },
    toBeGreaterThan(expected) {
      if (actual <= expected) {
        throw new Error(`Expected ${actual} to be greater than ${expected}`);
      }
    },
  };
}

// ============================================================
// UTILITY FUNCTIONS TO TEST (extracted from app.js)
// ============================================================

function escapeHtml(text) {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

function truncateText(text, maxLength) {
  if (text.length <= maxLength) return text;
  const truncated = text.substring(0, maxLength);
  const lastSpace = truncated.lastIndexOf(' ');
  if (lastSpace > maxLength * 0.5) {
    return truncated.substring(0, lastSpace) + '…';
  }
  return truncated + '…';
}

function parsePath(path) {
  if (!path) return { page: 'home', params: [] };
  let cleanPath = path.startsWith('/') ? path.slice(1) : path;
  if (cleanPath.endsWith('/')) {
    cleanPath = cleanPath.slice(0, -1);
  }
  const [page, ...params] = cleanPath.split('/');
  return { page: page || 'home', params };
}

function isValidPosition(position) {
  const FREE_SPACE = 12;
  const TOTAL_SQUARES = 25;
  return position >= 0 && position < TOTAL_SQUARES && position !== FREE_SPACE;
}

function calculateProgress(completed, total) {
  if (total === 0) return 0;
  return Math.round((completed / total) * 100);
}

function matchesDeleteAccountConfirmation(username, inputValue) {
  return inputValue.trim() === username;
}

function checkBingo(grid) {
  const bingos = [];

  // Check rows
  for (let row = 0; row < 5; row++) {
    if ([0, 1, 2, 3, 4].every(col => grid[row * 5 + col])) {
      bingos.push({ type: 'row', index: row });
    }
  }

  // Check columns
  for (let col = 0; col < 5; col++) {
    if ([0, 1, 2, 3, 4].every(row => grid[row * 5 + col])) {
      bingos.push({ type: 'col', index: col });
    }
  }

  // Check diagonals
  if ([0, 6, 12, 18, 24].every(i => grid[i])) {
    bingos.push({ type: 'diagonal', index: 0 });
  }
  if ([4, 8, 12, 16, 20].every(i => grid[i])) {
    bingos.push({ type: 'diagonal', index: 1 });
  }

  return bingos;
}

function countBingos(items) {
  const grid = Array(25).fill(false);
  grid[12] = true; // Free space

  for (const item of items) {
    if (item.is_completed) {
      grid[item.position] = true;
    }
  }

  return checkBingo(grid).length;
}

// ============================================================
// TESTS
// ============================================================

console.log(`${colors.blue}Year of Bingo JavaScript Tests${colors.reset}`);
console.log('='.repeat(40));

describe('escapeHtml', () => {
  test('escapes < and >', () => {
    expect(escapeHtml('<script>')).toBe('&lt;script&gt;');
  });

  test('escapes ampersand', () => {
    expect(escapeHtml('foo & bar')).toBe('foo &amp; bar');
  });

  test('escapes quotes', () => {
    expect(escapeHtml('"hello"')).toBe('&quot;hello&quot;');
  });

  test('escapes single quotes', () => {
    expect(escapeHtml("it's")).toBe("it&#039;s");
  });

  test('handles empty string', () => {
    expect(escapeHtml('')).toBe('');
  });

  test('handles plain text unchanged', () => {
    expect(escapeHtml('hello world')).toBe('hello world');
  });
});

describe('truncateText', () => {
  test('returns text unchanged if shorter than maxLength', () => {
    expect(truncateText('short', 10)).toBe('short');
  });

  test('truncates at space if available', () => {
    const result = truncateText('hello world this is a test', 15);
    expect(result).toBe('hello world…');
  });

  test('truncates at maxLength if no good space', () => {
    const result = truncateText('abcdefghijklmnop', 10);
    expect(result).toBe('abcdefghij…');
  });

  test('handles exact length', () => {
    expect(truncateText('hello', 5)).toBe('hello');
  });
});

describe('Premium navigation + page wiring', () => {
  class MockClassList {
    constructor() {
      this._set = new Set();
    }
    add(...names) { names.forEach((n) => this._set.add(n)); }
    remove(...names) { names.forEach((n) => this._set.delete(n)); }
    contains(name) { return this._set.has(name); }
  }

  class MockElement {
    constructor(doc, { id = '', tagName = 'DIV' } = {}) {
      this._doc = doc;
      this.id = id;
      this.tagName = tagName;
      this.classList = new MockClassList();
      this.style = {};
      this._innerHTML = '';
    }
    set innerHTML(html) {
      this._innerHTML = String(html || '');
      this._doc._indexIDsFromHTML(this._innerHTML);
    }
    get innerHTML() {
      return this._innerHTML;
    }
    setAttribute() {}
    getAttribute() { return null; }
    remove() {}
  }

  class MockDocument {
    constructor() {
      this._byID = new Map();
      this.head = new MockElement(this, { tagName: 'HEAD' });
      this.body = new MockElement(this, { tagName: 'BODY' });
      this._listeners = {};
      this._pageEl = new MockElement(this, { tagName: 'DIV' });
      this._pageEl.classList.add('page');
    }
    addEventListener(type, cb) {
      this._listeners[type] = cb;
    }
    createElement(tag) {
      return new MockElement(this, { tagName: String(tag || 'DIV').toUpperCase() });
    }
    getElementById(id) {
      return this._byID.get(id) || null;
    }
    querySelector(selector) {
      if (selector === '.page') return this._pageEl;
      return null;
    }
    _indexIDsFromHTML(html) {
      const re = /id=\"([a-zA-Z0-9_-]+)\"/g;
      let m;
      while ((m = re.exec(html)) !== null) {
        const id = m[1];
        if (!this._byID.has(id)) {
          this._byID.set(id, new MockElement(this, { id }));
        }
      }
    }
  }

  function loadBrowserApp() {
    const doc = new MockDocument();
    doc._byID.set('nav', new MockElement(doc, { id: 'nav', tagName: 'NAV' }));
    doc._byID.set('main-container', new MockElement(doc, { id: 'main-container' }));

    class MockStorage {
      constructor() { this._m = new Map(); }
      getItem(k) { return this._m.has(String(k)) ? this._m.get(String(k)) : null; }
      setItem(k, v) { this._m.set(String(k), String(v)); }
      removeItem(k) { this._m.delete(String(k)); }
      clear() { this._m.clear(); }
    }

    const win = {
      location: { pathname: '/', search: '', hash: '', origin: 'http://example.test' },
      addEventListener() {},
      scrollTo() {},
      history: { replaceState() {} },
    };

    const context = {
      console,
      window: win,
      document: doc,
      history: win.history,
      navigator: { onLine: true },
      localStorage: new MockStorage(),
      sessionStorage: new MockStorage(),
      URL,
      URLSearchParams,
      setTimeout,
      clearTimeout,
      setInterval,
      clearInterval,
    };

    const appJsPath = path.join(__dirname, '..', 'app.js');
    const code = fs.readFileSync(appJsPath, 'utf8') + '\n;globalThis.__AppForTests = App;\n';
    vm.createContext(context);
    vm.runInContext(code, context, { filename: 'app.js' });

    const AppForTests = context.__AppForTests;
    if (!AppForTests) {
      throw new Error('Failed to load App from app.js');
    }

    // Prevent any incidental work in route()/meta helpers.
    AppForTests.setRobotsMeta = () => {};

    return { App: AppForTests, document: doc, window: win };
  }

  globalThis.__loadBrowserAppForTests = loadBrowserApp;

  test('router supports /premium', () => {
    const { App } = loadBrowserApp();
    const { page } = App.getRouteFromPath('/premium', '');
    expect(page).toBe('premium');
  });

  test('route() closes any open modal overlay', () => {
    const { App, window } = loadBrowserApp();
    let closed = 0;
    App.closeMobileMenu = () => {};
    App.closeModal = () => { closed += 1; };
    App.renderPremium = () => {};
    window.location.pathname = '/premium';
    window.location.search = '';
    App.route();
    expect(closed).toBeGreaterThan(0);
  });

  test('premium route is considered SPA-routable', () => {
    const { App } = loadBrowserApp();
    expect(App.isRoutablePage('premium')).toBe(true);
  });

  test('templates route redirects non-premium users to premium', () => {
    const { App, window } = loadBrowserApp();
    App.user = { username: 'alice' };
    App.isPremium = false;
    App.closeMobileMenu = () => {};
    App.closeModal = () => {};
    App.renderPremium = () => {};

    let redirectedTo = '';
    App.navigate = (path) => { redirectedTo = path; };

    window.location.pathname = '/templates';
    window.location.search = '';
    App.route();
    expect(redirectedTo).toBe('/premium?upgrade=1');
  });

  test('templates route renders for premium users', () => {
    const { App, window } = loadBrowserApp();
    App.user = { username: 'alice' };
    App.isPremium = true;
    App.closeMobileMenu = () => {};
    App.closeModal = () => {};
    App.renderPremium = () => {};

    let rendered = false;
    App.renderTemplates = () => { rendered = true; };

    window.location.pathname = '/templates';
    window.location.search = '';
    App.route();
    expect(rendered).toBe(true);
  });

  test('templates route respects per-feature entitlements over premium', () => {
    const { App, window } = loadBrowserApp();
    App.user = { username: 'alice' };
    App.isPremium = true;
    App.entitlements = { templates: false };
    App.closeMobileMenu = () => {};
    App.closeModal = () => {};
    App.renderPremium = () => {};

    let redirectedTo = '';
    App.navigate = (path) => { redirectedTo = path; };

    window.location.pathname = '/templates';
    window.location.search = '';
    App.route();
    expect(redirectedTo).toBe('/premium?upgrade=1');
  });

  test('free users are upsold when opening finalized Edit flow', () => {
    const { App } = loadBrowserApp();
    App.user = { username: 'alice' };
    App.isPremium = false;
    App.entitlements = {};
    App.currentCard = {
      id: 'card-1',
      year: 2026,
      title: 'My Card',
      is_finalized: true,
    };

    let upgradeCalls = 0;
    let modalCalls = 0;
    App.openUpgradeModal = () => { upgradeCalls += 1; };
    App.openModal = () => { modalCalls += 1; };

    App.showEditFinalizedCardModal();
    expect(upgradeCalls).toBe(1);
    expect(modalCalls).toBe(0);
  });

  test('premium users can open finalized Edit modal', () => {
    const { App } = loadBrowserApp();
    App.user = { username: 'alice' };
    App.isPremium = true;
    App.entitlements = { edit_after_finalize: true };
    App.currentCard = {
      id: 'card-1',
      year: 2026,
      title: 'My Card',
      is_finalized: true,
    };

    let modalTitle = '';
    let modalHTML = '';
    App.openModal = (title, html) => {
      modalTitle = title;
      modalHTML = String(html || '');
    };

    App.showEditFinalizedCardModal();
    expect(modalTitle).toBe('Edit Finalized Card');
    expect(modalHTML.includes('data-action="edit-finalized-card"')).toBe(true);
    expect(modalHTML.includes('id="edit-finalized-card-reset"')).toBe(true);
  });

  test('finalized Edit flow respects per-feature entitlements over premium', () => {
    const { App } = loadBrowserApp();
    App.user = { username: 'alice' };
    App.isPremium = true;
    App.entitlements = { edit_after_finalize: false };
    App.currentCard = {
      id: 'card-1',
      year: 2026,
      title: 'My Card',
      is_finalized: true,
    };

    let upgradeCalls = 0;
    let modalCalls = 0;
    App.openUpgradeModal = () => { upgradeCalls += 1; };
    App.openModal = () => { modalCalls += 1; };

    App.showEditFinalizedCardModal();
    expect(upgradeCalls).toBe(1);
    expect(modalCalls).toBe(0);
  });

  test('navbar renders a Premium link without auto-opening upgrade', () => {
    const { App, document } = loadBrowserApp();
    App.user = { username: 'alice' };
    App.notificationUnreadCount = 0;
    App.updateNotificationBadge = () => {};
    App.setupNavigation();
    const nav = document.getElementById('nav');
    expect(!!nav).toBe(true);
    expect(nav.innerHTML.includes('nav-link--premium')).toBe(true);
    expect(nav.innerHTML.includes('href="/premium"')).toBe(true);
    expect(nav.innerHTML.includes('href="/premium?upgrade=1"')).toBe(false);
  });

  test('premium page does not include See options button', async () => {
    const { App, document } = loadBrowserApp();
    const container = document.getElementById('main-container');
    App.user = null;
    await App.renderPremium(container, new URLSearchParams());
    expect(container.innerHTML.includes('See options')).toBe(false);
  });

  test('premium page includes a Have a code entry point', async () => {
    const { App, document } = loadBrowserApp();
    const container = document.getElementById('main-container');
    App.user = null;
    await App.renderPremium(container, new URLSearchParams());
    const cta = document.getElementById('premium-cta-slot');
    expect(!!cta).toBe(true);
    expect(cta.innerHTML.includes('data-action="open-premium-code-modal"')).toBe(true);

    let modalHTML = '';
    App.openModal = (_title, html) => { modalHTML = String(html || ''); };
    App.openPremiumCodeModal({ errorMessage: 'Invalid code' });
    expect(modalHTML.includes('id="premium-code-error"')).toBe(true);
  });
});

describe('Module boundaries + action dispatch', () => {
  function readAppModule(fileName) {
    const filePath = path.join(__dirname, '..', fileName);
    return fs.readFileSync(filePath, 'utf8');
  }

  test('module scaffold files use App composition pattern', () => {
    const moduleFiles = [
      'app-core.js',
      'app-actions.js',
      'app-modals.js',
      'app-notifications.js',
      'app-reminders.js',
      'app-friends.js',
      'app-billing.js',
      'app-templates.js',
      'app-ai.js',
      'app-auth.js',
      'app-cards.js',
    ];
    moduleFiles.forEach((fileName) => {
      const source = readAppModule(fileName);
      expect(source.includes('Object.assign(App, {')).toBe(true);
    });
  });

  test('domain modules contain representative extracted methods', () => {
    expect(readAppModule('app-billing.js').includes('openUpgradeModal(')).toBe(true);
    expect(readAppModule('app-billing.js').includes('renderPremium(')).toBe(true);

    expect(readAppModule('app-templates.js').includes('renderTemplates(')).toBe(true);
    expect(readAppModule('app-templates.js').includes('handleCreateCardFromTemplate(')).toBe(true);

    expect(readAppModule('app-ai.js').includes('handleAIPremiumAssist(')).toBe(true);
    expect(readAppModule('app-ai.js').includes('fillEmptyWithAI(')).toBe(true);

    expect(readAppModule('app-auth.js').includes('renderLogin(')).toBe(true);
    expect(readAppModule('app-auth.js').includes('renderProfile(')).toBe(true);

    expect(readAppModule('app-cards.js').includes('showCreateCardModal(')).toBe(true);
    expect(readAppModule('app-cards.js').includes('renderCard(')).toBe(true);
  });

  test('handleActionClick dispatches extracted domain actions', () => {
    const { App } = globalThis.__loadBrowserAppForTests();
    let premiumCodeModalCount = 0;
    let createTemplateCount = 0;
    let aiFillCount = 0;
    let friendUserID = '';
    let viewedTemplateID = '';

    App.openPremiumCodeModal = () => { premiumCodeModalCount += 1; };
    App.showCreateTemplateModal = () => { createTemplateCount += 1; };
    App.fillEmptyWithAI = () => { aiFillCount += 1; };
    App.sendFriendRequest = (userID) => { friendUserID = userID; };
    App.showTemplateModal = (templateID) => { viewedTemplateID = templateID; };

    App.handleActionClick('open-premium-code-modal', { dataset: {} }, {});
    App.handleActionClick('show-create-template-modal', { dataset: {} }, {});
    App.handleActionClick('ai-fill-empty-premium', { dataset: {} }, {});
    App.handleActionClick('send-friend-request', { dataset: { userId: 'friend-123' } }, {});
    App.handleActionClick('view-template', { dataset: { templateId: 'tpl-007' } }, {});

    expect(premiumCodeModalCount).toBe(1);
    expect(createTemplateCount).toBe(1);
    expect(aiFillCount).toBe(1);
    expect(friendUserID).toBe('friend-123');
    expect(viewedTemplateID).toBe('tpl-007');
  });

  test('handleActionSubmit forwards template + item edit forms', () => {
    const { App } = globalThis.__loadBrowserAppForTests();
    const submitEvent = { type: 'submit' };
    const templateForm = { dataset: { templateId: 'tpl-abc' } };
    const itemForm = { dataset: { position: '7' } };

    let templateEvent = null;
    let templateFormSeen = null;
    let editEvent = null;
    let editPosition = null;
    let editFormSeen = null;

    App.handleCreateTemplate = (event, form) => {
      templateEvent = event;
      templateFormSeen = form;
    };
    App.saveItemEdit = (event, position, form) => {
      editEvent = event;
      editPosition = position;
      editFormSeen = form;
    };

    App.handleActionSubmit('create-template', templateForm, submitEvent);
    App.handleActionSubmit('save-item-edit', itemForm, submitEvent);

    expect(templateEvent).toBe(submitEvent);
    expect(templateFormSeen).toBe(templateForm);
    expect(editEvent).toBe(submitEvent);
    expect(editPosition).toBe(7);
    expect(editFormSeen).toBe(itemForm);
  });

  test('handleActionChange routes dashboard + reminder controls', () => {
    const { App } = globalThis.__loadBrowserAppForTests();
    const sortTarget = { value: 'year_desc' };
    const reminderTarget = { value: 'card-12' };

    let sortValue = '';
    let reminderTargetSeen = null;

    App.changeDashboardSort = (value) => { sortValue = value; };
    App.handleReminderCardSelect = (target) => { reminderTargetSeen = target; };

    App.handleActionChange('dashboard-sort', sortTarget, {});
    App.handleActionChange('reminder-card-select', reminderTarget, {});

    expect(sortValue).toBe('year_desc');
    expect(reminderTargetSeen).toBe(reminderTarget);
  });
});

describe('parsePath', () => {
  test('parses simple path', () => {
    const result = parsePath('/dashboard');
    expect(result.page).toBe('dashboard');
    expect(result.params).toEqual([]);
  });

  test('parses path with trailing slash', () => {
    const result = parsePath('/dashboard/');
    expect(result.page).toBe('dashboard');
    expect(result.params).toEqual([]);
  });

  test('parses path with params', () => {
    const result = parsePath('/card/abc-123');
    expect(result.page).toBe('card');
    expect(result.params).toEqual(['abc-123']);
  });

  test('parses path with multiple params', () => {
    const result = parsePath('/friend-card/123/2024');
    expect(result.page).toBe('friend-card');
    expect(result.params).toEqual(['123', '2024']);
  });

  test('parses share path', () => {
    const result = parsePath('/share/abc123');
    expect(result.page).toBe('share');
    expect(result.params).toEqual(['abc123']);
  });

  test('handles empty path', () => {
    const result = parsePath('');
    expect(result.page).toBe('home');
  });

  test('handles root path', () => {
    const result = parsePath('/');
    expect(result.page).toBe('home');
  });
});

describe('matchesDeleteAccountConfirmation', () => {
  test('matches after trimming whitespace', () => {
    expect(matchesDeleteAccountConfirmation('user123', '  user123  ')).toBe(true);
  });

  test('is case sensitive', () => {
    expect(matchesDeleteAccountConfirmation('User123', 'user123')).toBe(false);
  });

  test('rejects non-matching input', () => {
    expect(matchesDeleteAccountConfirmation('user123', 'user124')).toBe(false);
  });
});

describe('isValidPosition', () => {
  test('returns true for position 0', () => {
    expect(isValidPosition(0)).toBeTruthy();
  });

  test('returns true for position 24', () => {
    expect(isValidPosition(24)).toBeTruthy();
  });

  test('returns false for free space (12)', () => {
    expect(isValidPosition(12)).toBeFalsy();
  });

  test('returns false for negative positions', () => {
    expect(isValidPosition(-1)).toBeFalsy();
  });

  test('returns false for position >= 25', () => {
    expect(isValidPosition(25)).toBeFalsy();
  });
});

describe('calculateProgress', () => {
  test('returns 0 for 0 completed', () => {
    expect(calculateProgress(0, 24)).toBe(0);
  });

  test('returns 100 for all completed', () => {
    expect(calculateProgress(24, 24)).toBe(100);
  });

  test('returns 50 for half completed', () => {
    expect(calculateProgress(12, 24)).toBe(50);
  });

  test('handles zero total', () => {
    expect(calculateProgress(0, 0)).toBe(0);
  });

  test('rounds correctly', () => {
    expect(calculateProgress(1, 3)).toBe(33);
    expect(calculateProgress(2, 3)).toBe(67);
  });
});

describe('checkBingo', () => {
  test('detects no bingo on empty grid', () => {
    const grid = Array(25).fill(false);
    grid[12] = true; // Free space
    expect(checkBingo(grid).length).toBe(0);
  });

  test('detects first row bingo', () => {
    const grid = Array(25).fill(false);
    grid[0] = grid[1] = grid[2] = grid[3] = grid[4] = true;
    const bingos = checkBingo(grid);
    expect(bingos.length).toBe(1);
    expect(bingos[0].type).toBe('row');
    expect(bingos[0].index).toBe(0);
  });

  test('detects middle row with free space', () => {
    const grid = Array(25).fill(false);
    grid[10] = grid[11] = grid[12] = grid[13] = grid[14] = true;
    const bingos = checkBingo(grid);
    expect(bingos.length).toBe(1);
    expect(bingos[0].type).toBe('row');
    expect(bingos[0].index).toBe(2);
  });

  test('detects column bingo', () => {
    const grid = Array(25).fill(false);
    grid[0] = grid[5] = grid[10] = grid[15] = grid[20] = true;
    const bingos = checkBingo(grid);
    expect(bingos.length).toBe(1);
    expect(bingos[0].type).toBe('col');
  });

  test('detects diagonal (top-left to bottom-right)', () => {
    const grid = Array(25).fill(false);
    grid[0] = grid[6] = grid[12] = grid[18] = grid[24] = true;
    const bingos = checkBingo(grid);
    expect(bingos.length).toBe(1);
    expect(bingos[0].type).toBe('diagonal');
  });

  test('detects diagonal (top-right to bottom-left)', () => {
    const grid = Array(25).fill(false);
    grid[4] = grid[8] = grid[12] = grid[16] = grid[20] = true;
    const bingos = checkBingo(grid);
    expect(bingos.length).toBe(1);
    expect(bingos[0].type).toBe('diagonal');
  });

  test('detects all 12 bingos when grid is full', () => {
    const grid = Array(25).fill(true);
    const bingos = checkBingo(grid);
    expect(bingos.length).toBe(12); // 5 rows + 5 cols + 2 diagonals
  });
});

describe('countBingos (with items)', () => {
  test('returns 0 with no completed items', () => {
    const items = [
      { position: 0, is_completed: false },
      { position: 1, is_completed: false },
    ];
    expect(countBingos(items)).toBe(0);
  });

  test('counts row bingo correctly', () => {
    const items = [
      { position: 0, is_completed: true },
      { position: 1, is_completed: true },
      { position: 2, is_completed: true },
      { position: 3, is_completed: true },
      { position: 4, is_completed: true },
    ];
    expect(countBingos(items)).toBe(1);
  });

  test('counts middle row with free space', () => {
    // Middle row is 10, 11, 12 (free), 13, 14
    const items = [
      { position: 10, is_completed: true },
      { position: 11, is_completed: true },
      { position: 13, is_completed: true },
      { position: 14, is_completed: true },
    ];
    expect(countBingos(items)).toBe(1);
  });
});

describe('Grid Position Calculations', () => {
  test('position 12 is center (row 2, col 2)', () => {
    expect(Math.floor(12 / 5)).toBe(2);
    expect(12 % 5).toBe(2);
  });

  test('position 0 is top-left', () => {
    expect(Math.floor(0 / 5)).toBe(0);
    expect(0 % 5).toBe(0);
  });

  test('position 24 is bottom-right', () => {
    expect(Math.floor(24 / 5)).toBe(4);
    expect(24 % 5).toBe(4);
  });

  test('diagonal 1 positions are correct', () => {
    const diagonal = [0, 6, 12, 18, 24];
    diagonal.forEach((pos, i) => {
      expect(pos).toBe(i * 5 + i);
    });
  });

  test('diagonal 2 positions are correct', () => {
    const diagonal = [4, 8, 12, 16, 20];
    diagonal.forEach((pos, i) => {
      expect(pos).toBe(i * 5 + (4 - i));
    });
  });
});

// ============================================================
// SUMMARY
// ============================================================

async function runAll() {
  for (const t of registeredTests) {
    testCount++;
    try {
      const result = t.fn();
      if (result && typeof result.then === 'function') {
        await result;
      }
      passCount++;
      console.log(`  ${colors.green}✓${colors.reset} ${colors.dim}${t.name}${colors.reset}`);
    } catch (error) {
      failCount++;
      console.log(`  ${colors.red}✗ ${t.name}${colors.reset}`);
      console.log(`    ${colors.red}${error.message}${colors.reset}`);
    }
  }

  console.log('\n' + '='.repeat(40));
  console.log(`${colors.blue}Summary${colors.reset}`);
  console.log(`Total:  ${testCount}`);
  console.log(`${colors.green}Passed: ${passCount}${colors.reset}`);
  if (failCount > 0) {
    console.log(`${colors.red}Failed: ${failCount}${colors.reset}`);
  }

  // Coverage info
  const testedFunctions = [
    'escapeHtml',
    'truncateText',
    'parseHash',
    'isValidPosition',
    'calculateProgress',
    'checkBingo',
    'countBingos',
    'Grid calculations',
  ];
  console.log(`\n${colors.blue}Functions tested:${colors.reset}`);
  testedFunctions.forEach(fn => console.log(`  - ${fn}`));

  console.log('');
  process.exit(failCount > 0 ? 1 : 0);
}

runAll();
