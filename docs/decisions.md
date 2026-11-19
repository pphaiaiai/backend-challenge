# Design Decisions and Assumptions

Each entry records what was decided, why, and what it costs.
Entries are added as the project grows.

## 1. Architecture

### D1. Hexagonal architecture (ports and adapters)

**Decision.** The code is split into three rings:

| Ring | Package | May import |
| --- | --- | --- |
| Domain | `internal/domain` | standard library only |
| Application (use cases + ports) | `internal/application` | domain |
| Adapters (HTTP, gRPC, MongoDB, JWT, bcrypt, worker) | `internal/adapters/...` | application, domain, frameworks, drivers |

`cmd/api/main.go` is the composition root: the only place that knows every
concrete type and wires them together.

**Why.** Business rules can be tested without MongoDB or HTTP, REST and gRPC
share one implementation of each use case, and infrastructure (database,
framework) can change without touching business logic.

**Trade-off.** More packages and some mapping code (domain ↔ MongoDB
document ↔ JSON DTO). For a CRUD service this size the structure is more than
strictly necessary; it pays off here because there are two entry points
(REST and gRPC) and the database must be mocked in tests.

### D2. Interfaces are declared by the package that uses them

**Decision.** Outbound ports (`UserRepository`, `PasswordHasher`,
`TokenService`) live in `internal/application/ports.go`, not next to their
implementations. Inbound adapters (HTTP, gRPC) likewise declare the small
interface of the service they call.

**Why.** It keeps every dependency arrow pointing inward and follows the Go
proverb "accept interfaces, return structs". Because Go interfaces are
satisfied implicitly, adapters never import the application just to declare
that they implement something.

### D3. HTTP framework: Gin, confined to the HTTP adapter

**Decision.** Gin is used for routing, JSON binding and request validation,
and is imported only by `internal/adapters/inbound/http`.

**Why.** It is widely known, provides binding and validation out of the box,
and remains a standard `http.Handler`, so `net/http`'s `http.Server`
(graceful shutdown) and `httptest` work unchanged. Fiber was not chosen
because it is built on `fasthttp`, which is not compatible with `net/http`,
and raw router speed is not the bottleneck here (MongoDB and bcrypt are).

**Consequence.** Replacing Gin would only change one package.

## 2. User model and validation

### D4. User IDs are opaque strings in the domain

**Decision.** `domain.User.ID` is a `string`. The MongoDB adapter converts it
to and from `ObjectID`. An ID that is not a valid ObjectID is treated as
"user not found" (HTTP 404), not as a bad request.

**Why.** Only the database adapter knows what a valid ID looks like. If the
HTTP layer validated ObjectID format, it would leak a MongoDB detail into the
transport layer.

### D5. Emails are normalized and unique

**Decision.** Emails are trimmed and lower-cased before validation, storage
and lookup. Uniqueness is guaranteed by a unique index in MongoDB, and a
duplicate-key error is translated to `domain.ErrEmailAlreadyExists`
(HTTP 409).

**Why.** Without normalization, `Alice@Example.com` and `alice@example.com`
could become two accounts. Checking "does the email exist?" before inserting
is not enough on its own: two concurrent requests can both pass the check.
Only the database can enforce uniqueness atomically.

**Assumption.** Treating the whole address as case-insensitive is a
simplification; RFC 5321 technically allows case-sensitive local parts, but
virtually no mail provider uses them.

### D6. Password policy: 8 characters to 72 bytes, hashed with bcrypt

**Decision.** Passwords must be at least 8 characters and at most 72 bytes.
They are hashed with bcrypt and never returned by any endpoint.

**Why.** bcrypt is slow by design and salts every hash. It ignores
everything after 72 bytes, so longer passwords are rejected rather than
silently truncated.

### D7. Validation happens at two levels

**Decision.**

1. The HTTP adapter validates request shape with Gin binding tags
   (required fields, email format) and answers 400.
2. The domain validates the same rules again, plus business rules such as
   email uniqueness.

**Why.** The domain is the single source of truth: requests that arrive
through gRPC do not pass through Gin binding, yet must obey the same rules.
Validating early in the HTTP layer gives fast, field-specific errors.

### D8. Timestamps are UTC with millisecond precision

**Why.** MongoDB stores dates with millisecond precision. Truncating when the
user is created makes the `createdAt` returned by `POST` identical to what a
later `GET` returns.

## 3. Authentication and authorization

### D9. Login does not reveal whether an email is registered

