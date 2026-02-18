# Phase 13: Go Test Foundation - Context

**Gathered:** 2026-02-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish a reliable Go test suite covering the three untested API handler groups (mod, backup, metrics proxy) with proper test isolation and Makefile-driven execution. Fix the envtest cached-client pattern. Integration tests and E2E tests are separate phases.

</domain>

<decisions>
## Implementation Decisions

### Coverage depth
- Test both happy paths AND error cases for all three handler groups (mod, backup, metrics proxy)
- Scope is strictly those three handler groups — no audit of other handlers
- Error case assertions use HTTP status codes only — do not couple tests to specific error message wording
- For file-handling endpoints (mod upload, backup restore), use mock/pre-built request bodies rather than real multipart form data — tests exercise handler logic, not HTTP parsing

### Claude's Discretion
- Test output verbosity and filtering approach
- Envtest cached-client fix strategy (minimal fix vs broader cleanup)
- Fake boundary decisions (what to fake for K8s client, S3, filesystem)
- Makefile target naming and test execution workflow
- Test file organization and naming conventions

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 13-go-test-foundation*
*Context gathered: 2026-02-18*
