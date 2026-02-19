# Phase 16: Playwright E2E Tests - Research

**Researched:** 2026-02-19
**Domain:** Browser-based end-to-end testing with Playwright against a live Kterodactyl kind cluster
**Confidence:** HIGH

## Summary

Phase 16 adds Playwright browser tests in a top-level `e2e/` directory that exercise core user journeys against the kind-deployed Kterodactyl environment created by Phase 15 (`make test-e2e-setup`). The tests cover authentication (sign up, log in), game server CRUD (create, verify, delete), and must use auth fixtures to provide pre-authenticated browser contexts for both admin and regular user roles.

The most critical technical challenge is that Kterodactyl's auth store uses **pure in-memory Zustand state** -- not localStorage, cookies, or sessionStorage. The standard Playwright `storageState` approach (which serializes cookies and localStorage) will not preserve authentication across browser contexts. The recommended solution is a hybrid approach: (1) use a setup project that logs in via the UI and captures `storageState` (which includes any cookies the server might set), combined with (2) a custom fixture that uses `page.addInitScript()` to inject the JWT token into the Zustand store on every page load, ensuring auth survives navigation and context recreation. Alternatively, since the setup project's browser context persists the Zustand state during its own execution, the simplest approach is to perform login via API request in the setup, write the JWT token to a file, then have each test read the token and inject it via `addInitScript()` calling the Zustand store's `setToken()` method.

Registration requires an invite token, which means the E2E test setup must first bootstrap an admin user (via direct Kubernetes Secret creation using kubectl), log in as admin to create invite tokens, and then use those tokens for user registration tests.

**Primary recommendation:** Use `@playwright/test` v1.58.x with Chromium-only config, `workers: 1` in CI, a setup project that seeds an admin user via `kubectl` and authenticates via API, and custom auth fixtures that inject JWT tokens via `addInitScript()` to work around the in-memory Zustand auth store.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PW-01 | Playwright project initialized with config, auth fixtures, and Chromium-only setup in `e2e/` | Use `npm init playwright@latest` in `e2e/`, configure `playwright.config.ts` with Chromium-only project, setup project for auth, `baseURL: 'http://localhost:8080'`, `workers: 1` for CI. Auth fixtures use custom `test.extend()` pattern. |
| PW-02 | Auth fixture creates storageState for admin and regular user roles | Two-phase approach: (1) globalSetup or setup project seeds admin user via `kubectl create secret` + API login, creates invite for regular user via API, (2) custom fixture injects JWT token via `addInitScript()` to set Zustand store state on each page load. Separate `.auth/admin.json` and `.auth/user.json` token files. |
| PW-03 | User can sign up, log in, and see the dashboard | Registration test: use admin API to create invite token, navigate to `/register`, fill form fields (username, email, password, inviteToken), submit, verify redirect to `/` and "Dashboard" heading visible. Login test: navigate to `/login`, fill username/password, submit, verify "Welcome back" text. |
| PW-04 | User can create a game server and see it in the server list | Navigate to `/servers/create`, select game (minecraft), fill server name, submit form, verify toast "created successfully" or redirect to server detail. Navigate to `/servers`, verify server name appears in the grid. |
| PW-05 | User can delete a game server | On `/servers` page, locate the server card, click the trash icon button, verify server disappears from the list (or verify via API that server no longer exists). |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| @playwright/test | ^1.58.2 | Browser test runner with fixtures, assertions, and parallel execution | Official Playwright test framework. Includes `expect`, `test.extend()`, `storageState`, setup projects. Used by 90%+ of Playwright users. |
| playwright (chromium) | (bundled) | Chromium browser binary for headless testing | Bundled with @playwright/test. Chromium-only per prior decision. Use `npx playwright install chromium` to install only Chromium. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| @playwright/test (request API) | (included) | HTTP client for API-based auth setup and data seeding | Use in setup project to call `/api/v1/auth/login`, `/api/v1/admin/invites` for seeding test data without browser overhead. |
| dotenv | ^16.x | Load environment variables from `.env` file | Optional. If test credentials (admin password, base URL) need to be configurable. Could also use `process.env` directly. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Playwright setup project | globalSetup function | Setup projects integrate better with HTML reports, traces, and fixtures. globalSetup is simpler but loses these benefits. Use setup project. |
| addInitScript for token injection | page.evaluate before each navigation | addInitScript runs automatically on every navigation/reload. page.evaluate requires manual calls. addInitScript is more robust. |
| Chromium only | Multi-browser (Firefox, WebKit) | Prior decision: Chromium-only. Keeps CI fast and simple. Can add browsers later. |
| API-based auth seeding | UI-based auth seeding in setup | API seeding is faster and more reliable. UI seeding should only be used for the actual auth UI tests. |
| kubectl for admin bootstrap | Bootstrap API endpoint | The `/api/v1/auth/bootstrap` endpoint is referenced in docs but does NOT exist in code. Must seed admin via kubectl Secret creation. |

