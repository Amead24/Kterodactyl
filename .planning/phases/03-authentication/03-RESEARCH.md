# Phase 3: Authentication - Research

**Researched:** 2026-02-10
**Domain:** Go authentication system (JWT, password hashing, user storage, email invitations) for a Kubernetes operator project
**Confidence:** HIGH

## Summary

Phase 3 adds authentication to Kterodactyl: admin invitations, user self-registration, JWT session management, and per-user isolation enforcement. The core challenge is designing an auth system that is Kubernetes-native (no external database dependencies), integrates cleanly with the Phase 4 API server (Go REST gateway), and maintains the "single Helm chart" deployment promise.

The recommended approach stores user credentials in Kubernetes Secrets within the operator namespace, hashes passwords with Argon2id, issues JWT tokens via `golang-jwt/jwt/v5`, and sends invitation emails via `wneessen/go-mail`. The auth logic lives in `internal/auth/` as a reusable Go package that Phase 4's API server imports directly. User isolation is enforced by mapping authenticated usernames to their `user-<username>` namespace -- the same namespace pattern already established in Phase 1.

This approach keeps the operator single-binary, avoids adding SQLite or PostgreSQL dependencies, and stays Kubernetes-native. The user Secret pattern works well for homelab scale (< 100 users). OIDC/SSO is explicitly deferred to v2 (AUTH-05).

**Primary recommendation:** Store users as labeled Kubernetes Secrets in the operator namespace, hash passwords with Argon2id, issue HMAC-SHA256 JWTs, build auth as a reusable Go package in `internal/auth/` that Phase 4 consumes.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | JWT token creation and validation | De facto standard Go JWT library, 7k+ stars, actively maintained (last release Jan 2026), supports all standard signing methods |
| `golang.org/x/crypto/argon2` | latest | Password hashing with Argon2id | Go official extended library, Argon2id is the 2025-2026 gold standard for password hashing per Password Hashing Competition |
| `crypto/rand` | stdlib | Cryptographically secure random bytes | Go stdlib, used for generating salts, invitation tokens, and JWT signing keys |
| `k8s.io/client-go` | v0.35.0 | Kubernetes Secret CRUD for user storage | Already a project dependency, provides typed Secret operations |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `wneessen/go-mail` | v0.7.2 | SMTP email sending for invitations | When admin invites a user -- sends invitation email with registration link and token |
| `encoding/base64` | stdlib | Encoding hashed passwords and salts for Secret storage | Always -- Kubernetes Secret values are base64 |
| `net/http` | stdlib | HTTP handler interfaces for auth middleware | Auth middleware defined against stdlib interfaces for compatibility with Gin and any HTTP framework |
| `context` | stdlib | Passing authenticated user info through request context | Every authenticated request -- middleware sets user context |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Kubernetes Secrets (user store) | SQLite embedded (`modernc.org/sqlite` v1.45.0) | SQLite adds proper SQL queries and scales better, but adds a dependency, requires PVC for persistence, and breaks the "everything in K8s" model. Use SQLite if user count exceeds ~200. |
| Kubernetes Secrets (user store) | PostgreSQL | Production standard but massive overkill for homelab. Adds external dependency and operational burden. Defer to v2 if SaaS mode needed. |
| Kubernetes Secrets (user store) | User CRD | Architecture research explicitly warns against storing user data in CRDs (Anti-Pattern 3: etcd size limits, no ACID, poor query performance, RBAC security risk). Secrets are marginally better since they can leverage K8s encryption-at-rest. |
| Argon2id | bcrypt (`golang.org/x/crypto/bcrypt`) | bcrypt is simpler (single function call) but less resistant to GPU attacks. Argon2id is the current recommendation for all new systems in 2025-2026. |
| `wneessen/go-mail` | `jordan-wright/email` | jordan-wright/email is simpler but less actively maintained. go-mail has 1687 commits, 44 releases, SMTP auth variety, and active development. |
| HMAC-SHA256 JWT | RSA/ECDSA JWT | Asymmetric signing useful when multiple services verify tokens independently. For Kterodactyl v1, the API server both signs and verifies -- HMAC is simpler and sufficient. Switch to RS256 if OIDC integration added in v2. |