**Decision.** An unknown email and a wrong password both return
`invalid email or password` (HTTP 401). When the email is unknown, the
service still runs a bcrypt comparison against a dummy hash.

**Why.** Different messages, or a much faster response for unknown emails,
would let an attacker discover which emails have accounts (user
enumeration).

### D10. Any authenticated user may manage users

**Assumption.** The user model has no roles, so every valid token may call
every protected endpoint. Role-based access control (for example, only
admins may delete users) is out of scope.

### D11. Registration and "create user" share one use case

**Decision.** `POST /api/v1/auth/register` is public and
`POST /api/v1/users` requires a token; both call `UserService.Create`.

**Why.** The requirements list both "user registration" and "create a new
user". They differ only in who may call them, which is a transport concern.

## 4. API behaviour

### D12. Listing users is paginated

**Decision.** `GET /api/v1/users` accepts `offset` and `limit`
(default 20, maximum 100) and returns the total count.

**Why.** An unbounded "list all" query becomes slow and memory-hungry as the
collection grows. Clients can still page through every user.

**Trade-off.** `skip`-based pagination gets slower for very large offsets,
because MongoDB still walks the skipped documents. Cursor (keyset)
pagination, e.g. "users with `_id` greater than X", would scale better but is
harder for clients to use; it is unnecessary at this scale.

## 5. Persistence (MongoDB)

### D13. The storage format is separate from the domain entity

**Decision.** The MongoDB adapter has its own `userDocument` struct with
`bson` tags (`_id`, `name`, `email`, `passwordHash`, `createdAt`) and maps it
to and from `domain.User`.

**Why.** Renaming a stored field, or switching databases, never touches the
domain or the use cases.

### D14. The adapter generates the ObjectID before inserting

**Why.** The ID is known without reading `InsertOneResult.InsertedID` back
and type-asserting it, and the ObjectID's leading timestamp keeps `_id`
ordered by creation time, which `List` relies on for its sort order.

### D15. The unique email index is created at start-up

**Decision.** `UserRepository.EnsureIndexes` creates a unique index on
`email` every time the service starts.

**Why.** Creating an index that already exists is a no-op, so the service
can start against an empty database without a separate migration step.

**Trade-off.** In a larger system, index creation would usually be a
deployment or migration step, because building an index on a big collection
takes time and resources.

### D16. Every database call has a timeout

**Decision.** The client is created with a client-wide operation timeout
(`options.Client().SetTimeout`), configurable through `MONGO_TIMEOUT`.

**Why.** A call whose context has no deadline would otherwise wait forever on
an unreachable database. Request contexts are still passed through, so a
client that disconnects cancels its query too.

### D17. Counting uses CountDocuments

**Decision.** `Count` uses `CountDocuments` (exact).

**Trade-off.** `EstimatedDocumentCount` reads collection metadata in
constant time but can be inaccurate after an unclean shutdown. Exact counts
are cheap at this scale; the estimate would be the better choice for very
large collections where an approximate figure is acceptable.

## 6. Password hashing and tokens

### D18. bcrypt cost is configurable, default 10

**Why.** Each +1 doubles the hashing time. Cost 10 (`bcrypt.DefaultCost`)
takes tens of milliseconds, which slows down brute force without making
logins noticeably slow. The cost can be raised as hardware gets faster;
existing hashes keep working because each hash stores its own cost.

### D19. JWT design

**Decision.**

- Algorithm: HS256 only. The verifier rejects every other `alg`, including
  `none` and HS512, to prevent algorithm-confusion attacks.
- Claims: `sub` (user ID), `iss` (checked), `iat`, `exp` (required).
- Secret: read from `JWT_SECRET`, at least 32 bytes (256 bits, matching the
  HS256 output size). The service refuses to start with a shorter secret.
- Lifetime: 1 hour by default (`JWT_TTL`).

**Trade-off.** Tokens are stateless: the server keeps no session list, so a
token cannot be revoked before it expires, and a deleted user's token keeps
working until then. The short lifetime limits that window. Revocation would
need a denylist (for example, in Redis) or a "token version" field checked on
each request, at the cost of a database lookup per request. Refresh tokens
are out of scope.

## 7. Testing strategy

### D20. Mock at the port, test adapters for real

**Decision.**

| Layer | How it is tested |
| --- | --- |
| Domain | Plain unit tests |
| Application | Unit tests with hand-written fakes for `UserRepository`, `PasswordHasher` and `TokenService` |
| bcrypt and JWT adapters | Unit tests against the real libraries, including attack cases (tampered payload, `alg: none`, wrong secret, expired token) |
| MongoDB adapter | Unit tests for the domain ↔ document mapping, plus integration tests against a real MongoDB, behind the `integration` build tag |

