# Authentication Guide (JWT)

Protected endpoints accept a JSON Web Token (JWT) signed with HMAC-SHA256
(HS256). A token is obtained by logging in and is sent with every request
until it expires (1 hour by default).

- [How it works](#how-it-works)
- [1. Get a token](#1-get-a-token)
- [2. Use the token](#2-use-the-token)
- [What is inside a token](#what-is-inside-a-token)
- [Expiry and errors](#expiry-and-errors)
- [Configuration](#configuration)
- [Security notes](#security-notes)

## How it works

```mermaid
sequenceDiagram
    participant C as Client
    participant A as API
    participant DB as MongoDB

    C->>A: POST /api/v1/auth/login {email, password}
    A->>DB: find user by email
    A->>A: bcrypt compare, sign JWT (HS256)
    A-->>C: 200 {access_token, expires_in}

    C->>A: GET /api/v1/users  Authorization: Bearer <token>
    A->>A: verify signature, issuer and expiry
    A->>DB: query users
    A-->>C: 200 {users}
```

The server does not store tokens. It checks each one by recomputing its
signature with the secret key, so any instance of the service can verify a
token without a session store.

## 1. Get a token

Create an account (skip this if you already have one):

```bash
curl -s localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","password":"password123"}'
```

Log in with the same email and password:

```bash
curl -s localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}'
```

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJiYWNrZW5kLWNoYWxsZW5nZSIsInN1YiI6IjZhZmY2MDc5MzZkMmY0ZDBmOTY1M2MxMCIsImV4cCI6MTc5NTEyNDg3NCwiaWF0IjoxNzk1MTIxMjc0fQ.cY8jZu_UQn2TjeS3yqNY8ri5jjkn5NKW6jV8LdGnBvM",
  "token_type": "Bearer",
  "expires_in": 3600,
  "expires_at": "2026-11-19T21:47:54Z"
}
```

To keep it in a shell variable:

```bash
TOKEN=$(curl -s localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}' | jq -r .access_token)
```

## 2. Use the token

**REST:** send it in the `Authorization` header with the `Bearer` scheme.

```bash
curl -s localhost:8080/api/v1/users/me -H "Authorization: Bearer $TOKEN"
```

**gRPC:** send it as the `authorization` metadata entry.

```bash
grpcurl -plaintext -H "authorization: Bearer $TOKEN" \
  -d '{"id":"6aff607936d2f4d0f9653c10"}' localhost:9090 user.v1.UserService/GetUser
```

In Go, a gRPC client adds it to the outgoing context:

```go
ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
```

**Postman:** the [collection](postman_collection.json) stores the token in the
`token` variable after **Login**, and every protected request uses
`Bearer {{token}}`.

## What is inside a token

A JWT has three base64url-encoded parts separated by dots:
`header.payload.signature`. Decoding the example above gives:

```json
{"alg": "HS256", "typ": "JWT"}
```

```json
{
  "iss": "backend-challenge",
  "sub": "6aff607936d2f4d0f9653c10",
  "iat": 1795121274,
  "exp": 1795124874
}
```

| Claim | Meaning |
| --- | --- |
| `sub` | ID of the user the token belongs to |
| `iss` | Issuer; must equal `JWT_ISSUER` |
| `iat` | Issued at (Unix seconds) |
| `exp` | Expires at (Unix seconds); required |

The signature is `HMAC-SHA256(header + "." + payload, JWT_SECRET)`. Changing
any character of the header or payload invalidates it.

The payload is encoded, not encrypted: anyone holding a token can read it
(for example at [jwt.io](https://jwt.io)). That is why it contains only the
user ID and never anything secret.

## Expiry and errors

| Situation | REST | gRPC |
| --- | --- | --- |
| No `Authorization` header, or not `Bearer <token>` | `401 unauthorized` | `UNAUTHENTICATED` |
| Bad signature, wrong issuer, wrong algorithm, expired | `401 invalid_token` | `UNAUTHENTICATED` |
| Wrong email or password at login | `401 invalid_credentials` | – |

401 responses from protected REST endpoints include a
`WWW-Authenticate: Bearer` header. When a token expires, log in again to get
a new one; there is no refresh token.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `JWT_SECRET` | none (required) | HMAC key, at least 32 bytes |
| `JWT_TTL` | `1h` | Token lifetime, as a Go duration (`15m`, `2h`) |
| `JWT_ISSUER` | `backend-challenge` | Value of the `iss` claim |

Generate a secret with:

```bash
openssl rand -base64 48
```

The service refuses to start if `JWT_SECRET` is missing or shorter than 32
bytes. Changing the secret invalidates every token already issued, which is
also how to log everyone out in an emergency.

## Security notes

- **Only HS256 is accepted.** Tokens whose header names another algorithm,
  including `none`, are rejected, which prevents algorithm-confusion attacks.
- **`exp` is required and `iss` is checked**, so a token without an expiry or
  issued by another service is rejected even if its signature is valid.
- **Login does not reveal whether an email exists.** Unknown emails and wrong
  passwords return the same error, and both take a similar time.
- **Passwords are hashed with bcrypt** and never returned by the API.
- **Tokens are bearer credentials**: whoever holds one can use it. In
  production the API must be served over HTTPS (TLS), usually terminated at a
  load balancer.
- **Tokens cannot be revoked before they expire.** A deleted user's token
  keeps working until `exp`; the short default lifetime limits that window.
  See decision D19 in [decisions.md](decisions.md) for the options.