**Installation:**

```bash
go get github.com/golang-jwt/jwt/v5@v5.3.1
go get golang.org/x/crypto
go get github.com/wneessen/go-mail
```

## Architecture Patterns

### Recommended Project Structure

```
internal/
  auth/
    auth.go           # Public API: UserService interface, types (User, Claims, InviteToken)
    jwt.go            # JWT token generation, validation, claims parsing
    password.go       # Argon2id hashing, verification, salt generation
    store.go          # Kubernetes Secret-based user CRUD (implements UserService)
    middleware.go     # HTTP middleware: extract JWT, validate, set context
    invite.go         # Invitation token generation, email sending, redemption
    errors.go         # Typed auth errors (ErrInvalidCredentials, ErrUserExists, etc.)
    auth_test.go      # Unit tests with mock client
```

### Pattern 1: Kubernetes Secret as User Record

**What:** Each user is stored as a Kubernetes Secret in the operator namespace, with labels for querying and data fields for credentials.

**When to use:** For all user CRUD operations. This is the primary storage mechanism.

**Example:**

```go
// User Secret structure in kterodactyl-system namespace
// Name: user-<username>
// Labels:
//   kterodactyl.io/managed-by: kterodactyl
//   kterodactyl.io/resource-type: user
//   kterodactyl.io/user: <username>
//   kterodactyl.io/role: admin|user
// Data:
//   email: <email>
//   password-hash: <argon2id hash with embedded params>
//   created-at: <RFC3339 timestamp>
//   invited-by: <admin username who sent invite>

secret := &corev1.Secret{
    ObjectMeta: metav1.ObjectMeta{
        Name:      fmt.Sprintf("user-%s", username),
        Namespace: operatorNamespace,
        Labels: map[string]string{
            "kterodactyl.io/managed-by":    "kterodactyl",
            "kterodactyl.io/resource-type": "user",
            "kterodactyl.io/user":          username,
            "kterodactyl.io/role":          "user",
        },
    },
    Type: corev1.SecretTypeOpaque,
    Data: map[string][]byte{
        "email":         []byte(email),
        "password-hash": []byte(hashedPassword),
        "created-at":    []byte(time.Now().UTC().Format(time.RFC3339)),
    },
}
```

### Pattern 2: JWT Claims Structure with Namespace Mapping

**What:** JWT tokens encode the username, role, and namespace. The Phase 4 API server extracts the namespace from the token to scope all Kubernetes operations to that user's namespace.

**When to use:** Every authenticated request. This is how user isolation (AUTH-04) is enforced.

**Example:**

```go
// Custom claims extending RegisteredClaims
type KterodactylClaims struct {
    jwt.RegisteredClaims
    Username  string `json:"username"`
    Email     string `json:"email"`
    Role      string `json:"role"`      // "admin" or "user"
    Namespace string `json:"namespace"` // "user-<username>"
}

// Token creation
func (s *JWTService) GenerateToken(user *User) (string, error) {
    claims := &KterodactylClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   user.Username,
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            Issuer:    "kterodactyl",
        },
        Username:  user.Username,
        Email:     user.Email,
        Role:      user.Role,
        Namespace: util.UserNamespace(user.Username), // "user-<username>"
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.signingKey)
}
```

### Pattern 3: Auth Middleware as stdlib http.Handler

**What:** Auth middleware is defined against Go stdlib interfaces (not Gin-specific) so Phase 4 can use it regardless of HTTP framework choice.

**When to use:** On all protected routes in Phase 4's API server.

**Example:**

