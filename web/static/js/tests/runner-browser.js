(() => {
  // Test framework
  let testCount = 0;
  let passCount = 0;
  let failCount = 0;
  let currentSuite = '';
  let currentSuiteEl = null;
  let results = [];

  const PASS_ICON = '&#10003;';
  const FAIL_ICON = '&#10007;';

  function describe(name, fn) {
    currentSuite = name;
    const suiteDiv = document.createElement('div');
    suiteDiv.className = 'suite';
    suiteDiv.innerHTML = `<div class="suite-header">${escapeHtml(name)}</div>`;
    currentSuiteEl = suiteDiv;
    document.getElementById('results').appendChild(suiteDiv);
    fn();
  }

  function test(name, fn) {
    testCount++;
    const testDiv = document.createElement('div');
    testDiv.className = 'test';

    try {
      fn();
      passCount++;
      testDiv.innerHTML = `
        <span class="test-icon pass">${PASS_ICON}</span>
        <span class="test-name">${escapeHtml(name)}</span>
      `;
      results.push({ suite: currentSuite, name, passed: true });
    } catch (error) {
      failCount++;
      testDiv.innerHTML = `
        <span class="test-icon fail">${FAIL_ICON}</span>
        <span class="test-name">${escapeHtml(name)}</span>
      `;
      const errorDiv = document.createElement('div');
      errorDiv.className = 'test-error';
      errorDiv.textContent = error.message;
      currentSuiteEl.appendChild(testDiv);
      currentSuiteEl.appendChild(errorDiv);
      results.push({ suite: currentSuite, name, passed: false, error: error.message });
      return;
    }

    currentSuiteEl.appendChild(testDiv);
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
      toBeNull() {
        if (actual !== null) {
          throw new Error(`Expected null but got ${JSON.stringify(actual)}`);
        }
      },
      toBeUndefined() {
        if (actual !== undefined) {
          throw new Error(`Expected undefined but got ${JSON.stringify(actual)}`);
        }
      },
      toBeDefined() {
        if (actual === undefined) {
          throw new Error('Expected defined value but got undefined');
        }
      },
      toContain(expected) {
        if (typeof actual === 'string') {
          if (!actual.includes(expected)) {
            throw new Error(`Expected "${actual}" to contain "${expected}"`);
          }
        } else if (Array.isArray(actual)) {
          if (!actual.includes(expected)) {
            throw new Error(`Expected array to contain ${JSON.stringify(expected)}`);
          }
        }
      },
      toHaveLength(expected) {
        if (actual.length !== expected) {
          throw new Error(`Expected length ${expected} but got ${actual.length}`);
        }
      },
      toThrow() {
        if (typeof actual !== 'function') {
          throw new Error('Expected a function');
        }
        let threw = false;
        try {
          actual();
        } catch (e) {
          threw = true;
        }
        if (!threw) {
          throw new Error('Expected function to throw');
        }
      },
      toBeGreaterThan(expected) {
        if (actual <= expected) {
          throw new Error(`Expected ${actual} to be greater than ${expected}`);
        }
      },
      toBeLessThan(expected) {
        if (actual >= expected) {
          throw new Error(`Expected ${actual} to be less than ${expected}`);
        }
      },
      toBeInstanceOf(expected) {
        if (!(actual instanceof expected)) {
          throw new Error(`Expected instance of ${expected.name}`);
        }
      }
    };
  }

  function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  function runTests() {
    // Reset
    testCount = 0;
    passCount = 0;
    failCount = 0;
    results = [];
    document.getElementById('results').innerHTML = '';
    document.getElementById('summary').style.display = 'none';
    document.getElementById('run-btn').disabled = true;
    document.getElementById('run-btn').textContent = 'Running...';

    // Run tests after a brief delay to update UI
    setTimeout(() => {
      runAllTests();

      // Update summary
      document.getElementById('summary').style.display = 'flex';
      document.getElementById('total-count').textContent = testCount;
      document.getElementById('pass-count').textContent = passCount;
      document.getElementById('fail-count').textContent = failCount;

      document.getElementById('run-btn').disabled = false;
      document.getElementById('run-btn').textContent = 'Run Tests';
    }, 50);
  }

  // ============================================================
  // TEST CASES
  // ============================================================

  function runAllTests() {

    // --- Utility Functions ---

    function truncateText(text, maxLength) {
      if (text.length <= maxLength) return text;
      const truncated = text.substring(0, maxLength);
      const lastSpace = truncated.lastIndexOf(' ');
      if (lastSpace > maxLength * 0.5) {
        return truncated.substring(0, lastSpace) + '...';
      }
      return truncated + '...';
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

    function checkBingo(grid) {
      const bingos = [];
      for (let row = 0; row < 5; row++) {
        const rowComplete = [0, 1, 2, 3, 4].every(col => grid[row * 5 + col]);
        if (rowComplete) bingos.push({ type: 'row', index: row });
      }
      for (let col = 0; col < 5; col++) {
        const colComplete = [0, 1, 2, 3, 4].every(row => grid[row * 5 + col]);
        if (colComplete) bingos.push({ type: 'col', index: col });
      }
      if ([0, 6, 12, 18, 24].every(i => grid[i])) {
        bingos.push({ type: 'diagonal', index: 0 });
      }
      if ([4, 8, 12, 16, 20].every(i => grid[i])) {
        bingos.push({ type: 'diagonal', index: 1 });
      }
      return bingos;
    }

    // --- Tests ---

    describe('escapeHtml', () => {
      test('escapes < and >', () => {
        expect(escapeHtml('<script>')).toBe('&lt;script&gt;');
      });

      test('escapes ampersand', () => {
        expect(escapeHtml('foo & bar')).toBe('foo &amp; bar');
      });

      test('handles empty string', () => {
        expect(escapeHtml('')).toBe('');
      });

      test('handles plain text', () => {
        expect(escapeHtml('hello world')).toBe('hello world');
      });
    });

    describe('truncateText', () => {
      test('returns text unchanged if shorter than maxLength', () => {
        expect(truncateText('short', 10)).toBe('short');
      });

      test('truncates at space if available', () => {
        const result = truncateText('hello world this is a test', 15);
        expect(result).toBe('hello world...');
      });

      test('truncates at maxLength if no good space', () => {
        const result = truncateText('abcdefghijklmnop', 10);
        expect(result).toBe('abcdefghij...');
      });

      test('handles exact length', () => {
        expect(truncateText('hello', 5)).toBe('hello');
      });
    });

    describe('parsePath', () => {
      test('parses simple path', () => {
        const result = parsePath('/dashboard');
        expect(result.page).toBe('dashboard');
        expect(result.params).toEqual([]);
      });

      test('parses path with params', () => {
        const result = parsePath('/card/abc-123');
        expect(result.page).toBe('card');
        expect(result.params).toEqual(['abc-123']);
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

    describe('isValidPosition', () => {
      test('returns true for valid positions', () => {
        expect(isValidPosition(0)).toBeTruthy();
        expect(isValidPosition(11)).toBeTruthy();
        expect(isValidPosition(13)).toBeTruthy();
        expect(isValidPosition(24)).toBeTruthy();
      });

      test('returns false for free space (12)', () => {
        expect(isValidPosition(12)).toBeFalsy();
      });

      test('returns false for negative positions', () => {
        expect(isValidPosition(-1)).toBeFalsy();
      });

      test('returns false for out of range', () => {
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

      test('handles zero total', () => {
        expect(calculateProgress(0, 0)).toBe(0);
      });

      test('calculates 50% correctly', () => {
        expect(calculateProgress(12, 24)).toBe(50);
      });
    });

    describe('checkBingo', () => {
      test('detects no bingo on empty grid', () => {
        const grid = Array(25).fill(false);
        grid[12] = true;
        expect(checkBingo(grid)).toEqual([]);
      });

      test('detects first row bingo', () => {
        const grid = Array(25).fill(false);
        grid[12] = true;
        grid[0] = grid[1] = grid[2] = grid[3] = grid[4] = true;
        const bingos = checkBingo(grid);
        expect(bingos.length).toBe(1);
        expect(bingos[0].type).toBe('row');
      });

      test('detects middle row with free space', () => {
        const grid = Array(25).fill(false);
        grid[10] = grid[11] = grid[12] = grid[13] = grid[14] = true;
        const bingos = checkBingo(grid);
        expect(bingos.length).toBe(1);
      });

      test('detects column bingo', () => {
        const grid = Array(25).fill(false);
        grid[12] = true;
        grid[0] = grid[5] = grid[10] = grid[15] = grid[20] = true;
        const bingos = checkBingo(grid);
        expect(bingos.length).toBe(1);
        expect(bingos[0].type).toBe('col');
      });

      test('detects diagonal bingo', () => {
        const grid = Array(25).fill(false);
        grid[0] = grid[6] = grid[12] = grid[18] = grid[24] = true;
        const bingos = checkBingo(grid);
        expect(bingos.length).toBe(1);
        expect(bingos[0].type).toBe('diagonal');
      });

      test('detects all bingos when complete', () => {
        const grid = Array(25).fill(true);
        const bingos = checkBingo(grid);
        expect(bingos.length).toBe(12);
      });
    });

    describe('Grid Position Calculations', () => {
      test('position 12 is center', () => {
        const row = Math.floor(12 / 5);
        const col = 12 % 5;
        expect(row).toBe(2);
        expect(col).toBe(2);
      });

      test('first row positions are 0-4', () => {
        for (let i = 0; i < 5; i++) {
          expect(Math.floor(i / 5)).toBe(0);
        }
      });

      test('diagonal positions are correct', () => {
        const diagonal1 = [0, 6, 12, 18, 24];
        diagonal1.forEach((pos, i) => {
          expect(pos).toBe(i * 5 + i);
        });
      });
    });

    describe('API Client Structure', () => {
      test('CSRF token methods exist on API', () => {
        expect(typeof API).toBe('object');
        expect(API.csrfToken).toBe(null);
      });

      test('auth namespace exists', () => {
        expect(typeof API.auth).toBe('object');
        expect(typeof API.auth.login).toBe('function');
      });

      test('cards namespace exists', () => {
        expect(typeof API.cards).toBe('object');
        expect(typeof API.cards.create).toBe('function');
      });

      test('friends namespace exists', () => {
        expect(typeof API.friends).toBe('object');
        expect(typeof API.friends.list).toBe('function');
      });

      test('notifications namespace exists', () => {
        expect(typeof API.notifications).toBe('object');
        expect(typeof API.notifications.list).toBe('function');
        expect(typeof API.notifications.markRead).toBe('function');
        expect(typeof API.notifications.markAllRead).toBe('function');
        expect(typeof API.notifications.delete).toBe('function');
        expect(typeof API.notifications.deleteAll).toBe('function');
      });

      test('reactions namespace exists', () => {
        expect(typeof API.reactions).toBe('object');
        expect(typeof API.reactions.add).toBe('function');
      });
    });

    describe('App Object Structure', () => {
      test('App has user property', () => {
        expect(App.user).toBe(null);
      });

      test('App has route method', () => {
        expect(typeof App.route).toBe('function');
      });

      test('App has escapeHtml method', () => {
        expect(typeof App.escapeHtml).toBe('function');
      });

      test('App has toast method', () => {
        expect(typeof App.toast).toBe('function');
      });

      test('App has allowedEmojis', () => {
        expect(Array.isArray(App.allowedEmojis)).toBeTruthy();
        expect(App.allowedEmojis.length).toBeGreaterThan(0);
      });
    });
  }

  const runBtn = document.getElementById('run-btn');
  if (runBtn) {
    runBtn.addEventListener('click', runTests);
  }

  // Auto-run on load if running via file://
  if (window.location.protocol === 'file:') {
    if (document.readyState === 'complete' || document.readyState === 'interactive') {
      runTests();
    } else {
      document.addEventListener('DOMContentLoaded', runTests, { once: true });
    }
  }

  if (!window.runTests) {
    window.runTests = runTests;
  }
})();