**Why.** "Mock MongoDB interactions" is done at the `UserRepository`
interface, where the application meets the database. Mocking the driver
itself (collections, cursors, results) would mostly test the mock, and could
not prove things such as "a duplicate email really returns
`ErrEmailAlreadyExists`", which depend on MongoDB's unique index. Those
behaviours are checked against a real database instead.

**Fakes rather than mocks.** The fake repository behaves like a small
in-memory database (including the unique-email rule) and returns copies of
stored users. Tests assert outcomes rather than call sequences, so they stay
valid when the implementation is refactored.

## 8. HTTP API

### D21. Response format

**Decision.**

- JSON fields use `snake_case` (`created_at`, `access_token`), following the
  OAuth 2.0 token response convention.
- Every error has the same shape, with a stable machine-readable `code`:

  ```json
  {"error": {"code": "validation_failed", "message": "request validation failed",
             "fields": [{"field": "email", "message": "must be a valid email address"}]}}
  ```

- Unexpected errors return `500 internal_error` with a generic message; the
  real error is written to the log only.

| Situation | Status | Code |
| --- | --- | --- |
| User created | 201 + `Location` header | |
| User deleted | 204, empty body | |
| Malformed JSON, unknown field, wrong type | 400 | `invalid_request` |
| Field validation failed | 400 | `validation_failed` |
| Missing or malformed `Authorization` header | 401 + `WWW-Authenticate` | `unauthorized` |
| Invalid or expired token | 401 + `WWW-Authenticate` | `invalid_token` |
| Wrong email or password | 401 | `invalid_credentials` |
| User not found (including malformed ID) | 404 | `not_found` |
| Email already used | 409 | `email_already_exists` |
| Body larger than 1 MiB | 413 | `request_too_large` |

### D22. Strict request decoding

**Decision.** Request bodies are decoded with `DisallowUnknownFields` and
limited to 1 MiB. Validation errors report JSON field names (`email`, not
`Email`).

**Why.** A client that sends `{"password": "..."}` to the update endpoint gets
a clear 400 instead of a silent no-op, and typos in field names surface
immediately. The size limit stops a client from exhausting memory with a huge
body.

### D23. Gin set-up

**Decision.** The router is built with `gin.New()` rather than
`gin.Default()`, with our own request logger and a recovery middleware that
returns the standard JSON error. No proxy is trusted for client IP headers.
The router is served by a plain `net/http` `http.Server` with read, write and
idle timeouts.

**Why.** The logging middleware is a requirement, so it is implemented here
rather than taken from Gin. `ReadHeaderTimeout` protects against clients that
open connections and send headers very slowly (Slowloris). Using
`http.Server` directly gives graceful shutdown through `Shutdown`.

### D24. Middleware order and request logging

**Decision.** Request logger → panic recovery → body size limit → (for
`/api/v1/users`) authentication.

**Why.** The logger is outermost, so it records the final status and the
full duration even when a handler panics. Each request produces one
structured JSON log line with `method`, `path`, `status`, `duration_ms`,
`client_ip`, and `user_id` when authenticated. Query strings and headers are
not logged because they can contain credentials.

## 9. Background work and lifecycle

### D25. User count worker

**Decision.** A goroutine uses a `time.Ticker` to call `UserService.Count`
every `USER_COUNT_INTERVAL` (default 10s) and logs the result. It is a
driving adapter, like the HTTP handlers, so it goes through the application
layer rather than the database directly.

**Details.** Each count has a timeout equal to the interval, so a slow
database cannot pile up overlapping queries. The ticker is stopped when the
worker returns, and a count cancelled by shutdown is not reported as an
error.

### D26. Graceful shutdown

**Decision.** `signal.NotifyContext` turns SIGINT and SIGTERM into context
cancellation. The HTTP server and the worker run in an `errgroup`; if either
fails (for example, the port is already in use), the shared context is
cancelled and the other stops too. On shutdown the HTTP server stops
accepting connections and waits up to `SHUTDOWN_TIMEOUT` for in-flight
requests, then the MongoDB client disconnects.

### D27. Configuration from environment variables

**Decision.** All settings come from environment variables with safe
defaults; `JWT_SECRET` has no default. Invalid values are reported together
and stop the service at start-up.

**Why.** Environment variables work the same locally, in Docker Compose and
in orchestrators, and keep secrets out of the repository. Failing at
start-up is safer than discovering a bad setting on the first request.