```go
// Middleware returns an http.Handler wrapper
// Compatible with any framework that supports stdlib middleware
func (a *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tokenString := extractBearerToken(r)
        if tokenString == "" {
            http.Error(w, "missing authorization header", http.StatusUnauthorized)
            return
        }

        claims, err := a.jwtService.ValidateToken(tokenString)
        if err != nil {
            http.Error(w, "invalid token", http.StatusUnauthorized)
            return
        }

        // Set user info in context for downstream handlers
        ctx := context.WithValue(r.Context(), ContextKeyUser, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Extract from context in handlers (Phase 4)
func GetUserFromContext(ctx context.Context) *KterodactylClaims {
    claims, _ := ctx.Value(ContextKeyUser).(*KterodactylClaims)
    return claims
}
```

### Pattern 4: Invitation Token Flow

**What:** Admin creates an invitation that generates a time-limited, single-use token. Token is emailed to the invitee. User redeems token during registration.

**When to use:** AUTH-01 (admin invites user) and AUTH-02 (user self-registers).

**Flow:**

```
1. Admin calls POST /api/invites {email: "alice@example.com"}
2. Server generates random token, stores as K8s Secret with TTL annotation
3. Server sends email with link: https://panel.domain.com/register?token=<token>
4. User visits link, fills in username + password
5. Server validates token (exists, not expired, not used), creates user
6. Server marks invitation Secret as used (or deletes it)
7. Server returns JWT token -- user is now logged in
```

```go
// Invitation Secret structure
// Name: invite-<random-id>
// Labels:
//   kterodactyl.io/managed-by: kterodactyl
//   kterodactyl.io/resource-type: invite
// Annotations:
//   kterodactyl.io/expires-at: <RFC3339 timestamp>
// Data:
//   token: <cryptographically random 32-byte hex string>
//   email: <invited email>
//   invited-by: <admin username>
```

### Pattern 5: JWT Signing Key Storage

**What:** The JWT HMAC signing key is stored as a Kubernetes Secret in the operator namespace. If it does not exist, the auth package generates one on first startup and persists it.

**When to use:** On auth package initialization. Critical for token persistence across restarts.

**Example:**

```go
const jwtKeySecretName = "kterodactyl-jwt-signing-key"

func (s *JWTService) EnsureSigningKey(ctx context.Context) error {
    secret := &corev1.Secret{}
    err := s.client.Get(ctx, types.NamespacedName{
        Name:      jwtKeySecretName,
        Namespace: s.operatorNamespace,
    }, secret)

    if errors.IsNotFound(err) {
        // Generate new 256-bit key
        key := make([]byte, 32)
        if _, err := rand.Read(key); err != nil {
            return fmt.Errorf("failed to generate signing key: %w", err)
        }

        secret = &corev1.Secret{
            ObjectMeta: metav1.ObjectMeta{
                Name:      jwtKeySecretName,
                Namespace: s.operatorNamespace,
                Labels: map[string]string{
                    "kterodactyl.io/managed-by":    "kterodactyl",
                    "kterodactyl.io/resource-type": "jwt-key",
                },
            },
            Type: corev1.SecretTypeOpaque,
            Data: map[string][]byte{
                "signing-key": key,
            },
        }
        return s.client.Create(ctx, secret)
    }
    if err != nil {
        return err
    }

    s.signingKey = secret.Data["signing-key"]
    return nil
}
```

### Anti-Patterns to Avoid

