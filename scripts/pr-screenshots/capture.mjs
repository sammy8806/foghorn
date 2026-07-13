#!/usr/bin/env node
import { spawn } from 'node:child_process';
import { access, mkdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';
import { buildBridgeInit } from './mock-bridge.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '../..');
const FRONTEND = path.join(ROOT, 'frontend');
const OUT = process.env.SCREENSHOT_OUT || '/opt/cursor/artifacts/pr-screenshots';

const VIEWPORT = { width: 520, height: 760 };

const PREVIEW_URL = 'http://127.0.0.1:4173/';

async function run(cmd, args, cwd = ROOT) {
  await new Promise((resolve, reject) => {
    const child = spawn(cmd, args, { cwd, stdio: 'inherit', shell: false });
    let settled = false;
    const finish = (fn, value) => {
      if (settled) return;
      settled = true;
      fn(value);
    };
    child.once('error', err => finish(reject, err));
    child.once('exit', code =>
      finish(code === 0 ? resolve : reject, code === 0 ? undefined : new Error(`${cmd} exited ${code}`)),
    );
  });
}

async function waitForPreview(url, timeoutMs = 30000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(url);
      if (res.ok) return;
    } catch {
      // server not ready yet
    }
    await new Promise(r => setTimeout(r, 250));
  }
  throw new Error(`Preview server not ready at ${url} within ${timeoutMs}ms`);
}

async function startPreview() {
  return new Promise((resolve, reject) => {
    const proc = spawn('npm', ['run', 'preview', '--', '--host', '127.0.0.1', '--port', '4173'], {
      cwd: FRONTEND,
      stdio: 'inherit',
    });
    let settled = false;
    const finish = (fn, value) => {
      if (settled) return;
      settled = true;
      fn(value);
    };

    proc.once('error', err => finish(reject, err));
    proc.once('spawn', async () => {
      try {
        await waitForPreview(PREVIEW_URL);
        finish(resolve, () => {
          try { proc.kill(); } catch { /* already stopped */ }
        });
      } catch (err) {
        try { proc.kill(); } catch { /* ignore */ }
        finish(reject, err);
      }
    });
  });
}

async function waitForAppReady(page) {
  await page.waitForFunction(() => {
    const text = document.body.innerText;
    return !text.includes('Loading alerts…') && !text.includes('Loading alerts...');
  }, null, { timeout: 15000 });
  await page.waitForSelector('.group-header, .alert-card, .empty-state', { timeout: 15000 });
}

async function openSearch(page) {
  const search = page.locator('[aria-label="Filter alerts"]');
  await search.click();
  await page.waitForSelector('.search.open input.search-input, .search-input', { state: 'visible', timeout: 5000 });
}

async function capture(browser, name, scenario, action) {
  const context = await browser.newContext({ viewport: VIEWPORT });
  const page = await context.newPage();
  try {
    await page.addInitScript(buildBridgeInit(scenario));
    if (scenario.localStorage) {
      await page.addInitScript(storage => {
        for (const [k, v] of Object.entries(storage)) localStorage.setItem(k, v);
      }, scenario.localStorage);
    }
    await page.goto(PREVIEW_URL, { waitUntil: 'networkidle' });
    await waitForAppReady(page);
    if (action) await action(page);
    await page.waitForTimeout(400);
    const file = path.join(OUT, `${name}.png`);
    await page.screenshot({ path: file, fullPage: false });
    console.log('saved', file);
    return file;
  } finally {
    await context.close();
  }
}

