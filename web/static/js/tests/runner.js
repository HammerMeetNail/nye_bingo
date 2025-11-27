#!/usr/bin/env node
/**
 * Year of Bingo - JavaScript Test Runner
 *
 * Zero dependencies - uses only Node.js built-ins.
 * Run with: node web/static/js/tests/runner.js
 */

const fs = require('fs');
const path = require('path');

// Test state
let testCount = 0;
let passCount = 0;
let failCount = 0;
let currentSuite = '';

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
  fn();
}

function test(name, fn) {
  testCount++;
  try {
    fn();
    passCount++;
    console.log(`  ${colors.green}✓${colors.reset} ${colors.dim}${name}${colors.reset}`);
  } catch (error) {
    failCount++;
    console.log(`  ${colors.red}✗ ${name}${colors.reset}`);
    console.log(`    ${colors.red}${error.message}${colors.reset}`);
  }
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

describe('parseHash', () => {
  test('parses simple hash', () => {
    const result = parseHash('#dashboard');
    expect(result.page).toBe('dashboard');
    expect(result.params).toEqual([]);
  });

  test('parses hash with params', () => {
    const result = parseHash('#card/abc-123');
    expect(result.page).toBe('card');
    expect(result.params).toEqual(['abc-123']);
  });

  test('parses hash with multiple params', () => {
    const result = parseHash('#friend-card/123/2024');
    expect(result.page).toBe('friend-card');
    expect(result.params).toEqual(['123', '2024']);
  });

  test('handles empty hash', () => {
    const result = parseHash('');
    expect(result.page).toBe('home');
  });

  test('handles just #', () => {
    const result = parseHash('#');
    expect(result.page).toBe('home');
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