- **Storing passwords in CRD spec/status:** CRDs are not encrypted at rest by default. Use Kubernetes Secrets which can leverage EncryptionConfiguration for at-rest encryption.
- **Storing JWT signing key in ConfigMap:** ConfigMaps are not designed for sensitive data. Use a Secret.
- **Using bcrypt for new code in 2026:** Argon2id provides superior GPU/ASIC resistance. bcrypt is acceptable for existing systems but not recommended for greenfield.
- **Gin-specific middleware:** Tying auth middleware to Gin makes it impossible to reuse with other frameworks. Use stdlib `http.Handler` pattern.
- **Embedding user data in JWT without validation:** Always re-validate user existence on sensitive operations (e.g., server creation). JWT claims can be stale if user was deleted after token issuance.
- **Hardcoded JWT expiration:** Make token expiration configurable via AdminConfig ConfigMap.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Password hashing | Custom hash function or raw SHA-256 | `golang.org/x/crypto/argon2` (IDKey) | Timing attacks, salt management, parameter tuning, GPU resistance -- all solved by Argon2id |
| JWT creation/parsing | Manual base64 + HMAC + JSON marshaling | `golang-jwt/jwt/v5` | Algorithm negotiation attacks, claims validation, clock skew, signature verification edge cases |
| Email sending | Raw `net/smtp` calls | `wneessen/go-mail` | TLS negotiation, SMTP auth methods, connection pooling, RFC compliance, error handling |
| Random token generation | `math/rand` | `crypto/rand` | `math/rand` is not cryptographically secure -- tokens would be predictable |
| Argon2 parameter encoding | Custom string format for stored hashes | PHC string format (`$argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>`) | Standard format, self-describing, forward-compatible with parameter upgrades |

**Key insight:** Authentication is a security-critical domain where subtle implementation bugs create exploitable vulnerabilities. Every component (hashing, tokens, random generation, email) has well-tested standard solutions. Hand-rolling any of these creates security risk with zero benefit.

## Common Pitfalls

### Pitfall 1: JWT Signing Key Lost on Pod Restart

**What goes wrong:** JWT signing key generated in memory at startup. When pod restarts, new key is generated. All existing tokens become invalid, logging out all users simultaneously.

**Why it happens:** Developers generate the key in `init()` or `main()` without persisting it.

**How to avoid:** Store the signing key in a Kubernetes Secret (pattern 5 above). On startup, check if the Secret exists; if not, generate and persist. If it exists, load from Secret.

**Warning signs:** All users logged out after operator/API server restart.

### Pitfall 2: Timing Attack on Password Comparison

**What goes wrong:** Using `==` to compare password hashes leaks information about which bytes match via response timing differences.

**Why it happens:** Standard string comparison short-circuits on first mismatch.

**How to avoid:** Use `crypto/subtle.ConstantTimeCompare()` for hash comparison. Better yet, use Argon2id's IDKey to re-derive the hash and compare the derived output -- the comparison is against a freshly computed value, not a stored one.

**Warning signs:** Authentication endpoint response time varies based on how "close" the password guess is.

### Pitfall 3: Invitation Token Reuse

**What goes wrong:** Invitation token is not invalidated after use. Attacker with access to the email can create multiple accounts from one invitation.

**Why it happens:** Token validation checks existence and expiry but forgets to delete or mark as used.

**How to avoid:** Delete the invitation Secret immediately upon successful registration, before returning the JWT. Use atomic operations: validate-and-delete in a single reconciliation.

**Warning signs:** Multiple accounts created from the same invitation email.

### Pitfall 4: Username Injection into Namespace Name

**What goes wrong:** Username containing special characters breaks namespace creation or allows accessing other users' namespaces.

**Why it happens:** `UserNamespace()` already does `fmt.Sprintf("user-%s", username)` but doesn't validate the username input.

**How to avoid:** Validate usernames against DNS label rules (lowercase alphanumeric + hyphens, 1-63 chars) during registration. The `UserNamespace()` function is already established -- ensure registration validates that the resulting namespace name is valid.

**Warning signs:** Registration fails with "invalid namespace name" errors; or worse, crafted usernames like `../admin` cause path traversal.

### Pitfall 5: Admin Bootstrap Problem

**What goes wrong:** No admin user exists on first install. Admin endpoints require admin auth. Chicken-and-egg: can't create admin without admin.

**Why it happens:** Developers forget about the initial setup flow.

