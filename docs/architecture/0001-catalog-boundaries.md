# ADR 0001: Catalog boundaries and operational checks

Status: accepted

## Context

The application is a modular monolith with a read-only game catalog backed by
PostgreSQL. HTTP tests must not require a database, and SQL behavior must be
verified against PostgreSQL. Public write operations require an authorization
model that is not implemented yet.

## Decisions

- The HTTP consumer defines a small Catalog interface. PostgreSQL implements it;
  the immutable memory catalog supports deterministic tests.
- Use explicit SQL and parameter binding. No generic repository or service layer
  is added until business rules require one.
- List games with an ascending ID cursor (`id > after_id`), default 20 and maximum
  100 items. Fetch one extra row to determine whether to return a next cursor.
  The database primary key supports this access pattern.
- Return `items` and nullable `next_after_id`. This replaces the initial bare-array
  contract before external clients exist. Further client-facing changes need an
  explicit compatibility decision.
- Apply versioned schema migrations separately from starting API processes.
- Liveness only checks that HTTP can respond. Readiness queries the columns used
  by the catalog with a one-second deadline; missing schema and database failures
  return 503 without exposing internal errors. Do not use readiness failures as
  a reason to restart an otherwise healthy process.
- Database-dependent request work has a three-second deadline inherited from
  the request context. Readiness is not a guarantee that a later query will succeed.
- Keep public catalog writes disabled until authentication and admin permissions
  exist. Do not introduce a temporary unauthenticated write endpoint.

## Trade-offs and limits

ID pagination supports sequential traversal, not arbitrary page numbers or
sorting by rating. Concurrent inserts may appear on later pages; this is not a
snapshot across requests. New sort modes need their own stable cursor and index.
No total count is calculated for each page.

Readiness verifies the current read contract, not all future database migrations.
It does not yet include draining state during shutdown.

Local Compose uses a database owner account for convenience. Separate migration
and restricted runtime roles, backups, authentication, and deployment controls
are required before public deployment. Redis and Kafka are not yet application
dependencies and must not affect readiness.