**Installation:**
```bash
cd e2e
npm init -y
npm install -D @playwright/test
npx playwright install chromium
```

## Architecture Patterns

### Recommended Project Structure

```
e2e/
  package.json             # Separate from web/package.json
  playwright.config.ts     # Chromium-only, baseURL, auth setup
  .gitignore               # playwright/.auth/, test-results/, playwright-report/
  fixtures/
    auth.ts                # Custom test.extend() with admin/user auth fixtures
  tests/
    auth.setup.ts          # Setup project: seeds admin, creates tokens
    auth.spec.ts           # PW-03: sign up, log in, dashboard
    servers.spec.ts        # PW-04 + PW-05: create, list, delete server
  playwright/
    .auth/                 # Generated: admin.json, user.json (gitignored)
```

### Pattern 1: Admin User Seeding via kubectl

**What:** Before any browser tests run, create an admin user as a Kubernetes Secret directly, bypassing the API registration flow (which requires an invite token from an admin that doesn't exist yet).

**When to use:** In the auth setup project, as the very first step.

**Why needed:** The application has no bootstrap endpoint (docs reference `/api/v1/auth/bootstrap` but it's not implemented). Registration requires an invite token. There are no users to create invites. This is a chicken-and-egg problem that must be solved by seeding the admin user at the Kubernetes level.

**Critical details:**
- Username "admin" is **reserved** (`internal/auth/auth.go` line 80). Use a different name like "e2e-admin".
- Password must be hashed with Argon2id in PHC format (`internal/auth/password.go`).
- User Secret format: `user-<username>` with labels `kterodactyl.io/resource-type: user`, `kterodactyl.io/user: <username>`, `kterodactyl.io/role: admin`.
- Secret data keys: `email`, `password-hash`, `created-at`, `invited-by`.

**Example:**
```typescript
// In auth.setup.ts or a helper script
// Option A: Pre-compute the Argon2id hash and create Secret via kubectl
// Option B: Use a small Go helper to hash and create the Secret
// Option C: Create the Secret with a known hash computed at build time

// The simplest approach: include a pre-computed hash as a constant
// and create the Secret via kubectl in the setup
import { execSync } from 'child_process';

const ADMIN_USERNAME = 'e2e-admin';
const ADMIN_PASSWORD = 'testpassword123';
const ADMIN_EMAIL = 'admin@e2e-test.local';
const NAMESPACE = 'kterodactyl-system';

// Pre-computed Argon2id hash for "testpassword123"
// (generated once, hardcoded for deterministic E2E setup)
const ADMIN_PASSWORD_HASH = '<pre-computed-argon2id-hash>';

execSync(`kubectl create secret generic user-${ADMIN_USERNAME} \
  --namespace=${NAMESPACE} \
  --from-literal=email=${ADMIN_EMAIL} \
  --from-literal=password-hash='${ADMIN_PASSWORD_HASH}' \
  --from-literal=created-at='2026-01-01T00:00:00Z' \
  --from-literal=invited-by='bootstrap' \
  --dry-run=client -o yaml | kubectl apply -f -`);

// Add required labels
execSync(`kubectl label secret user-${ADMIN_USERNAME} \
  --namespace=${NAMESPACE} \
  --overwrite \
  kterodactyl.io/resource-type=user \
  kterodactyl.io/user=${ADMIN_USERNAME} \
  kterodactyl.io/role=admin \
  app.kubernetes.io/managed-by=kterodactyl`);
```

**Confidence: HIGH** -- Based on direct codebase analysis of `internal/auth/store.go` (Secret format) and `internal/auth/auth.go` (reserved usernames).

### Pattern 2: Auth Fixture with JWT Token Injection

**What:** A custom Playwright fixture that extends `test` with pre-authenticated `adminPage` and `userPage` fixtures. These fixtures log in via the API, get a JWT token, and inject it into the browser context using `addInitScript()` so the Zustand store is populated on every page load.

**When to use:** Every test that needs authentication (which is nearly all tests, since all routes except `/login` and `/register` are behind `ProtectedRoute`).

**Why addInitScript:** The Kterodactyl frontend uses Zustand with **no persist middleware** (`web/src/stores/auth-store.ts`). The auth state lives only in JavaScript memory. Standard `storageState` (cookies + localStorage) will not restore it. `addInitScript()` runs a script before any page JavaScript, allowing us to call `window.__E2E_TOKEN = '<jwt>'` and then have the app check for it.

**Challenge:** The Zustand store is created by the app's JavaScript, which runs after `addInitScript()`. We cannot call `useAuthStore.getState().setToken()` in addInitScript because the store doesn't exist yet.

**Solution approaches (pick one):**

**Approach A (Recommended): Modify app to check for E2E token on init.**
Add a small check in `main.tsx` or the auth store that reads from `window.__E2E_TOKEN` or `localStorage.getItem('e2e-token')` on startup. This is a minimal production code change that makes testing possible.

```typescript
// In auth-store.ts or main.tsx initialization:
if (import.meta.env.DEV || window.__E2E_TOKEN) {
  const token = window.__E2E_TOKEN || localStorage.getItem('e2e-token');
  if (token) useAuthStore.getState().setToken(token);
}
```

Then in the fixture:
```typescript
await context.addInitScript((token: string) => {
  window.__E2E_TOKEN = token;
}, jwtToken);
```

**Approach B: Login via UI in each test's beforeEach.**
Each test navigates to `/login`, fills credentials, submits. Slow but requires no production code changes.

**Approach C: Use storageState with localStorage injection.**
In the setup, after API login, use `page.evaluate` to store the token in localStorage, then save storageState. Modify the app to check localStorage on startup as a secondary token source.

**Confidence: MEDIUM** -- Approach A is cleanest but requires a small production code change. Approach B is safest but slowest. The planner should decide.

### Pattern 3: Setup Project Configuration

**What:** A Playwright config with a `setup` project that runs before the `chromium` test project, handling admin seeding and auth token preparation.

**Example:**
```typescript
// playwright.config.ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false, // Sequential for server state dependencies
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : 1, // Always 1 for kind (shared state)
  reporter: process.env.CI ? 'github' : 'html',
  timeout: 60_000, // 60s per test (server creation can be slow)

  use: {
    baseURL: 'http://localhost:8080',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'setup',
      testMatch: /.*\.setup\.ts/,
    },
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        storageState: 'playwright/.auth/user.json',
      },
      dependencies: ['setup'],
    },
  ],
});
```

**Confidence: HIGH** -- Standard Playwright setup project pattern, verified against official docs.

### Pattern 4: API-Based Token Acquisition

**What:** Use Playwright's built-in `request` API context to call the login endpoint and get a JWT token, rather than performing UI login.

**Example:**
```typescript
// In auth.setup.ts
import { test as setup, expect } from '@playwright/test';
import path from 'path';

const adminAuthFile = path.join(__dirname, '../playwright/.auth/admin.json');
const userAuthFile = path.join(__dirname, '../playwright/.auth/user.json');

setup('authenticate as admin', async ({ request }) => {
  // Login via API
  const response = await request.post('/api/v1/auth/login', {
    data: {
      username: 'e2e-admin',
      password: 'testpassword123',
    },
  });
  expect(response.ok()).toBeTruthy();
  const { token } = await response.json();

  // Write token to file for fixtures to read
  const fs = await import('fs');
  fs.mkdirSync(path.dirname(adminAuthFile), { recursive: true });
  fs.writeFileSync(adminAuthFile, JSON.stringify({ token }));
});

setup('create regular user and authenticate', async ({ request }) => {
  // Read admin token
  const fs = await import('fs');
  const adminAuth = JSON.parse(fs.readFileSync(adminAuthFile, 'utf-8'));

  // Create invite via admin API
  const inviteResponse = await request.post('/api/v1/admin/invites', {
    headers: { Authorization: `Bearer ${adminAuth.token}` },
    data: { email: 'e2e-user@test.local' },
  });
  expect(inviteResponse.ok()).toBeTruthy();
  const { token: inviteToken } = await inviteResponse.json();

  // Register user
  const registerResponse = await request.post('/api/v1/auth/register', {
    data: {
      username: 'e2e-user',
      email: 'e2e-user@test.local',
      password: 'testpassword123',
      inviteToken,
    },
  });
  expect(registerResponse.ok()).toBeTruthy();
  const { token } = await registerResponse.json();

  fs.writeFileSync(userAuthFile, JSON.stringify({ token }));
});
```

**Confidence: HIGH** -- Uses Playwright's native request API. Verified against API routes in `internal/api/routes.go`.

### Anti-Patterns to Avoid

- **Using storageState alone for auth:** Zustand's in-memory state is not captured by storageState. Tokens stored in storageState's localStorage section won't be read by the app unless the app is modified to check localStorage.
- **Hardcoding admin user with username "admin":** The username "admin" is reserved in `internal/auth/auth.go:80`. Use "e2e-admin" or similar.
- **Running tests in parallel against a shared kind cluster:** Game server creation modifies shared cluster state. Use `workers: 1` to avoid race conditions.
- **Skipping the admin seeding step:** Without a bootstrap API endpoint, there is no way to create the first admin user through the API alone. kubectl seeding is mandatory.
- **Using `page.goto` for every API call in setup:** Use `request` context instead -- it's faster and doesn't require a browser.
- **Expecting server creation to complete instantly:** Game server creation involves creating K8s resources. The server may be in "Creating" state initially. Tests should wait for the expected state or use `expect` with polling.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Browser automation | Custom Puppeteer/Selenium wrapper | @playwright/test | Playwright has built-in assertions, fixtures, setup projects, trace viewer, and CI integration |
| Auth state management | Custom token injection framework | Playwright setup project + addInitScript | Setup projects are first-class Playwright feature with dependency ordering |
| Test data seeding | Custom K8s client in TypeScript | kubectl exec via child_process | kubectl is already available in the kind environment. Simpler than importing @kubernetes/client-node |
| Waiting for async UI updates | Custom polling loops | Playwright auto-waiting + `expect().toBeVisible()` | Playwright auto-waits for elements. Use `expect` with timeout for async state changes |
| Test isolation | Custom cleanup between tests | Unique server names per test + afterEach cleanup | Unique names prevent collisions. afterEach deletes created resources via API |
| Password hashing for admin seed | Custom Argon2id implementation in JS | Pre-computed hash constant OR small Go helper | Argon2id has specific parameters (time=1, memory=64MB, threads=4). Easier to pre-compute once than re-implement in JS |

**Key insight:** The biggest complexity is not Playwright itself but the bootstrapping problem: seeding the first admin user without a bootstrap API endpoint, and working around in-memory Zustand auth. Both are solvable with straightforward approaches (kubectl + addInitScript).

## Common Pitfalls

### Pitfall 1: Zustand In-Memory Auth Not Persisted

**What goes wrong:** Tests start with a fresh browser context, navigate to a page, and immediately get redirected to `/login` because the Zustand auth store has no token.
**Why it happens:** The auth store at `web/src/stores/auth-store.ts` uses `create()` without Zustand's `persist` middleware. State lives only in JavaScript memory and is lost on page refresh or new context.
**How to avoid:** Use one of the token injection approaches (addInitScript with app-side check, or UI login per test). Do NOT assume storageState will restore auth.
**Warning signs:** All tests redirect to `/login` despite setup project running successfully.

### Pitfall 2: Reserved Username "admin"

**What goes wrong:** Admin user seed fails silently or the Secret is created but login fails with "invalid credentials."
**Why it happens:** `ValidateUsername()` in `internal/auth/auth.go` rejects "admin", "system", "operator", "default", and "kube" as reserved names. The UserStore's `CreateUser()` calls this validation, but direct kubectl Secret creation bypasses it. However, if the app's validation is applied elsewhere (e.g., during login resolution), it could cause issues.
**How to avoid:** Use "e2e-admin" as the username. Verify the Secret is created with correct labels so the UserStore can find it via label selectors.
**Warning signs:** Login returns 401 "invalid credentials" even though the Secret exists. Check labels match: `kterodactyl.io/resource-type=user`, `kterodactyl.io/user=e2e-admin`, `kterodactyl.io/role=admin`, `app.kubernetes.io/managed-by=kterodactyl`.

### Pitfall 3: Rate Limiting on Auth Endpoints

**What goes wrong:** Login or registration requests fail with 429 Too Many Requests during test execution.
**Why it happens:** The API has rate limits: login is 5 req/min per IP, registration is 3 req/min per IP (`internal/api/routes.go:62-63`).
**How to avoid:** (1) Minimize login calls -- authenticate once in setup, reuse tokens. (2) If rate limiting is hit, the CI values (`hack/ci-values.yaml`) may need to disable or relax rate limits for the test environment. (3) Add a wait between retries.
**Warning signs:** Tests fail intermittently with HTTP 429 responses.

### Pitfall 4: Game Server Creation Requires Games to Exist

**What goes wrong:** Server creation test fails because no games are available in the dropdown.
**Why it happens:** The game list comes from `/api/v1/games`, which reads from the manifest loader. The manifests are baked into the Docker image from the `games/` directory (`Dockerfile` line 39: `COPY --from=builder /workspace/games /games`). If the image build didn't include games, the list is empty.
**How to avoid:** The `make test-e2e-setup` flow builds the Docker image which includes `games/minecraft/manifest.yaml`. Verify that the game list is populated before running server creation tests.
**Warning signs:** The "Create Server" page shows "No games available. Contact your administrator."

### Pitfall 5: Slow Server State Transitions in Kind

**What goes wrong:** Test creates a server and immediately checks the server list, but the server shows "Creating" instead of appearing fully.
**Why it happens:** In kind, the operator needs time to create the Pod, and the Pod may not start quickly (especially the minecraft image which is large). The server status goes through Creating -> Starting -> Ready.
**How to avoid:** For create/delete verification, don't wait for "Ready" state -- just verify the server appears/disappears from the list. The E2E test should assert the server *exists* in the list (any state), not that it's "Ready". For deletion, assert it's *removed* from the list.
**Warning signs:** Tests timeout waiting for server to reach "Ready" state.

### Pitfall 6: Admin User Not Found by API After kubectl Seeding

**What goes wrong:** The admin user Secret is created via kubectl but the API returns 401 for login attempts.
**Why it happens:** The API's `handleLogin` calls `s.userStore.GetUser()` which looks up `user-<username>` Secret. If the Secret name, namespace, or required labels don't match exactly, the lookup fails.
**How to avoid:** Ensure the Secret name is `user-e2e-admin`, namespace is `kterodactyl-system`, and ALL four labels are set: `app.kubernetes.io/managed-by=kterodactyl`, `kterodactyl.io/resource-type=user`, `kterodactyl.io/user=e2e-admin`, `kterodactyl.io/role=admin`. The password hash must be valid Argon2id PHC format.
**Warning signs:** `kubectl get secret user-e2e-admin -n kterodactyl-system` shows the Secret exists, but login still returns 401.

## Code Examples

### playwright.config.ts (Chromium-only with setup project)

```typescript
// Source: Playwright official docs + project-specific customization
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1, // Shared kind cluster state
  reporter: process.env.CI ? 'github' : 'html',
  timeout: 60_000,

  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:8080',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'setup',
      testMatch: /.*\.setup\.ts/,
    },
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
      dependencies: ['setup'],
    },
  ],
});
```

### Custom Auth Fixture (token injection via addInitScript)

```typescript
// e2e/fixtures/auth.ts
import { test as base, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';

type AuthFixtures = {
  adminPage: ReturnType<typeof base>['page'];
  userPage: ReturnType<typeof base>['page'];
};

export const test = base.extend<AuthFixtures>({
  adminPage: async ({ browser }, use) => {
    const authFile = path.join(__dirname, '../playwright/.auth/admin.json');
    const { token } = JSON.parse(fs.readFileSync(authFile, 'utf-8'));

    const context = await browser.newContext();
    // Inject token before any page JS runs
    await context.addInitScript((t: string) => {
      (window as any).__KTERODACTYL_E2E_TOKEN = t;
    }, token);

    const page = await context.newPage();
    await use(page);
    await context.close();
  },

  userPage: async ({ browser }, use) => {
    const authFile = path.join(__dirname, '../playwright/.auth/user.json');
    const { token } = JSON.parse(fs.readFileSync(authFile, 'utf-8'));

    const context = await browser.newContext();
    await context.addInitScript((t: string) => {
      (window as any).__KTERODACTYL_E2E_TOKEN = t;
    }, token);

    const page = await context.newPage();
    await use(page);
    await context.close();
  },
});

export { expect } from '@playwright/test';
```

### Auth Test (PW-03: Sign up, log in, dashboard)

```typescript
// e2e/tests/auth.spec.ts
import { test, expect } from '../fixtures/auth';

test.describe('Authentication', () => {
  test('user can log in and see the dashboard', async ({ userPage: page }) => {
    await page.goto('/');
    // With token injection, should land on dashboard
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();
    await expect(page.getByText('Welcome back')).toBeVisible();
  });

  test('new user can sign up with invite token', async ({ adminPage, page }) => {
    // Admin creates invite
    const authFile = path.join(__dirname, '../playwright/.auth/admin.json');
    const { token: adminToken } = JSON.parse(fs.readFileSync(authFile, 'utf-8'));

    const inviteResponse = await page.request.post('/api/v1/admin/invites', {
      headers: { Authorization: `Bearer ${adminToken}` },
      data: { email: 'newuser@e2e-test.local' },
    });
    const { token: inviteToken } = await inviteResponse.json();

    // Navigate to register page (no auth needed)
    await page.goto('/register');
    await page.getByLabel('Username').fill('e2e-newuser');
    await page.getByLabel('Email').fill('newuser@e2e-test.local');
    await page.getByLabel('Password').fill('testpassword123');
    await page.getByLabel('Invite Token').fill(inviteToken);
    await page.getByRole('button', { name: 'Create account' }).click();

    // Should redirect to dashboard after successful registration
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();
  });
});
```

### Server CRUD Test (PW-04 + PW-05)

```typescript
// e2e/tests/servers.spec.ts
import { test, expect } from '../fixtures/auth';

test.describe('Game Servers', () => {
  const serverName = `e2e-test-${Date.now()}`;

  test('user can create a game server', async ({ userPage: page }) => {
    await page.goto('/servers/create');
    // Select minecraft game
    await page.getByText('Minecraft Java Edition').click();
    // Fill server name
    await page.getByLabel('Server Name').fill(serverName);
    // Submit (the form has a submit button inside GameConfigForm)
    await page.getByRole('button', { name: /create/i }).click();

    // Verify redirect to server detail or success toast
    await expect(page).toHaveURL(new RegExp(`/servers/${serverName}`));
  });

  test('server appears in the server list', async ({ userPage: page }) => {
    await page.goto('/servers');
    await expect(page.getByText(serverName)).toBeVisible();
  });

  test('user can delete a game server', async ({ userPage: page }) => {
    await page.goto('/servers');
    // Find the server card and click delete
    const serverCard = page.locator(`text=${serverName}`).locator('..');
    await serverCard.getByRole('button', { name: /delete/i }).or(
      serverCard.locator('button:has(svg)').last()
    ).click();

    // Verify server is removed
    await expect(page.getByText(serverName)).not.toBeVisible({ timeout: 10_000 });
  });
});
```

### Makefile Target Update

```makefile
.PHONY: test-playwright
test-playwright: ## Run Playwright browser tests against kind cluster.
	cd e2e && npx playwright test
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| globalSetup for auth | Setup projects with dependencies | Playwright v1.31 (2023) | Setup projects integrate with HTML reports, traces, and fixtures. Preferred over globalSetup. |
| Cookie-only storageState | storageState with cookies + localStorage + IndexedDB | Playwright v1.30+ | Broader auth state capture. Still doesn't help with in-memory JS state. |
| Multi-browser default | Single-browser projects | Community practice 2024+ | Most teams test on Chromium-only in CI for speed, add Firefox/WebKit later. |
| `page.waitForSelector()` | `expect().toBeVisible()` with auto-retry | Playwright v1.20+ | Built-in assertions auto-retry. No manual wait loops needed. |
| `page.click()` / `page.fill()` | `getByRole()` / `getByLabel()` / `getByText()` | Playwright v1.27+ | Locator-based API with semantic selectors. More resilient to DOM changes. |

**Deprecated/outdated:**
- `page.$()` and `page.$$()` selectors: Use `page.locator()` instead
- `page.waitForNavigation()`: Use `page.waitForURL()` or `expect(page).toHaveURL()`
- Direct `page.click()` without locators: Use `page.getByRole().click()`

## Codebase-Specific Findings

### Auth Store Has No Persistence

The Zustand store in `web/src/stores/auth-store.ts` explicitly notes "Token is stored in memory only (not localStorage) per security best practices. Token is lost on page refresh; user re-authenticates." This is by design and means standard storageState won't work for auth persistence in tests. Every new browser context or page refresh loses authentication.

### Registration Requires Invite Token

`internal/api/request.go:78-95` -- `RegisterRequest.Validate()` requires `inviteToken` to be non-empty. The `handleRegister` flow calls `inviteService.RedeemInvite()` which validates and deletes the invite Secret. This is a single-use token pattern.

### Server Delete Button Has No Confirmation Dialog

`web/src/components/servers/server-card.tsx:87-98` -- The delete button directly calls `deleteMutation.mutate(server.name)` without a confirmation dialog. This simplifies the E2E delete test (no dialog to accept).

### No Bootstrap API Endpoint

The documentation at `docs-site/docs/getting-started/installation.md` references `/api/v1/auth/bootstrap` but this endpoint does not exist in `internal/api/routes.go`. The README uses `/api/v1/auth/register` without an invite token, which would also fail. Admin seeding must happen at the Kubernetes level.

### Rate Limiting Could Affect Tests

`internal/api/routes.go:62-63` -- Login: 5 req/min/IP, Register: 3 req/min/IP. Server creation: 10 req/min/IP. With `workers: 1` and sequential execution, this should not be a problem for the planned test count, but something to watch.

### Frontend Form Structure

- Login page (`web/src/pages/login.tsx`): Uses `Label htmlFor="username"` and `Label htmlFor="password"`. Playwright's `getByLabel('Username')` and `getByLabel('Password')` will find these.
- Register page (`web/src/pages/register.tsx`): Labels for "Username", "Email", "Password", "Invite Token".
- Create server page (`web/src/pages/create-server.tsx`): Two-step flow. Step 1: select game card. Step 2: fill "Server Name" input + dynamic config form with "Create Server" button (inside `GameConfigForm`).

### Server List Uses React Query Polling

`web/src/hooks/use-servers.ts:16-20` -- The server list refetches every 5 seconds (`refetchInterval: 5000`). After creating a server, it will appear in the list within 5 seconds even without manual refresh. After deleting, it will disappear within 5 seconds.

### Server Card Delete Button Structure

`web/src/components/servers/server-card.tsx:87-98` -- The delete button is a ghost variant with `Trash2` icon, no text label. Locator strategy: find by the destructive color class or by the Trash2 SVG, within the card context.

## Open Questions

1. **How to handle Argon2id password hashing for admin seed?**
   - What we know: The Go code uses Argon2id with specific params (time=1, memory=64MB, threads=4, keyLen=32, saltLen=16). The hash is stored in PHC format.
   - What's unclear: Whether to pre-compute a hash constant, use a Go helper script, or use a Node.js argon2 library.
   - Recommendation: Pre-compute the hash for a known test password and hardcode it as a constant. This is deterministic, fast, and requires no extra dependencies. A Makefile target or shell script can generate it once using a small Go program. Alternatively, use the `argon2` npm package in the setup script.

2. **Should the app be modified to support E2E token injection?**
   - What we know: The Zustand store has no persistence. addInitScript can set window globals. But the Zustand store initializes from its `create()` call, not from window globals.
   - What's unclear: Whether the team is willing to add a small production code change (check `window.__E2E_TOKEN` on store init) to enable clean E2E auth.
   - Recommendation: Yes, add a small conditional in the auth store initialization that checks for a window global or localStorage entry. Guard it behind `import.meta.env.DEV` or `import.meta.env.MODE === 'test'`. This is a common pattern. The alternative (UI login per test) is significantly slower.

3. **Should tests clean up created resources?**
   - What we know: Tests run against a shared kind cluster. Created servers persist between tests.
   - What's unclear: Whether to rely on test ordering (create then delete) or do explicit cleanup in afterEach hooks.
   - Recommendation: Use unique timestamps in resource names to avoid collisions. Tests that create servers should also delete them in the same spec file, in order. Since `workers: 1` and `fullyParallel: false`, ordering is deterministic.

## Sources

### Primary (HIGH confidence)
- [Playwright Authentication Docs](https://playwright.dev/docs/auth) -- Setup project pattern, storageState, multi-role auth, API-based auth
- [Playwright Configuration Docs](https://playwright.dev/docs/test-configuration) -- playwright.config.ts structure, projects, webServer, workers
- [Playwright Fixtures Docs](https://playwright.dev/docs/test-fixtures) -- test.extend(), worker-scoped fixtures, auto fixtures
- [Playwright Installation Docs](https://playwright.dev/docs/intro) -- npm init playwright@latest, project scaffold, Chromium install
- [@playwright/test npm](https://www.npmjs.com/package/@playwright/test) -- Version 1.58.2 (stable)
- Codebase analysis: `web/src/stores/auth-store.ts` (Zustand in-memory), `internal/api/routes.go` (API routes + rate limits), `internal/auth/store.go` (K8s Secret format), `internal/auth/auth.go` (reserved usernames), `internal/auth/password.go` (Argon2id params)

### Secondary (MEDIUM confidence)
- [Playwright storageState - BrowserStack](https://www.browserstack.com/guide/playwright-storage-state) -- storageState covers cookies + localStorage + IndexedDB
- [Playwright Global Setup Docs](https://playwright.dev/docs/test-global-setup-teardown) -- globalSetup vs setup projects, passing data via env vars
- [Playwright GitHub Issue #20182](https://github.com/microsoft/playwright/issues/20182) -- storageState with SPAs using in-memory tokens, workarounds
- [Testing JWT Tokens with Playwright - Medium](https://medium.com/@mahtabnejad/testing-jwt-tokens-with-playwright-2002a8b64341) -- API-based token capture pattern

### Tertiary (LOW confidence)
- [Playwright Feature Request #31108](https://github.com/microsoft/playwright/issues/31108) -- sessionStorage support in storageState (not yet implemented)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- @playwright/test is the clear standard for browser E2E testing. Version pinned to 1.58.x.
- Architecture: HIGH -- Setup project + custom fixtures is the documented Playwright pattern. Project structure follows conventions.
- Auth approach: MEDIUM -- The in-memory Zustand workaround (addInitScript + app modification) is a known pattern but requires a production code change. The planner needs to decide between approaches.
- Admin seeding: HIGH -- kubectl-based Secret creation is the only viable path given no bootstrap API. Secret format verified against codebase.
- Pitfalls: HIGH -- Each pitfall identified from direct codebase analysis (reserved usernames, rate limits, no storageState for Zustand).

**What might I have missed:**
- The `GameConfigForm` component from `@rjsf/shadcn` renders a dynamic form from JSON Schema. The submit button text and form structure may not be straightforward to locate with Playwright selectors. The planner should investigate the RJSF-rendered DOM structure.
- If the kind cluster already has servers or users from a previous test run (if teardown wasn't done), tests may fail. The setup should ensure clean state.
- The `test-playwright` Makefile target currently prints a placeholder message. It needs to be updated to run `cd e2e && npx playwright test`.
- Network policies or CORS could affect Playwright's API requests from the Node.js process (separate from browser). The API has `AllowedOrigins: ["*"]` so CORS should not be an issue.

**Research date:** 2026-02-19
**Valid until:** 2026-03-19 (Playwright releases monthly but core patterns are stable)
