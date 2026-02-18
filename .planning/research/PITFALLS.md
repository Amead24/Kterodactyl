# Domain Pitfalls: E2E CI/CD Test Suite

**Domain:** Adding Playwright E2E tests, Go API integration tests, kind-based test environments, and GitHub Actions CI to a Kubernetes operator project (v1.1 -- first test suite for a project with zero tests)
**Researched:** 2026-02-17
**Confidence:** HIGH (multiple verified sources, project-specific analysis)

---

## Critical Pitfalls

Mistakes that cause rewrites, multi-day debugging sessions, or permanently flaky CI.

---

### Pitfall 1: EnvTest Cached Client Causes Flaky Assertions

**What goes wrong:**
Controller integration tests pass 80% of the time but randomly fail with "expected state X, got state Y" errors. The test creates a resource, immediately reads it back, and gets stale data. Developers add `time.Sleep` everywhere, making tests slow and still intermittently failing.

**Why it happens:**
The kubebuilder-scaffolded `suite_test.go` uses `k8sManager.GetClient()` which returns a cached client backed by informer caches. The informer cache depends on etcd watches for updates, so after creating or deleting objects, the cache takes milliseconds to seconds to sync. Tests asserting against cache contents instead of live API server state get stale reads.

The existing `internal/controller/suite_test.go` in Kterodactyl already uses this pattern -- the `k8sClient` variable is likely the manager's cached client.

**Consequences:**
- Tests pass locally but fail in CI (resource-constrained runners have slower cache sync)
- Developers add arbitrary `time.Sleep` calls that slow the suite without fixing the root cause
- Eventually, team marks tests as "known flaky" and ignores failures, defeating the purpose of CI

**Prevention:**
1. Create a **separate "live" client** for test assertions that reads directly from the API server, not the cache:
   ```go
   // In suite_test.go BeforeSuite
   k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})  // live client
   // NOT: k8sClient = k8sManager.GetClient()  // cached client
   ```
2. Always use `Eventually` / `Consistently` for assertions after mutations -- never bare `Expect` after Create/Update/Delete
3. Use `Eventually` with a function that re-fetches the resource on each poll:
   ```go
   Eventually(func(g Gomega) {
       gs := &v1alpha1.GameServer{}
       g.Expect(k8sClient.Get(ctx, key, gs)).To(Succeed())
       g.Expect(gs.Status.State).To(Equal(v1alpha1.GameServerStateStarting))
   }, timeout, interval).Should(Succeed())
   ```
4. Run `go test -race` and `ginkgo --until-it-fails` to surface race conditions before they hit CI

**Detection:**
- Test passes in isolation but fails in parallel or in CI
- Adding `time.Sleep(500ms)` "fixes" the test
- Different results with `-count=1` vs `-count=10`

**Phase to address:** Phase 1 (Go unit/integration tests) -- fix the test client setup before writing any new tests