const shots = {
  'wire-show-resolved-silenced': [
    {
      file: '07-silenced-alerts-visible',
      scenario: { showSilenced: true },
    },
    {
      file: '07-silenced-toggle-active',
      scenario: { showSilenced: true },
      action: async (page) => {
        const btn = page.locator('button[aria-label="Hide silenced alerts"], button[title="Hide silenced alerts"]').first();
        if (await btn.count()) await btn.click();
        await page.waitForTimeout(200);
      },
    },
  ],
  'theme-support': [
    { file: '08-theme-dark', scenario: { theme: 'dark' } },
    {
      file: '08-theme-light',
      scenario: { theme: 'light' },
      action: async (page) => page.evaluate(() => { document.documentElement.dataset.theme = 'light'; }),
    },
  ],
  'source-health-banner': [
    { file: '09-health-failure-banner', scenario: { health: 'failing' } },
  ],
  'empty-state-filters': [
    {
      file: '10-empty-state-clear-filters',
      scenario: {},
      action: async (page) => {
        await openSearch(page);
        await page.locator('.search-input').fill('does-not-match-anything');
        await page.waitForTimeout(300);
      },
    },
    {
      file: '10-hidden-count-chip',
      scenario: {},
      action: async (page) => {
        await page.locator('.segment').filter({ hasText: 'Severity' }).click();
        await page.locator('.filter-menu-option').filter({ hasText: 'Critical' }).click();
        await page.waitForTimeout(300);
      },
    },
  ],
  'persist-ui-prefs': [
    {
      file: '11-persisted-filters',
      scenario: {},
      action: async (page) => {
        await openSearch(page);
        await page.locator('.search-input').fill('namespace=payments');
        await page.locator('.segment').filter({ hasText: 'Group' }).click();
        await page.locator('.filter-menu-option').filter({ hasText: 'Cluster' }).click();
        await page.waitForTimeout(300);
      },
    },
  ],
  'search-syntax-help': [
    {
      file: '12-search-syntax-help',
      scenario: {},
      action: async (page) => {
        await page.locator('button[aria-label="Search syntax help"]').click();
        await page.waitForTimeout(200);
      },
    },
  ],
  'group-silence-menu': [
    {
      file: '13-group-silence-menu',
      scenario: {},
      action: async (page) => {
        await page.locator('button.group-menu-btn, button[aria-label="Group actions"]').first().click();
        await page.waitForTimeout(200);
      },
    },
  ],
  'alert-actions-ui': [
    {
      file: '14-alert-actions',
      scenario: {
        actions: [{ Name: 'Open runbook', Match: {}, Action: { Type: 'url', Template: 'https://wiki.example/runbooks/disk' }, Icon: '' }],
        actionsForAll: false,
      },
      action: async (page) => {
        await page.locator('.group-header').first().click();
        await page.waitForTimeout(200);
        await page.locator('.alert-header').filter({ hasText: 'DiskSpaceLow' }).click();
        await page.waitForTimeout(500);
      },
    },
  ],
  'accessibility-improvements': [
    {
      file: '15-verbose-toggle-focused',
      scenario: {},
      action: async (page) => {
        await page.locator('button[aria-label="Toggle verbose display"]').focus();
        await page.waitForTimeout(200);
      },
    },
  ],
};

async function ensureFrontendDeps() {
  try {
    await access(path.join(FRONTEND, 'node_modules'));
  } catch {
    console.error('Missing frontend dependencies. Run: cd frontend && npm install');
    process.exit(1);
  }
}

async function main() {
  const branchKey = process.argv[2];
  if (!branchKey || !shots[branchKey]) {
    console.error('Usage: node capture.mjs <branch-key>');
    console.error('Keys:', Object.keys(shots).join(', '));
    process.exit(1);
  }

  await ensureFrontendDeps();
  await mkdir(OUT, { recursive: true });
  await run('npm', ['run', 'build'], FRONTEND);

  let stopPreview;
  let browser;
  let cleaning = false;

  const cleanup = async () => {
    if (cleaning) return;
    cleaning = true;
    try {
      if (browser) await browser.close();
    } catch { /* ignore */ }
    try {
      if (stopPreview) stopPreview();
    } catch { /* ignore */ }
  };

  const onSignal = signal => {
    cleanup().finally(() => process.exit(signal === 'SIGINT' ? 130 : 143));
  };
  process.once('SIGINT', () => onSignal('SIGINT'));
  process.once('SIGTERM', () => onSignal('SIGTERM'));

  try {
    stopPreview = await startPreview();
    browser = await chromium.launch({ headless: true });
    for (const shot of shots[branchKey]) {
      await capture(browser, shot.file, shot.scenario, shot.action);
    }
  } finally {
    await cleanup();
  }
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});
