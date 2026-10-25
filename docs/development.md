# Local development

Requirements: Go and Docker Desktop with Linux containers.

Copy `.env.example` to `.env` if it does not already exist. Then:

```powershell
docker compose up -d postgres --wait
docker compose --profile app build api
docker compose --profile app run --rm --entrypoint /migrate api
docker compose --profile app up -d api
```

The migration command applies pending SQL migrations, recording versions in
PostgreSQL. Run it before the API when deploying schema changes. Existing
migrations must not be edited after deployment; add another numbered file.
Migration rollback can delete data; it is deliberately not exposed by the command.

For GoLand, set `DATABASE_URL` in the run configuration. A local example using
the credentials from `.env.example`:

```powershell
$env:DATABASE_URL='postgres://game_reviews:local-development-only@localhost:15432/game_reviews?sslmode=disable'
$env:COOKIE_SECURE='false' # Local HTTP only; defaults to true outside Compose.
go run ./cmd/migrate
go run ./cmd/api
```

The Go programs read process environment variables, not `.env` files.
Use your actual credentials if you changed `.env`. `sslmode=disable` is only
for local development; deployed connections should use verified TLS.

The catalog starts empty. API endpoints:

- `GET /healthz`: process liveness only.
- `GET /readyz`: database connectivity and required catalog columns; 503 if unavailable.
- `GET /api/v1/games?limit=20&after_id=0`: games ordered by ascending ID.
- `GET /api/v1/games/{id}`: a game, 400 for invalid IDs, or 404 if absent.

The list response is `{"items":[],"next_after_id":null}` for an empty catalog.
The default limit is 20, maximum 100. If `next_after_id` is non-null, pass it as
`after_id` in the next request, keeping the same limit. Duplicate, unknown, and
invalid query parameters return 400. Demo data is not inserted by migrations.

## Tests

```powershell
go test ./...
go vet ./...
```

Database integration tests run only with `TEST_DATABASE_URL` set. Use a dedicated
test database: the test creates and drops its own schema and requires those rights.
CI supplies PostgreSQL and runs the same tests automatically.

```powershell
$env:TEST_DATABASE_URL='postgres://user:password@localhost:15432/game_reviews_test?sslmode=disable'
go test ./internal/postgres -v
```

## Accounts

Apply migrations again before starting the updated API. Authentication uses
server-side sessions and a cookie; retain cookies between requests.

```powershell
$body = @{ username='player_one'; password='replace-with-a-long-unique-passphrase' } | ConvertTo-Json
Invoke-RestMethod http://localhost:8080/api/v1/auth/register -Method Post -ContentType application/json -Body $body
Invoke-RestMethod http://localhost:8080/api/v1/auth/login -Method Post -ContentType application/json -Body $body -SessionVariable loginSession
Invoke-RestMethod http://localhost:8080/api/v1/auth/me -WebSession $loginSession
Invoke-RestMethod http://localhost:8080/api/v1/auth/logout -Method Post -WebSession $loginSession
```

Registration returns 201 and does not log the user in. Login returns 200 and sets
an HttpOnly, SameSite=Strict cookie. Logout returns 204 and revokes the session;
`me` without a valid session returns 401. Usernames are case-insensitive ASCII
letters, digits and underscores, 3-32 characters. Passwords are 15-128 Unicode
characters, with no trimming or composition rules.

Secure cookies are enabled by default and require HTTPS. Local Compose disables
Secure explicitly for HTTP. The frontend must use the same origin as the API;
cross-origin state-changing browser requests are rejected. Request bodies are
limited to 4096 bytes and unknown fields (including `role`) are rejected.

Login and registration share a limit of 10 requests/minute per source IP within
each API process. Proxy forwarding headers are not trusted. This is an initial
single-instance limit, not a distributed credential-stuffing defense.

## Docker Desktop startup on Windows

An error mentioning an inaccessible `dockerInference` or `engine.sock` can be
caused by a stale Windows AF_UNIX socket. This occurs before PostgreSQL starts
and is not a database migration failure. See the
[Docker issue](https://github.com/docker/desktop-feedback/issues/554).

With all Docker Desktop/backend processes stopped, preserving the affected
runtime directory under a backup name can allow Desktop to recreate its sockets.
Inspect the directory contents first; do not move Docker's VM disks or data
directories. The observed locations were `%LOCALAPPDATA%\Docker\run` and
`%LOCALAPPDATA%\docker-secrets-engine`. Keep the backup directories until startup
and existing volumes have been verified. A factory reset is not this procedure
and may remove local container data.