**Confidence:** HIGH -- documented in [Kubebuilder Book](https://book.kubebuilder.io/cronjob-tutorial/writing-tests), [InfraCloud EnvTest Guide](https://www.infracloud.io/blogs/testing-kubernetes-operator-envtest/), [SuperOrbital Testing Production Controllers](https://superorbital.io/blog/testing-production-controllers/)

---

### Pitfall 2: EnvTest Cannot Delete Namespaces -- Tests Contaminate Each Other

**What goes wrong:**
Test A creates namespace `test-ns-1` with resources. Test B runs in `test-ns-2`. But the controller is still reconciling resources from `test-ns-1` because namespace deletion in envtest only sets the namespace to `Terminating` state -- it never actually reclaims it. The controller processes leftover resources from previous tests, causing unexpected state and failures.

**Why it happens:**
EnvTest runs a real kube-apiserver and etcd but has no kubelet, no garbage collector, and no namespace controller. When you `kubectl delete ns`, the namespace enters `Terminating` but never completes deletion. Resources inside the "deleted" namespace remain, and the controller's informer cache still sees them. The existing test file `gameserver_controller_test.go` uses separate namespaces per test (`test-ns-1` through `test-ns-7`) but relies on `AfterEach` cleanup which may not fully work.

**Consequences:**
- Tests that pass individually fail when run together
- Controller reconciles resources from a "previous" test, causing mysterious state transitions
- Namespace reuse across test runs fails with "already exists" errors
- CI becomes unreliable without anyone understanding why

**Prevention:**
1. **Generate unique namespace names per test run** using random suffixes:
   ```go
   testNs := fmt.Sprintf("test-%s-%s", t.Name(), rand.String(6))
   ```
2. **Clean up all resources explicitly** in AfterEach/AfterAll -- do not rely on namespace deletion
3. **Delete individual resources** (GameServer CRs, Pods, Services) rather than deleting the namespace
4. **Wait for deletions to complete** with `Eventually` before proceeding:
   ```go
   Eventually(func() bool {
       err := k8sClient.Get(ctx, key, &v1alpha1.GameServer{})
       return errors.IsNotFound(err)
   }, timeout, interval).Should(BeTrue())
   ```
5. Consider scoping the reconciler to a specific namespace during tests to prevent cross-contamination

**Detection:**
- `go test ./internal/controller/ -count=1` passes but `go test ./internal/controller/ -count=3` fails
- Different test ordering produces different results
- Tests reference resources they did not create

**Phase to address:** Phase 1 (Go integration tests) -- establish namespace isolation pattern before writing new tests

**Confidence:** HIGH -- documented in [Kubebuilder EnvTest reference](https://book.kubebuilder.io/reference/envtest): "envtest does not support namespace deletion"

---

### Pitfall 3: Kind Cluster Image Loading Timeout in CI

**What goes wrong:**
The e2e test workflow times out during `kind load docker-image` because the operator image is large (Go binary + embedded React SPA + game manifests). On GitHub Actions runners with limited I/O bandwidth, loading a 200MB+ image into the kind cluster takes 3-5 minutes. Combined with cluster creation, CRD installation, and Helm deploy, the test exceeds the job timeout.

**Why it happens:**
`kind load docker-image` exports the Docker image as a tarball, transfers it into the kind node container, then imports it into containerd. This involves serializing the entire image to disk, then copying it through Docker's API. For the Kterodactyl image (multi-stage build with Node.js frontend + Go backend), the uncompressed layers are substantial. The existing `test-e2e.yml` workflow has no caching, no image size optimization for test builds, and no timeout configuration.

**Consequences:**
- E2E tests take 10-15 minutes before any test code even runs
- Job exceeds GitHub Actions 6-hour limit on complex test matrices
- Developers stop running e2e tests because "they take too long"
- Flaky timeouts on CI but not locally (faster I/O)

**Prevention:**
1. **Build a test-specific image** that skips unnecessary layers (no frontend for API-only e2e tests):
   ```dockerfile
   # Dockerfile.test -- smaller image for CI
   FROM golang:1.25 AS builder
   COPY . .
   RUN CGO_ENABLED=0 go build -o manager cmd/main.go
   FROM gcr.io/distroless/static:nonroot
   COPY --from=builder /workspace/manager .
   COPY --from=builder /workspace/games /games
   ```
2. **Use `kind load image-archive`** with pre-built tarballs instead of `kind load docker-image` -- avoids double-serialization
3. **Use a local registry** instead of image loading:
   ```yaml
   # kind-config.yaml
   containerdConfigPatches:
   - |-
     [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5001"]
       endpoint = ["http://kind-registry:5001"]
   ```
4. **Cache Docker build layers** in GitHub Actions:
   ```yaml
   - uses: docker/build-push-action@v6
     with:
       cache-from: type=gha
       cache-to: type=gha,mode=max
   ```
5. **Set `imagePullPolicy: Never`** in test manifests to prevent kind from trying to pull from remote registries
6. **Add `--wait 5m`** to `kind create cluster` to give the control plane time to stabilize

**Detection:**
- E2E job takes >10 minutes before first test assertion
- "context deadline exceeded" or timeout errors during image load
- Sporadic "image not found" errors despite successful `kind load`

**Phase to address:** Phase 3 (kind-based E2E environment setup) -- design the image loading strategy before writing e2e tests

**Confidence:** HIGH -- documented in [kind issue #3002](https://github.com/kubernetes-sigs/kind/issues/3002), [kind issue #2922](https://github.com/kubernetes-sigs/kind/issues/2922), [iximiuz blog](https://iximiuz.com/en/posts/kubernetes-kind-load-docker-image/)

---

### Pitfall 4: Playwright Waiting on Kubernetes State Changes -- False Timeouts and Flaky Tests

**What goes wrong:**
Playwright E2E test creates a game server via the UI, then asserts the server shows "Ready" status. The test times out because the underlying Kubernetes Pod takes 30-120 seconds to pull an image, start, and become ready. Playwright's default 30-second timeout fires before the k8s state transition completes. Alternatively, the test passes when the cluster is warm (image cached) but fails on fresh CI clusters.

**Why it happens:**
Playwright's auto-waiting only waits for DOM elements to be actionable -- it has no awareness of Kubernetes reconciliation loops, Pod scheduling, or image pulls happening behind the scenes. The Kterodactyl architecture has a multi-hop latency path: UI action -> API call -> CRD mutation -> controller reconciliation -> Pod creation -> Pod scheduling -> container start -> status update -> API poll -> UI update. Each hop adds variable latency, and Playwright has no way to know how long the full chain takes.

**Consequences:**
- Tests fail intermittently in CI (cold image cache) but pass locally (warm cache)
- Developers increase global timeout to 5 minutes, making test failures take forever to surface
- Real bugs are masked by "expected" timeouts
- Test suite becomes untrusted -- nobody believes red CI means real failure

**Prevention:**
1. **Use `page.waitForResponse`** to wait for specific API responses instead of DOM polling:
   ```typescript
   const responsePromise = page.waitForResponse(
     resp => resp.url().includes('/api/v1/gameservers/') && resp.status() === 200
   );
   await page.click('[data-testid="create-server"]');
   await responsePromise;
   ```
2. **Set per-test timeouts** based on the operation type, not a global timeout:
   ```typescript
   test('create game server', async ({ page }) => {
     test.setTimeout(120_000); // 2 min for k8s operations
     // ...
   });
   ```
3. **Use API-level waits, not UI-level waits** for k8s state transitions:
   ```typescript
   // Poll the API directly, not the UI rendering
   await expect(async () => {
     const resp = await page.request.get('/api/v1/gameservers/my-server/');
     const body = await resp.json();
     expect(body.state).toBe('Ready');
   }).toPass({ timeout: 90_000, intervals: [2_000] });
   ```
4. **Pre-pull images in kind** during cluster setup to eliminate image pull latency:
   ```bash
   docker pull itzg/minecraft-server:latest
   kind load docker-image itzg/minecraft-server:latest --name $KIND_CLUSTER
   ```
5. **Use mock/stub game images** in E2E tests (an `nginx:alpine` that becomes "Ready" in 5 seconds) instead of real game server images that take 60+ seconds to start
6. **Add `data-testid` attributes** to all state-dependent UI elements for reliable selectors

**Detection:**
- Test passes on retry but fails on first run
- Same test takes 10 seconds locally but 90 seconds in CI
- `page.waitForSelector` timeout errors in CI logs

**Phase to address:** Phase 4 (Playwright E2E tests) -- design the waiting strategy before writing any tests

**Confidence:** HIGH -- verified with [Playwright docs on auto-waiting](https://playwright.dev/docs/writing-tests), [Semaphore flaky tests guide](https://semaphore.io/blog/flaky-tests-playwright), [BrowserStack flaky tests guide](https://www.browserstack.com/guide/playwright-flaky-tests)

---

### Pitfall 5: GitHub Actions Disk Space Exhaustion During E2E Tests

**What goes wrong:**
The e2e test job fails with "No space left on device" errors. The GitHub Actions runner starts with ~22GB free, but between Docker images (Go builder, Node.js, kind node image, operator image, game server images) and kind's containerd storage, disk usage exceeds available space.

**Why it happens:**
The Kterodactyl build involves multiple large Docker images:
- `golang:1.25` (~1.2GB)
- `node:22-alpine` (~200MB)
- Kind node image (~800MB)
- Built operator image (~200MB compressed, more uncompressed)
- Game server images for testing (e.g., `itzg/minecraft-server` ~500MB)
- Plus: Go module cache, npm cache, CRD manifests, Helm charts

GitHub-hosted runners start with limited disk space, and `kind load docker-image` creates additional copies of images. The existing workflow has no disk cleanup step.

**Consequences:**
- E2E tests fail with cryptic I/O errors, not obvious disk space messages
- Intermittent failures based on how much preinstalled software the runner has
- Debugging takes hours because the error surfaces in unexpected places (etcd, containerd, Docker)

**Prevention:**
1. **Add disk cleanup step** before kind cluster creation:
   ```yaml
   - name: Free disk space
     run: |
       sudo rm -rf /opt/hostedtoolcache
       sudo rm -rf /usr/local/lib/android
       sudo rm -rf /usr/share/dotnet
       docker system prune -af
   ```
2. **Use minimal game server images** for testing -- `nginx:alpine` (8MB) instead of `itzg/minecraft-server` (500MB)
3. **Use multi-stage Dockerfile** that does not retain build tools in the test image
4. **Monitor disk usage** in the workflow:
   ```yaml
   - name: Check disk space
     run: df -h
   ```
5. **Cache Go modules and npm dependencies** to avoid redownloading:
   ```yaml
   - uses: actions/cache@v4
     with:
       path: ~/go/pkg/mod
       key: go-mod-${{ hashFiles('go.sum') }}
   ```

**Detection:**
- Errors like "write /var/lib/containerd/...: no space left on device"
- `docker build` fails with ENOSPC
- etcd crashes during test with "database space exceeded"

**Phase to address:** Phase 5 (GitHub Actions CI) -- design the workflow with disk management from the start

**Confidence:** HIGH -- documented in [GitHub community discussions](https://github.com/orgs/community/discussions/25678), [Gerald on IT cleanup guide](https://www.geraldonit.com/mastering-disk-space-on-github-actions-runners-a-deep-dive-into-cleanup-strategies-for-x64-and-arm64-runners/)

---

## Moderate Pitfalls

Issues that cause days of debugging or significant rework but are recoverable.

---

### Pitfall 6: Playwright Auth State Not Shared Across Tests

**What goes wrong:**
Every single Playwright test logs in through the UI, making the test suite 3-5x slower than necessary. Or worse, tests manually set JWT tokens via `localStorage` but miss HttpOnly cookie behavior, leading to tests that "pass" but don't actually test the real auth flow.

**Why it happens:**
Developers don't use Playwright's `storageState` feature to save and reuse authenticated sessions. Each test opens the login page, types credentials, clicks submit, waits for redirect. With 50+ E2E tests, this adds 10+ minutes of login overhead.

Kterodactyl uses JWT tokens in HTTP-only cookies with session management via the Go API server. The auth flow involves: POST /api/v1/auth/login -> server sets cookie -> subsequent requests include cookie. Playwright needs to capture this cookie state once and reuse it.

**Prevention:**
1. **Use Playwright's global setup** to authenticate once and save state:
   ```typescript
   // global-setup.ts
   async function globalSetup() {
     const browser = await chromium.launch();
     const page = await browser.newPage();
     await page.goto('/login');
     await page.fill('[name=username]', process.env.TEST_USER);
     await page.fill('[name=password]', process.env.TEST_PASS);
     await page.click('[type=submit]');
     await page.waitForURL('/dashboard');
     await page.context().storageState({ path: 'playwright/.auth/user.json' });
     await browser.close();
   }
   ```
2. **Configure projects** to use stored auth state:
   ```typescript
   // playwright.config.ts
   projects: [
     { name: 'setup', testMatch: /.*\.setup\.ts/ },
     { name: 'tests', use: { storageState: 'playwright/.auth/user.json' }, dependencies: ['setup'] },
   ]
   ```
3. **Add `playwright/.auth` to `.gitignore`** to prevent committing credentials
4. **Create separate auth states** for admin and regular user test suites
5. **Seed test users via the API** in global setup rather than through the invite flow

**Detection:**
- Test suite takes 20+ minutes for 30 tests
- Every test has a login preamble
- Tests fail when auth cookies expire mid-suite

**Phase to address:** Phase 4 (Playwright E2E setup) -- implement auth state management as part of test infrastructure

**Confidence:** HIGH -- verified with [Playwright auth docs](https://playwright.dev/docs/auth)

---

### Pitfall 7: Kind Cluster Not Cleaned Up Between CI Runs

**What goes wrong:**
E2E tests pass on the first CI run but fail on subsequent runs with "kind cluster already exists" or "namespace already exists" errors. On self-hosted runners (if used later), previous test state leaks into new runs.

**Why it happens:**
The existing `Makefile` cleanup target (`cleanup-test-e2e`) runs at the end of `test-e2e`, but if the test fails or times out, the cleanup step never executes. The `setup-test-e2e` target checks for existing clusters but doesn't verify the cluster is in a clean state. On GitHub-hosted runners this is less critical (fresh VM each run), but the workflow design should be robust regardless.

**Consequences:**
- Self-hosted runner builds are completely broken after first failure
- CRDs from previous runs conflict with current installation
- Leftover resources cause unexpected controller behavior
- Developers waste time debugging "works on first run" problems

**Prevention:**
1. **Delete the kind cluster at the START of the workflow**, not just the end:
   ```yaml
   - name: Clean up any existing kind cluster
     run: kind delete cluster --name $KIND_CLUSTER 2>/dev/null || true
   ```
2. **Use `always()` condition** for cleanup in GitHub Actions:
   ```yaml
   - name: Cleanup kind cluster
     if: always()
     run: kind delete cluster --name $KIND_CLUSTER
   ```
3. **Use unique cluster names per workflow run** to avoid collisions:
   ```yaml
   env:
     KIND_CLUSTER: kterodactyl-e2e-${{ github.run_id }}
   ```
4. **Implement cleanup as a separate job** with `needs` dependency and `if: always()`:
   ```yaml
   cleanup:
     needs: test-e2e
     if: always()
     runs-on: ubuntu-latest
     steps:
       - run: kind delete cluster --name $KIND_CLUSTER
   ```
5. **Add timeouts to jobs** to prevent runaway tests from consuming runner hours:
   ```yaml
   jobs:
     test-e2e:
       timeout-minutes: 30
   ```

**Detection:**
- Second CI run on same runner fails with cluster-exists errors
- CRD version conflicts in logs
- "resource already exists" errors in setup steps

**Phase to address:** Phase 3 (kind cluster setup) and Phase 5 (GitHub Actions CI) -- bake cleanup into the workflow from day one

**Confidence:** HIGH -- verified from existing `Makefile` and `test-e2e.yml` analysis

---

### Pitfall 8: Playwright WebSocket Console Tests Are Inherently Flaky

**What goes wrong:**
Tests that verify the WebSocket console (xterm.js terminal) fail intermittently. The test opens the console, waits for a WebSocket connection, sends a command, and asserts output appears. But the WebSocket connection timing, the game server's readiness to accept stdin, and xterm.js rendering all introduce variable latency.

**Why it happens:**
The Kterodactyl console works by: UI opens WebSocket to API server -> API server creates exec session to Pod -> Pod's game server process accepts stdin/stdout. Each hop has independent failure modes:
- WebSocket connection may not be established yet when the test sends input
- Pod exec session may timeout or fail if the container is still initializing
- xterm.js rendering is asynchronous -- characters may not be visible immediately
- Game server processes have varying startup times before accepting console input

**Consequences:**
- Console tests fail 10-20% of the time in CI
- Developers disable console tests or mark them as `skip`
- Real console bugs ship because the tests aren't trusted

**Prevention:**
1. **Wait for WebSocket connection before interacting:**
   ```typescript
   const wsPromise = page.waitForEvent('websocket');
   await page.click('[data-testid="open-console"]');
   const ws = await wsPromise;
   await ws.waitForEvent('framereceived'); // wait for initial output
   ```
2. **Use a mock game server** for console E2E tests that immediately echoes stdin to stdout -- do not test against a real Minecraft server
3. **Test the WebSocket API directly** (without xterm.js) for functional validation -- save xterm.js visual tests for a small smoke test
4. **Add retry logic at the assertion level**, not the action level:
   ```typescript
   await expect(page.locator('.xterm-rows')).toContainText('server>', { timeout: 15_000 });
   ```
5. **Test WebSocket messages programmatically** using Playwright's WebSocket API rather than through the terminal UI:
   ```typescript
   page.on('websocket', ws => {
     ws.on('framereceived', event => { /* assert on messages */ });
   });
   ```
6. **Separate console connectivity tests** (WebSocket connects, data flows) from **console content tests** (specific game output)

**Detection:**
- Console tests have highest flake rate in the suite
- Tests pass with `--debug` (slower execution gives more time) but fail headless
- Different results on different browser engines (Chromium vs Firefox WebSocket timing)

**Phase to address:** Phase 4 (Playwright E2E) -- design console test strategy separately from regular UI tests

**Confidence:** MEDIUM -- based on Playwright WebSocket API docs and general WebSocket testing patterns; no specific xterm.js + k8s testing guides found

---

### Pitfall 9: Go API Tests Use Fake Client but E2E Tests Use Real Cluster -- Gap in Coverage

**What goes wrong:**
API handler tests use `fake.NewClientBuilder()` (as in the existing `helpers_test.go`) which does not run the controller. E2E tests run the full stack against kind. There is a significant coverage gap between these two levels: the API handler tests verify HTTP behavior but skip reconciliation, while e2e tests are slow and coarse-grained. Bugs in the API-to-controller interaction (e.g., status subresource updates, CRD validation, label selectors) slip through both test layers.

**Why it happens:**
The fake client does not support:
- Server-side validation (CRD structural schemas)
- Status subresource semantics (separate client.Status().Update needed but fake client may not enforce it)
- Finalizer behavior (fake client doesn't trigger reconciliation on delete with finalizers)
- Watch/informer cache behavior
- Admission webhooks

Developers assume that if API tests pass (with fake client) and e2e tests pass (full stack), everything is covered. But the fake client has different semantics than a real API server.

**Consequences:**
- API tests pass but the same operation fails in production due to CRD validation rejecting the request
- Status updates work in tests but fail in real cluster because of subresource handling differences
- Label selectors that work with fake client return different results with real API server

**Prevention:**
1. **Add an integration test layer** that runs the controller with envtest AND hits the API server:
   ```go
   // integration_test.go -- runs both controller and API server against envtest
   func TestCreateGameServerIntegration(t *testing.T) {
       // Start envtest with controller
       // Start API server against envtest's k8s client
       // Make HTTP request to API
       // Assert reconciliation creates Pod
   }
   ```
2. **Acknowledge the gap explicitly** in test documentation -- list what each test level does and does not cover
3. **Use envtest for API handler tests** instead of fake client for critical paths (creation, deletion, state transitions)
4. **Keep fake client tests** for pure HTTP behavior (400 errors, validation, auth checks) where controller behavior is irrelevant
5. **Add contract tests** that verify the fake client and real API server produce the same results for key operations

**Detection:**
- "Works in tests, fails in kind" pattern
- API tests assert behaviors the fake client doesn't actually enforce
- Status subresource updates silently differ between fake and real client

**Phase to address:** Phase 2 (Go API integration tests) -- decide the test architecture before writing extensive API tests

**Confidence:** HIGH -- verified from existing test code analysis and [Operator SDK testing docs](https://sdk.operatorframework.io/docs/building-operators/golang/testing/)

---

### Pitfall 10: Playwright Tests Hardcode URLs and Selectors That Break on Layout Changes

**What goes wrong:**
E2E tests break every time the UI team refactors a component. Tests use CSS selectors like `.MuiButton-root`, text content like `await page.click('text=Create Server')`, or structural selectors like `div > div:nth-child(3) > button` that are coupled to implementation details.

**Why it happens:**
Without a testing strategy, developers write Playwright tests by inspecting the browser, copying selectors, and pasting into tests. The React SPA (using Radix UI + Tailwind) generates class names that are unstable across builds. Text content changes during i18n or copy updates.

**Consequences:**
- Every UI PR breaks 5-10 E2E tests
- Developers stop running E2E tests before merging UI changes
- Maintaining test selectors becomes a full-time job
- Tests are brittle indicators of UI correctness

**Prevention:**
1. **Add `data-testid` attributes** to all interactive and assertable elements:
   ```tsx
   <Button data-testid="create-server-btn" onClick={handleCreate}>
     Create Server
   </Button>
   ```
2. **Create a test ID convention** in the project:
   - `data-testid="page-{name}"` for page containers
   - `data-testid="{entity}-{action}-btn"` for buttons
   - `data-testid="{entity}-{field}"` for display values
   - `data-testid="{entity}-status"` for state indicators
3. **Use Playwright's `getByRole`** and `getByLabel` for form elements (accessible and stable)
4. **Never use CSS class selectors or nth-child** in E2E tests
5. **Co-locate test IDs with components** and enforce via ESLint rule or code review

**Detection:**
- E2E tests fail after UI-only PRs
- Tests use `.class-name` or `:nth-child` selectors
- Selector strings longer than 50 characters

**Phase to address:** Phase 4 (Playwright E2E) -- establish test ID conventions BEFORE writing any tests; retrofit `data-testid` attributes to existing components

**Confidence:** HIGH -- standard Playwright best practice from [Playwright writing tests docs](https://playwright.dev/docs/writing-tests), [Elaichenkov 17 mistakes guide](https://elaichenkov.github.io/posts/17-playwright-testing-mistakes-you-should-avoid/)

---

## Minor Pitfalls

Issues that waste hours but are straightforward to fix.

---

### Pitfall 11: Playwright CI Runs All Workers, Overloading the Runner

**What goes wrong:**
Playwright defaults to using half the available CPU cores as workers. On a developer machine with 16 cores, this means 8 parallel browser instances. On a GitHub Actions runner with 2 cores, this means 1 worker -- but if configured explicitly for speed, too many workers cause OOM kills and flaky timeouts due to resource contention.

**Prevention:**
- Set `workers: 1` in CI configuration to ensure sequential, stable execution:
  ```typescript
  // playwright.config.ts
  export default defineConfig({
    workers: process.env.CI ? 1 : undefined,
  });
  ```
- Use the `--shard` flag for parallelism across CI jobs instead of within a single runner
- Run only Chromium in CI (skip Firefox and WebKit unless cross-browser is critical)

**Phase to address:** Phase 4 (Playwright config)

**Confidence:** HIGH -- verified with [Playwright CI docs](https://playwright.dev/docs/ci)

---

### Pitfall 12: Go Test Timeout Defaults Are Too Short for Controller Tests

**What goes wrong:**
`go test` defaults to a 10-minute timeout per package. Controller integration tests that start envtest, run the manager, execute tests with `Eventually` waits, and tear down can exceed this. The test binary is killed mid-run with no diagnostic output.

**Prevention:**
- Set explicit timeouts: `go test -timeout 30m ./internal/controller/...`
- Configure per-test timeouts in the Makefile:
  ```makefile
  test: manifests generate fmt vet setup-envtest
  	KUBEBUILDER_ASSETS="..." go test -timeout 20m $$(go list ./... | grep -v /e2e) -coverprofile cover.out
  ```
- Set Ginkgo suite-level timeout: `SetDefaultEventuallyTimeout(2 * time.Minute)` (already in existing code -- good)
- Add `context.WithTimeout` to individual test helpers

**Phase to address:** Phase 1 (Go test infrastructure)

**Confidence:** HIGH -- standard Go testing behavior

---

### Pitfall 13: E2E Tests Pull Images from Docker Hub -- Rate Limiting in CI

**What goes wrong:**
GitHub Actions runners share IP ranges. Docker Hub imposes rate limits of 100 pulls per 6 hours for unauthenticated users. When multiple CI runs or other projects on the same runner IP hit Docker Hub, image pulls fail with `429 Too Many Requests`, causing e2e tests to fail.

**Prevention:**
- **Pre-pull and cache game server images** in the workflow:
  ```yaml
  - name: Pull and cache game images
    run: |
      docker pull itzg/minecraft-server:latest || true
      kind load docker-image itzg/minecraft-server:latest --name $KIND_CLUSTER
  ```
- **Use a lightweight stub image** instead of real game server images for most e2e tests
- **Authenticate with Docker Hub** using a free account (200 pulls / 6 hours):
  ```yaml
  - name: Login to Docker Hub
    uses: docker/login-action@v3
    with:
      username: ${{ secrets.DOCKERHUB_USERNAME }}
      password: ${{ secrets.DOCKERHUB_TOKEN }}
  ```
- **Mirror critical images** to GitHub Container Registry (ghcr.io) which has no rate limits for public images

**Phase to address:** Phase 5 (GitHub Actions CI) -- configure image strategy with rate limiting in mind

**Confidence:** HIGH -- well-documented Docker Hub rate limiting policy

---

### Pitfall 14: Test Data Coupling Between Playwright and API Tests

**What goes wrong:**
Playwright tests create users like "alice" and game servers like "mc-server-1" -- the same names used in Go API tests. When both test suites run against the same kind cluster (or if someone tries to run them simultaneously), they collide. More subtly, Playwright tests assume specific database state that the Go API tests may have modified.

**Prevention:**
- **Use distinct prefixes** for each test layer:
  - Go API tests: `api-test-*` names
  - Playwright tests: `e2e-test-*` names
  - Controller tests: `ctrl-test-*` names
- **Each Playwright test should create AND clean up its own data** -- never assume a resource exists from a previous test
- **Run Playwright and Go e2e tests in separate kind namespaces** or separate cluster instances
- **Use unique usernames per test file** to prevent auth state collisions

**Phase to address:** Phase 2 and Phase 4 -- establish naming conventions early

**Confidence:** MEDIUM -- project-specific analysis based on existing test code

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|---|---|---|
| Go unit tests (Phase 1) | Cached client flakiness (#1), namespace contamination (#2), test timeout (#12) | Use live client, unique namespaces, explicit timeouts |
| Go API integration tests (Phase 2) | Fake vs real client coverage gap (#9), test data naming collisions (#14) | Add envtest integration layer for critical paths, naming conventions |
| Kind cluster setup (Phase 3) | Image loading timeout (#3), disk space (#5), cluster cleanup (#7) | Local registry, disk cleanup step, always() cleanup |
| Playwright E2E (Phase 4) | K8s state wait strategy (#4), auth state management (#6), WebSocket flakiness (#8), brittle selectors (#10), CI workers (#11) | API-level waits, storageState, mock game servers, data-testid, workers:1 |
| GitHub Actions CI (Phase 5) | Disk space (#5), Docker Hub rate limits (#13), cluster cleanup (#7), total pipeline duration | Disk cleanup, image caching/mirroring, job timeouts, parallelization via matrix |

---

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **Test client setup:** Using manager's cached client instead of live client -- all assertions will be intermittently stale
- [ ] **Namespace cleanup in envtest:** Relying on namespace deletion for test isolation -- namespaces never actually delete in envtest
- [ ] **Kind image loading:** Using `kind load docker-image` without caching or size optimization -- CI will be 3-5x slower than necessary
- [ ] **Playwright timeouts:** Using default 30s timeout for operations that involve k8s reconciliation -- will timeout in CI
- [ ] **Auth state reuse:** Each test logs in through UI -- test suite will take 3-5x longer than necessary
- [ ] **Console tests:** Testing xterm.js against real game servers -- will be flaky due to WebSocket + Pod exec latency
- [ ] **Test data isolation:** Using hardcoded resource names shared across test layers -- tests will collide
- [ ] **CI disk space:** No cleanup of preinstalled software -- will run out of disk space with game server images
- [ ] **Docker Hub rate limits:** No authentication for image pulls in CI -- will get 429 errors during high-activity periods
- [ ] **Cleanup on failure:** Kind cluster cleanup only runs after successful tests -- failed runs leave dirty state on self-hosted runners
- [ ] **Test ID attributes:** No `data-testid` on React components -- Playwright tests coupled to implementation details
- [ ] **Coverage gap:** API tests use fake client, e2e tests are full-stack -- no integration tests covering API-to-controller interaction

---

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---|---|---|
| Cached client flaky tests (#1) | LOW | Replace `k8sManager.GetClient()` with `client.New()` in suite_test.go; add `Eventually` wrappers to existing assertions |
| Namespace contamination (#2) | MEDIUM | Refactor all tests to use random namespace names; add explicit resource cleanup; may require rewriting test setup/teardown |
| Image loading timeout (#3) | LOW | Add local registry to kind config; change Makefile to push to local registry instead of `kind load` |
| Playwright k8s waits (#4) | MEDIUM | Rewrite all state-dependent assertions to use `toPass()` with API polling; add `data-testid` to status elements |
| Disk space exhaustion (#5) | LOW | Add cleanup step to workflow; switch to stub images |
| Auth state not shared (#6) | LOW | Add global setup script; configure Playwright projects with dependencies |
| Kind cluster cleanup (#7) | LOW | Add `if: always()` to cleanup step; prefix cluster name with run ID |
| WebSocket console flakiness (#8) | MEDIUM | Replace real game server with mock echo server; separate functional and visual console tests |
| Fake/real client gap (#9) | HIGH | Requires adding new integration test layer; significant test architecture change |
| Brittle selectors (#10) | HIGH | Requires retrofitting `data-testid` to all React components; updating all Playwright selectors |

---

## Sources

### EnvTest and Controller Testing
- [Writing Tests - The Kubebuilder Book](https://book.kubebuilder.io/cronjob-tutorial/writing-tests)
- [Configuring EnvTest - The Kubebuilder Book](https://book.kubebuilder.io/reference/envtest)
- [Testing Kubernetes Operators using EnvTest - InfraCloud](https://www.infracloud.io/blogs/testing-kubernetes-operator-envtest/)
- [Testing Production Kubernetes Controllers - SuperOrbital](https://superorbital.io/blog/testing-production-controllers/)
- [Testing Kubernetes Operators with Ginkgo, Gomega and the Operator Runtime - ITNEXT](https://itnext.io/testing-kubernetes-operators-with-ginkgo-gomega-and-the-operator-runtime-6ad4c2492379)
- [Speeding Up Kubernetes Controller Integration Tests with Ginkgo Parallelism - Kevin Fan](https://kev.fan/posts/04-k8s-ginkgo-parallel-tests/)
- [Testing your Operator project - Operator SDK](https://sdk.operatorframework.io/docs/building-operators/golang/testing/)

### Kind and CI
- [Why Your Kubernetes Tests Are Flaky (It's Not the Code) - Testkube](https://testkube.io/blog/flaky-tests-cicd-kubernetes-infrastructure-issues)
- [Running Kubernetes e2e tests with Kind and GitHub Actions - Radu Matei](https://radu-matei.com/blog/kubernetes-e2e-github-actions/)
- [Testing Kubernetes Operators using GitHub Actions and Kind - Medium/CodeX](https://medium.com/codex/testing-kubernetes-operators-using-github-actions-and-kind-c4086d37dd30)
- [kind load docker-image slow - kind issue #3002](https://github.com/kubernetes-sigs/kind/issues/3002)
- [kind load docker-image performance - kind issue #2922](https://github.com/kubernetes-sigs/kind/issues/2922)
- [KiND - How I Wasted a Day Loading Local Docker Images - iximiuz](https://iximiuz.com/en/posts/kubernetes-kind-load-docker-image/)
- [How to Use Docker Images with Kind - OneUptime](https://oneuptime.com/blog/post/2026-02-08-how-to-use-docker-images-with-kind-kubernetes-in-docker/view)
- [helm/kind-action - GitHub](https://github.com/helm/kind-action)

### Playwright Testing
- [Authentication - Playwright Docs](https://playwright.dev/docs/auth)
- [Writing Tests - Playwright Docs](https://playwright.dev/docs/writing-tests)
- [Continuous Integration - Playwright Docs](https://playwright.dev/docs/ci)
- [How to Avoid Flaky Tests in Playwright - Semaphore](https://semaphore.io/blog/flaky-tests-playwright)
- [How to Detect and Avoid Playwright Flaky Tests - BrowserStack](https://www.browserstack.com/guide/playwright-flaky-tests)
- [17 Playwright Testing Mistakes You Should Avoid - Elaichenkov](https://elaichenkov.github.io/posts/17-playwright-testing-mistakes-you-should-avoid/)
- [Avoiding Flaky Tests in Playwright - Better Stack](https://betterstack.com/community/guides/testing/avoid-flaky-playwright-tests/)
- [Playwright WebSocket Testing - DZone](https://dzone.com/articles/playwright-for-real-time-applications-testing-webs)

### GitHub Actions Resource Management
- [Mastering Disk Space on GitHub Actions Runners - Gerald on IT](https://www.geraldonit.com/mastering-disk-space-on-github-actions-runners-a-deep-dive-into-cleanup-strategies-for-x64-and-arm64-runners/)
- [Freeing disk space on GitHub Actions runners - Chris Dzombak](https://www.dzombak.com/blog/2024/09/freeing-disk-space-on-github-actions-runners/)
- [GitHub Actions Limits - GitHub Docs](https://docs.github.com/en/actions/reference/limits)

### Kubernetes Test Isolation and Flakiness
- [Kubernetes Community - Flaky Tests Guide](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-testing/flaky-tests.md)
- [Resolve Stuck Namespace Deletions by Cleaning Finalizers - Medium](https://medium.com/@sirtcp/how-to-resolve-stuck-kubernetes-namespace-deletions-by-cleaning-finalizers-38190bf3165f)
- [kind create cluster flaky - kind issue #1865](https://github.com/kubernetes-sigs/kind/issues/1865)

---
*Pitfalls research for: Kterodactyl v1.1 E2E CI/CD Test Suite*
*Researched: 2026-02-17*
