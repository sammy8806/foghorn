# PR screenshot capture

Headless Playwright captures for Foghorn PR descriptions.

## Setup

```bash
cd frontend && npm install
cd ../scripts/pr-screenshots
npm install
npx playwright install chromium
```

## Capture

From the repo root, on the feature branch you want to screenshot:

```bash
node scripts/pr-screenshots/capture.mjs <branch-key>
```

Branch keys: `wire-show-resolved-silenced`, `theme-support`, `source-health-banner`, `empty-state-filters`, `persist-ui-prefs`, `search-syntax-help`, `group-silence-menu`, `alert-actions-ui`, `accessibility-improvements`.

Output defaults to `/opt/cursor/artifacts/pr-screenshots/`. Override with `SCREENSHOT_OUT=/path/to/dir`.

## PR bodies

Reference captured PNGs in PR descriptions:

```html
<img alt="Description" src="/opt/cursor/artifacts/pr-screenshots/09-health-failure-banner.png" />
```

The PR tooling uploads artifact paths to stable public URLs automatically.
