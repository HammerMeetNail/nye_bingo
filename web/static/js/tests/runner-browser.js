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

  // Alias
  const it = test;

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

    function parseHash(hash) {
      const cleanHash = hash.startsWith('#') ? hash.slice(1) : hash;
      const [page, ...params] = cleanHash.split('/');
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
        const result = truncateText('averylongwordwithoutspaces', 10);
        expect(result).toBe('averylongw...');
      });
    });

    describe('parseHash', () => {
      test('parses simple hash', () => {
        const result = parseHash('#dashboard');
        expect(result.page).toBe('dashboard');
        expect(result.params).toEqual([]);
      });

      test('parses hash with params', () => {
        const result = parseHash('#card/123/456');
        expect(result.page).toBe('card');
        expect(result.params).toEqual(['123', '456']);
      });

      test('handles missing hash', () => {
        const result = parseHash('');
        expect(result.page).toBe('home');
      });
    });

    describe('isValidPosition', () => {
      test('validates positions correctly', () => {
        expect(isValidPosition(0)).toBeTruthy();
        expect(isValidPosition(12)).toBeFalsy(); // free space
        expect(isValidPosition(24)).toBeTruthy();
        expect(isValidPosition(25)).toBeFalsy();
        expect(isValidPosition(-1)).toBeFalsy();
      });
    });

    describe('calculateProgress', () => {
      test('calculates progress correctly', () => {
        expect(calculateProgress(5, 10)).toBe(50);
        expect(calculateProgress(3, 7)).toBe(43);
        expect(calculateProgress(0, 0)).toBe(0);
      });
    });

    describe('checkBingo', () => {
      test('detects row bingo', () => {
        const grid = Array(25).fill(false);
        grid[0] = grid[1] = grid[2] = grid[3] = grid[4] = true;
        const result = checkBingo(grid);
        expect(result).toHaveLength(1);
        expect(result[0].type).toBe('row');
      });

      test('detects column bingo', () => {
        const grid = Array(25).fill(false);
        grid[0] = grid[5] = grid[10] = grid[15] = grid[20] = true;
        const result = checkBingo(grid);
        expect(result).toHaveLength(1);
        expect(result[0].type).toBe('col');
      });

      test('detects diagonal bingo', () => {
        const grid = Array(25).fill(false);
        grid[0] = grid[6] = grid[12] = grid[18] = grid[24] = true;
        const result = checkBingo(grid);
        expect(result).toHaveLength(1);
        expect(result[0].type).toBe('diagonal');
      });
    });

    describe('API Object Structure', () => {
      test('API exists', () => {
        expect(typeof API).toBe('object');
      });

      test('API has auth property', () => {
        expect(typeof API.auth).toBe('object');
        expect(typeof API.auth.login).toBe('function');
      });

      test('API has cards property', () => {
        expect(typeof API.cards).toBe('object');
        expect(typeof API.cards.create).toBe('function');
      });

      test('API has suggestions property', () => {
        expect(typeof API.suggestions).toBe('object');
        expect(typeof API.suggestions.get).toBe('function');
      });

      test('API has friends property', () => {
        expect(typeof API.friends).toBe('object');
        expect(typeof API.friends.search).toBe('function');
      });

      test('API has reactions property', () => {
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
    window.addEventListener('load', runTests);
  }

  window.runTests = runTests;
})();