**How to avoid:** Provide a bootstrap mechanism: either (a) a CLI command `kterodactyl-admin create-admin --email admin@example.com --password <pwd>` that creates the admin Secret directly, or (b) check if any admin user exists on startup and auto-create one from Helm values / environment variables, or (c) check for a bootstrap Secret with initial admin credentials.

**Warning signs:** Fresh install has no way to access admin features.

### Pitfall 6: SMTP Configuration Missing at Runtime

**What goes wrong:** Admin tries to send invitation, but SMTP is not configured. Error is opaque.

**Why it happens:** Email requires external SMTP server config (host, port, credentials). Not everyone has one.

**How to avoid:** Make email optional: if SMTP is not configured in AdminConfig, invitation endpoint returns the registration link directly in the API response (admin can manually share it). Log a warning that email sending is disabled. Store SMTP config in AdminConfig ConfigMap.

**Warning signs:** Invitation endpoint returns 500 with "dial tcp: connection refused."

### Pitfall 7: JWT Token Not Refreshed Before Expiry

**What goes wrong:** User's 24-hour token expires mid-session. Frontend makes API call, gets 401, user loses unsaved work.

**Why it happens:** No refresh mechanism; frontend doesn't track token expiry proactively.

**How to avoid:** Implement token refresh: if token is valid but within 1 hour of expiry, issue a new token automatically in the response header. Frontend checks for refreshed token in responses. Alternative: use short-lived access tokens (15min) + longer-lived refresh tokens -- but this adds complexity. For v1, single token with auto-refresh on API calls is simpler.

**Warning signs:** Users report being randomly logged out after ~24 hours.

## Code Examples

Verified patterns from official sources:

### Argon2id Password Hashing (PHC String Format)

```go
// Source: golang.org/x/crypto/argon2 docs + PHC string format spec
package auth

import (
    "crypto/rand"
    "crypto/subtle"
    "encoding/base64"
    "fmt"
    "strings"

    "golang.org/x/crypto/argon2"
)

// Argon2id parameters (OWASP 2024 recommended minimums)
const (
    argonTime    = 1
    argonMemory  = 64 * 1024 // 64 MB
    argonThreads = 4
    argonKeyLen  = 32
    argonSaltLen = 16
)

// HashPassword hashes a password using Argon2id and returns a PHC-format string.
func HashPassword(password string) (string, error) {
    salt := make([]byte, argonSaltLen)
    if _, err := rand.Read(salt); err != nil {
        return "", fmt.Errorf("failed to generate salt: %w", err)
    }

    hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

    // PHC string format: $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
    return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
        argon2.Version,
        argonMemory, argonTime, argonThreads,
        base64.RawStdEncoding.EncodeToString(salt),
        base64.RawStdEncoding.EncodeToString(hash),
    ), nil
}

// VerifyPassword checks a password against a PHC-format Argon2id hash.
func VerifyPassword(password, encodedHash string) (bool, error) {
    // Parse PHC string format
    parts := strings.Split(encodedHash, "$")
    if len(parts) != 6 {
        return false, fmt.Errorf("invalid hash format")
    }

    var version int
    var memory, time uint32
    var threads uint8
    _, _ = fmt.Sscanf(parts[2], "v=%d", &version)
    _, _ = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads)

    salt, err := base64.RawStdEncoding.DecodeString(parts[4])
    if err != nil {
        return false, fmt.Errorf("failed to decode salt: %w", err)
    }

    expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
    if err != nil {
        return false, fmt.Errorf("failed to decode hash: %w", err)
    }

    // Re-derive hash with same parameters
    computedHash := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(expectedHash)))

    // Constant-time comparison
    return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1, nil
}
```

### JWT Token Generation and Validation

