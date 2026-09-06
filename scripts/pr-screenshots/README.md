# PR screenshot capture

Headless Playwright captures for Foghorn PR descriptions.

## README demo

The README screenshot uses the current frontend with synthetic alert data from `mock-bridge.mjs`. Start the development server:

```bash
cd frontend
npm run dev -- --host 127.0.0.1 --port 4173
```

Open `http://127.0.0.1:4173/demo.html` to inspect the demo in a browser. The README image at `docs/screenshots/demo.png` is a native macOS window capture, not a browser screenshot. It includes the real traffic lights, rounded corners, and translucent title bar.

For a native capture, use an isolated copy of the app with the mock bridge installed as a static module before the frontend mounts, reporting `darwin` as its platform. Use an empty source configuration, disable notifications, and open a 720 × 460 window visibly at startup. Keep the normal macOS window options unchanged. Expand `DiskSpaceLow` and capture the app window. Do not evaluate the mock bridge string at runtime in the native build, since the production content security policy blocks dynamic code evaluation.

The demo entry point loads the real app components with a mock Wails bridge. It does not connect to alert sources or use local credentials. It is only served during development and is not an entry point in the production build.

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

Branch keys: `wire-show-resolved-silenced`, `theme-support`, `source-health-banner`, `empty-state-filters`, `persist-ui-prefs`, `search-syntax-help`, `group-silence-menu`, `accessibility-improvements`.

Output defaults to `/opt/cursor/artifacts/pr-screenshots/`. Override with `SCREENSHOT_OUT=/path/to/dir`.

## PR bodies

Reference captured PNGs in PR descriptions:

```html
<img alt="Description" src="/opt/cursor/artifacts/pr-screenshots/09-health-failure-banner.png" />
```

The PR tooling uploads artifact paths to stable public URLs automatically.
