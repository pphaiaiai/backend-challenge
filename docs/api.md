# API Reference

Sample requests and responses for every endpoint. The responses were
captured from the running service; IDs, timestamps and tokens will differ on
your machine.

- [REST API](#rest-api)
- [Errors](#errors)
- [gRPC API](#grpc-api)
- [Postman](#postman)

## REST API

Base URL: `http://localhost:8080`. Request and response bodies are JSON.
Endpoints under `/api/v1/users` require `Authorization: Bearer <token>`
(see [authentication.md](authentication.md)).

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/healthz` | – | Liveness check |
| POST | `/api/v1/auth/register` | – | Register a user |
| POST | `/api/v1/auth/login` | – | Get an access token |
| POST | `/api/v1/users` | Bearer | Create a user |
| GET | `/api/v1/users` | Bearer | List users (paginated) |
| GET | `/api/v1/users/me` | Bearer | Get the authenticated user |
| GET | `/api/v1/users/{id}` | Bearer | Get a user by ID |
| PATCH | `/api/v1/users/{id}` | Bearer | Update a user's name and/or email |
| DELETE | `/api/v1/users/{id}` | Bearer | Delete a user |

The examples below assume the token is in a shell variable:

```bash
TOKEN=$(curl -s localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}' | jq -r .access_token)
```

### Health check

```bash
curl -i localhost:8080/healthz
```

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{"status":"ok"}
```

### Register

Public. Validation: `name` is required (at most 100 characters), `email` must
be a valid address, `password` must be 8 characters to 72 bytes. Emails are
stored trimmed and lower-cased.

```bash
curl -i localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","password":"password123"}'
```

```http
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8
Location: /api/v1/users/6aff607936d2f4d0f9653c10

{
  "id": "6aff607936d2f4d0f9653c10",
  "name": "Alice",
  "email": "alice@example.com",
  "created_at": "2026-11-19T20:47:53.86Z"
}
```

### Login

```bash
curl -i localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}'
```

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJiYWNrZW5kLWNoYWxsZW5nZSIsInN1YiI6IjZhZmY2MDc5MzZkMmY0ZDBmOTY1M2MxMCIsImV4cCI6MTc5NTEyNDg3NCwiaWF0IjoxNzk1MTIxMjc0fQ.cY8jZu_UQn2TjeS3yqNY8ri5jjkn5NKW6jV8LdGnBvM",
  "token_type": "Bearer",
  "expires_in": 3600,
  "expires_at": "2026-11-19T21:47:54Z"
}
```

### Create user

Same body and validation as register, but requires a token.

```bash
curl -i localhost:8080/api/v1/users \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Bob","email":"bob@example.com","password":"password123"}'
```

```http
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8
Location: /api/v1/users/6aff607a36d2f4d0f9653c11

{
  "id": "6aff607a36d2f4d0f9653c11",
  "name": "Bob",
  "email": "bob@example.com",
  "created_at": "2026-11-19T20:47:54.103Z"
}
```

### List users

Query parameters: `offset` (default 0) and `limit` (default 20, maximum 100).
Users are returned oldest first; `total` is the number of users overall.

```bash
curl -i "localhost:8080/api/v1/users?offset=0&limit=20" -H "Authorization: Bearer $TOKEN"
```

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "users": [
    {"id": "6aff607936d2f4d0f9653c10", "name": "Alice", "email": "alice@example.com", "created_at": "2026-11-19T20:47:53.86Z"},
    {"id": "6aff607a36d2f4d0f9653c11", "name": "Bob", "email": "bob@example.com", "created_at": "2026-11-19T20:47:54.103Z"}
  ],
  "total": 2,
  "offset": 0,
  "limit": 20
}
```

### Get the authenticated user

```bash
curl -i localhost:8080/api/v1/users/me -H "Authorization: Bearer $TOKEN"
```

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "id": "6aff607936d2f4d0f9653c10",
  "name": "Alice",
  "email": "alice@example.com",
  "created_at": "2026-11-19T20:47:53.86Z"
}
```

### Get user by ID

```bash
curl -i localhost:8080/api/v1/users/6aff607936d2f4d0f9653c10 -H "Authorization: Bearer $TOKEN"
```

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "id": "6aff607936d2f4d0f9653c10",
  "name": "Alice",
  "email": "alice@example.com",
  "created_at": "2026-11-19T20:47:53.86Z"
}
```

### Update user

Send `name`, `email` or both; omitted fields are unchanged. Other fields,
including `password`, are rejected.

```bash
curl -i -X PATCH localhost:8080/api/v1/users/6aff607936d2f4d0f9653c10 \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice Smith"}'
```

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "id": "6aff607936d2f4d0f9653c10",
  "name": "Alice Smith",
  "email": "alice@example.com",
  "created_at": "2026-11-19T20:47:53.86Z"
}
```

### Delete user

```bash
curl -i -X DELETE localhost:8080/api/v1/users/6aff607a36d2f4d0f9653c11 -H "Authorization: Bearer $TOKEN"
```

```http
HTTP/1.1 204 No Content
```

## Errors

Every error has the same shape. `code` is stable and meant for programs;
`message` is for people. `fields` appears only for validation errors.

```json
{"error": {"code": "validation_failed", "message": "request validation failed", "fields": [...]}}
```

| Status | Code | When |
| --- | --- | --- |
| 400 | `invalid_request` | Malformed JSON, unknown field, wrong type, empty body |
| 400 | `validation_failed` | A field fails validation |
| 401 | `unauthorized` | Missing or malformed `Authorization` header |
| 401 | `invalid_token` | Token is invalid or expired |
| 401 | `invalid_credentials` | Wrong email or password |
| 404 | `not_found` | User or route does not exist |
| 405 | `method_not_allowed` | Method not supported on the route |
| 409 | `email_already_exists` | Email is used by another user |
| 413 | `request_too_large` | Body larger than 1 MiB |
| 500 | `internal_error` | Unexpected error (details are only logged) |

Examples:

```http
POST /api/v1/auth/register  {"name":"","email":"not-an-email","password":"short"}

HTTP/1.1 400 Bad Request
{
  "error": {
    "code": "validation_failed",
    "message": "request validation failed",
    "fields": [
      {"field": "name", "message": "is required"},
      {"field": "email", "message": "must be a valid email address"},
      {"field": "password", "message": "must be at least 8 characters"}
    ]
  }
}
```

```http
POST /api/v1/auth/register  {"name":"Alice","email":"alice@example.com","password":"password123"}

HTTP/1.1 409 Conflict
{"error": {"code": "email_already_exists", "message": "email already exists"}}
```

```http
PATCH /api/v1/users/6aff607936d2f4d0f9653c10  {"password":"new-password"}

HTTP/1.1 400 Bad Request
{"error": {"code": "invalid_request", "message": "invalid request: unknown field \"password\""}}
```

```http
POST /api/v1/auth/login  {"email":"alice@example.com","password":"wrong-password"}

HTTP/1.1 401 Unauthorized
{"error": {"code": "invalid_credentials", "message": "invalid email or password"}}
```

```http
GET /api/v1/users   (no Authorization header)

HTTP/1.1 401 Unauthorized
Www-Authenticate: Bearer
{"error": {"code": "unauthorized", "message": "missing or malformed Authorization header"}}
```

```http
GET /api/v1/users   Authorization: Bearer invalid-token

HTTP/1.1 401 Unauthorized
Www-Authenticate: Bearer error="invalid_token"
{"error": {"code": "invalid_token", "message": "invalid or expired token"}}
```

```http
GET /api/v1/users/6aff607a36d2f4d0f9653c11   (deleted user)

HTTP/1.1 404 Not Found
{"error": {"code": "not_found", "message": "user not found"}}
```

## gRPC API

Address: `localhost:9090` (plaintext). Contract:
[`proto/user/v1/user.proto`](../proto/user/v1/user.proto). Both methods
require the metadata `authorization: Bearer <token>`, using the token from
the REST login endpoint. Server reflection is enabled, so
[grpcurl](https://github.com/fullstorydev/grpcurl) and Postman can discover
the service without the `.proto` file.

gRPC uses the protobuf JSON mapping, so fields appear in lowerCamelCase
(`createdAt`), unlike the REST API's `created_at`.

```bash
grpcurl -plaintext localhost:9090 list
```

```text
grpc.reflection.v1.ServerReflection
grpc.reflection.v1alpha.ServerReflection
user.v1.UserService
```

### CreateUser

```bash
grpcurl -plaintext -H "authorization: Bearer $TOKEN" \
  -d '{"name":"Carol","email":"carol@example.com","password":"password123"}' \
  localhost:9090 user.v1.UserService/CreateUser
```

```json
{
  "user": {
    "id": "6aff608936d2f4d0f9653c12",
    "name": "Carol",
    "email": "carol@example.com",
    "createdAt": "2026-11-19T20:48:09.428Z"
  }
}
```

### GetUser

```bash
grpcurl -plaintext -H "authorization: Bearer $TOKEN" \
  -d '{"id":"6aff608936d2f4d0f9653c12"}' \
  localhost:9090 user.v1.UserService/GetUser
```

```json
{
  "user": {
    "id": "6aff608936d2f4d0f9653c12",
    "name": "Carol",
    "email": "carol@example.com",
    "createdAt": "2026-11-19T20:48:09.428Z"
  }
}
```

### gRPC errors

| Code | When |
| --- | --- |
| `INVALID_ARGUMENT` | A field fails validation |
| `ALREADY_EXISTS` | Email is used by another user |
| `NOT_FOUND` | User does not exist |
| `UNAUTHENTICATED` | Missing, malformed, invalid or expired token |
| `INTERNAL` | Unexpected error (details are only logged) |

```text
$ grpcurl -plaintext -H "authorization: Bearer $TOKEN" \
    -d '{"name":"Dan","email":"bad","password":"password123"}' \
    localhost:9090 user.v1.UserService/CreateUser
ERROR:
  Code: InvalidArgument
  Message: email must be a valid email address

$ grpcurl -plaintext -d '{"id":"6aff608936d2f4d0f9653c12"}' \
    localhost:9090 user.v1.UserService/GetUser
ERROR:
  Code: Unauthenticated
  Message: missing or malformed authorization metadata
```

## Postman

Import [`postman_collection.json`](postman_collection.json). It contains every
REST request above plus the error cases, stores the token after login, and
has tests for each response, so **Run collection** checks the whole API in
one go. Register generates a new email on every run, so the collection can be
run repeatedly.

For gRPC, create a gRPC request in Postman with the URL `localhost:9090`
(TLS off), pick a method from server reflection, and add the metadata
`authorization: Bearer {{token}}`.