```go
// Source: github.com/golang-jwt/jwt/v5 official docs
package auth

import (
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
    signingKey       []byte
    tokenExpiration  time.Duration
}

type KterodactylClaims struct {
    jwt.RegisteredClaims
    Username  string `json:"username"`
    Email     string `json:"email"`
    Role      string `json:"role"`
    Namespace string `json:"namespace"`
}

func NewJWTService(signingKey []byte, expiration time.Duration) *JWTService {
    return &JWTService{
        signingKey:      signingKey,
        tokenExpiration: expiration,
    }
}

func (s *JWTService) GenerateToken(user *User) (string, error) {
    now := time.Now()
    claims := &KterodactylClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   user.Username,
            Issuer:    "kterodactyl",
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenExpiration)),
            ID:        generateTokenID(), // For potential revocation
        },
        Username:  user.Username,
        Email:     user.Email,
        Role:      user.Role,
        Namespace: UserNamespace(user.Username),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.signingKey)
}

func (s *JWTService) ValidateToken(tokenString string) (*KterodactylClaims, error) {
    claims := &KterodactylClaims{}

    token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
        return s.signingKey, nil
    },
        jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
        jwt.WithIssuer("kterodactyl"),
        jwt.WithExpirationRequired(),
    )
    if err != nil {
        return nil, fmt.Errorf("token validation failed: %w", err)
    }

    if !token.Valid {
        return nil, fmt.Errorf("token is not valid")
    }

    return claims, nil
}
```

### Kubernetes Secret-Based User Store

