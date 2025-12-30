const { test, expect } = require('@playwright/test');

function getHeader(headers, key) {
  const lower = key.toLowerCase();
  return headers[lower] || headers[key] || '';
}

function parseCspDirectives(csp) {
  const directives = {};
  for (const part of String(csp || '').split(';')) {
    const trimmed = part.trim();
    if (!trimmed) continue;
    const [name, ...tokens] = trimmed.split(/\s+/);
    directives[name] = tokens;
  }
  return directives;
}

test('document response includes CSP without unsafe-inline scripts', async ({ page }, testInfo) => {
  // Hash-only navigation can be treated as a same-document navigation in some browsers,
  // which may return `null` from `page.goto()`. Fetch the actual document instead.
  const response = await page.goto('/');
  expect(response).not.toBeNull();

  const headers = response.headers();
  const csp = getHeader(headers, 'content-security-policy');
  expect(csp).toContain("default-src 'self'");
  expect(csp).toContain("script-src 'self'");

  const directives = parseCspDirectives(csp);
  expect(directives['script-src'] || []).not.toContain("'unsafe-inline'");
  expect(directives['script-src'] || []).not.toContain("'unsafe-hashes'");
});