```go
// Source: k8s.io/client-go Secret operations
package auth

import (
    "context"
    "fmt"

    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

type UserStore struct {
    client    client.Client
    namespace string // operator namespace (kterodactyl-system)
}

type User struct {
    Username     string
    Email        string
    PasswordHash string
    Role         string // "admin" or "user"
    CreatedAt    string
    InvitedBy    string
}

func (s *UserStore) CreateUser(ctx context.Context, user *User) error {
    secret := &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fmt.Sprintf("user-%s", user.Username),
            Namespace: s.namespace,
            Labels: map[string]string{
                "kterodactyl.io/managed-by":    "kterodactyl",
                "kterodactyl.io/resource-type": "user",
                "kterodactyl.io/user":          user.Username,
                "kterodactyl.io/role":          user.Role,
            },
        },
        Type: corev1.SecretTypeOpaque,
        Data: map[string][]byte{
            "email":         []byte(user.Email),
            "password-hash": []byte(user.PasswordHash),
            "created-at":    []byte(user.CreatedAt),
            "invited-by":    []byte(user.InvitedBy),
        },
    }

    if err := s.client.Create(ctx, secret); err != nil {
        if errors.IsAlreadyExists(err) {
            return ErrUserExists
        }
        return fmt.Errorf("failed to create user secret: %w", err)
    }
    return nil
}

func (s *UserStore) GetUser(ctx context.Context, username string) (*User, error) {
    secret := &corev1.Secret{}
    err := s.client.Get(ctx, client.ObjectKey{
        Name:      fmt.Sprintf("user-%s", username),
        Namespace: s.namespace,
    }, secret)
    if err != nil {
        if errors.IsNotFound(err) {
            return nil, ErrUserNotFound
        }
        return nil, fmt.Errorf("failed to get user secret: %w", err)
    }
    return userFromSecret(secret), nil
}

func (s *UserStore) ListUsers(ctx context.Context) ([]*User, error) {
    secrets := &corev1.SecretList{}
    err := s.client.List(ctx, secrets,
        client.InNamespace(s.namespace),
        client.MatchingLabels{
            "kterodactyl.io/managed-by":    "kterodactyl",
            "kterodactyl.io/resource-type": "user",
        },
    )
    if err != nil {
        return nil, fmt.Errorf("failed to list user secrets: %w", err)
    }

    users := make([]*User, 0, len(secrets.Items))
    for i := range secrets.Items {
        users = append(users, userFromSecret(&secrets.Items[i]))
    }
    return users, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| bcrypt for password hashing | Argon2id | 2015 (PHC winner), widespread adoption 2023+ | GPU/ASIC resistant, memory-hard, OWASP recommended |
| `dgrijalva/jwt-go` | `golang-jwt/jwt/v5` | 2021 (fork), v5 in 2023 | Original archived, v5 has better Claims interface and validation |
| Symmetric JWT only | Symmetric for single-service, asymmetric for multi-service | Ongoing | HMAC fine for v1 single-service; switch to RS256/EdDSA if OIDC added in v2 |
| Session cookies | JWT Bearer tokens | Ongoing | JWTs are stateless, work with API-first architecture, SPA-friendly |
| `net/smtp` directly | Dedicated mail libraries (`go-mail`) | 2022+ | Better TLS handling, auth methods, connection pooling |

**Deprecated/outdated:**
- `dgrijalva/jwt-go`: Archived, do not use. Use `golang-jwt/jwt/v5` instead.
- bcrypt for new systems: Still secure but Argon2id is strictly better for greenfield projects.
- Session cookies for API-first apps: JWT tokens are standard for SPA + API architectures.

## Integration with Existing Code

### How Auth Connects to Existing Operator Code

The existing codebase has several touch points that auth integrates with:

1. **`internal/util/labels.go`** -- Already has `UserNamespace()` function that returns `user-<username>`. Auth claims encode this same namespace. No changes needed.

2. **`internal/util/labels.go`** -- Already has `LabelOwner`, `LabelUser` constants. Auth should use these same labels on user Secrets for consistency.

3. **`internal/controller/gameserver_controller.go`** -- Already validates `LabelOwner` on GameServer CRs. Phase 4 API server will set this label based on authenticated user from JWT claims.

4. **`AdminConfig`** -- Auth-related configuration (JWT expiration, SMTP settings, admin bootstrap) should be added to the admin ConfigMap, loaded by the same `LoadAdminConfig()` pattern.

5. **`OperatorNamespace`** -- User Secrets and JWT key Secret live in the operator namespace. Auth package needs this value, available via `OPERATOR_NAMESPACE` env var (same pattern as existing code).

### AdminConfig Extensions for Auth

```go
// Additional fields to add to AdminConfig struct
type AdminConfig struct {
    // ... existing fields ...

    // Authentication
    JWTExpirationHours  int    // JWT token lifetime (default: 24)
    InviteExpirationHours int  // Invitation token lifetime (default: 72)

    // SMTP (optional -- invites work without email, returning link in response)
    SMTPHost     string
    SMTPPort     int
    SMTPUsername string
    SMTPPassword string // Stored in separate Secret, referenced by name
    SMTPFrom     string // e.g., "Kterodactyl <noreply@tonymead.org>"

    // Registration
    RegistrationEnabled  bool   // Allow self-registration (default: false, invite-only)
    PanelURL             string // Base URL for invitation links (e.g., "https://panel.tonymead.org")
}
```

### RBAC Requirements

The operator ServiceAccount needs additional permissions for auth:

```yaml
# Additional RBAC for auth (secrets in operator namespace)
# +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
```

Note: The existing RBAC already includes `configmaps` read access. Secrets CRUD is the only addition needed.

## Open Questions

1. **Admin Bootstrap Method**
   - What we know: First install needs an admin user. Options: CLI command, Helm values, bootstrap Secret.
   - What's unclear: Which UX is best for the homelab target audience.
   - Recommendation: Use Helm values for initial admin credentials (`values.auth.adminEmail`, `values.auth.adminPassword`). The operator creates the admin user Secret on first startup if no admin exists. This is the simplest UX -- admin is ready after `helm install`.

2. **Token Refresh Strategy**
   - What we know: 24h token expiry is reasonable for v1. Users will be frustrated if they lose sessions.
   - What's unclear: Whether to implement refresh tokens (access + refresh pair) or auto-refresh (extend on activity).
   - Recommendation: For v1, use auto-refresh: if the token is valid and within 2 hours of expiry, include a fresh token in a response header (`X-Refresh-Token`). Frontend replaces the stored token. Simpler than refresh token pairs, good enough for homelab scale.

3. **Email Requirement for Invitations**
   - What we know: AUTH-01 says "admin can send email invitations." SMTP may not be configured in all homelabs.
   - What's unclear: Whether email should be strictly required or optional.
   - Recommendation: Make email optional. If SMTP is configured, send invitation email. If not, return the invitation link in the API response so admin can share it manually (e.g., via Discord, Slack). This keeps the feature usable without SMTP.

4. **User Deletion and Namespace Cleanup**
   - What we know: Users own namespaces with GameServers. Deleting a user should clean up their resources.
   - What's unclear: Whether user deletion should cascade to namespace deletion or just disable the user.
   - Recommendation: Defer cascade deletion to future work. For v1, deleting a user disables login but does not delete their namespace or servers. Admin can manually clean up. This prevents accidental data loss.

## Sources

### Primary (HIGH confidence)

- [golang-jwt/jwt v5.3.1 - pkg.go.dev](https://pkg.go.dev/github.com/golang-jwt/jwt/v5) - JWT API, claims types, signing methods, parser options
- [golang.org/x/crypto/argon2 - pkg.go.dev](https://pkg.go.dev/golang.org/x/crypto/argon2) - Argon2id IDKey function, parameter documentation
- [Kubernetes Secrets documentation](https://kubernetes.io/docs/concepts/configuration/secret/) - Secret storage, encryption at rest, RBAC
- [wneessen/go-mail v0.7.2 - GitHub](https://github.com/wneessen/go-mail) - SMTP email library, version, features, maintenance status
- [modernc.org/sqlite v1.45.0 - pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite) - Pure Go SQLite (evaluated as alternative, not recommended for v1)

### Secondary (MEDIUM confidence)

- [Password Hashing Guide 2025: Argon2 vs Bcrypt](https://guptadeepak.com/the-complete-guide-to-password-hashing-argon2-vs-bcrypt-vs-scrypt-vs-pbkdf2-2026/) - Argon2id as current gold standard, recommended parameters
- [Neon Guides: Go JWT + PostgreSQL Authentication](https://neon.com/guides/golang-jwt) - JWT auth flow patterns in Go
- [Leapcell: JWT Authentication in Gin Middleware](https://leapcell.io/blog/secure-your-apis-with-jwt-authentication-in-gin-middleware) - Gin middleware patterns for JWT
- [Dex CRD Storage Discussion](https://github.com/dexidp/dex/discussions/2310) - Security of CRD vs Secret storage for sensitive data
- [Mailtrap: Go Send Email Tutorial 2026](https://mailtrap.io/blog/golang-send-email/) - Go email library comparison and usage

### Tertiary (LOW confidence)

- [Medium: Argon2 vs Bcrypt](https://medium.com/@lastgigin0/argon2-vs-bcrypt-the-modern-standard-for-secure-passwords-6d19911485c5) - Single source, but aligned with OWASP guidance
- [GitHub: sebnyberg/sqlite-migrate-example](https://github.com/sebnyberg/sqlite-migrate-example) - SQLite + go:embed migration pattern (for future reference if SQLite needed)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - golang-jwt/jwt v5 and golang.org/x/crypto/argon2 are official/semi-official Go packages with verified versions
- Architecture: HIGH - Kubernetes Secret storage pattern verified against K8s docs; integration points validated against existing codebase
- Pitfalls: HIGH - Password hashing, JWT, and invitation flow pitfalls are well-documented in security literature
- User storage decision: MEDIUM - Kubernetes Secrets work at homelab scale but are unconventional for user management; may need migration to SQLite/PostgreSQL if scale increases

**Research date:** 2026-02-10
**Valid until:** 2026-03-10 (30 days -- stable domain, libraries are mature)
